package cassette

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/cespare/xxhash/v2"
	_ "github.com/mattn/go-sqlite3"
)

type (
	Control struct {
		db        *sql.DB
		writeable bool
	}
)

func openCassetteDatabase(ctx context.Context, tape string, readwrite bool) (*sql.DB, error) {
	var connstr string
	if readwrite {
		connstr = fmt.Sprintf("file:%v?_writable_schema=false&_journal=wal&mode=rwc", tape)
	} else {
		connstr = fmt.Sprintf("file:%v?_writable_schema=false&mode=r", tape)
	}
	conn, err := sql.Open("sqlite3", connstr)
	if err != nil {
		return nil, fmt.Errorf("unable to open %v, cause %v", tape, err)
	}
	err = conn.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to ping cassette %v, cause %v", tape, err)
	}
	return conn, nil
}

func LoadControlCassette(ctx context.Context, tape string, readwrite bool) (*Control, error) {
	conn, err := openCassetteDatabase(ctx, tape, readwrite)
	if err != nil {
		return nil, err
	}
	c := &Control{db: conn, writeable: readwrite}
	err = c.init(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to init cassette %v, cause %v", tape, err)
	}
	return c, nil
}

func (c *Control) ListAssets(ctx context.Context) ([]string, error) {
	var out []string
	rows, err := c.db.QueryContext(ctx, `select path from assets order by path asc`)
	if err != nil {
		return nil, fmt.Errorf("unable to list assets, cause %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, fmt.Errorf("unable to scan asset name to output, cause %v", err)
		}
		out = append(out, name)
	}
	return out, nil
}

func (c *Control) CopyAsset(ctx context.Context, out io.Writer, assetPath string) (int64, string, error) {
	assetPath, pathHash := c.normalizeAssetPath(assetPath)
	var content string
	var aid int64
	var mt string
	err := c.db.QueryRowContext(ctx, `select asset_id, mime_type, content from assets where path_hash64 = ? and path = ?`, pathHash, assetPath).Scan(&aid, &mt, &content)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", AssetNotFound{Path: assetPath}
	} else if err != nil {
		return 0, "", fmt.Errorf("unable to load %v from cassete, cause %w", assetPath, err)
	}
	_, err = io.WriteString(out, content)
	if err != nil {
		return 0, "", fmt.Errorf("unable to copy %v from cassete to destination, cause %w", assetPath, err)
	}
	return aid, mt, nil
}

func (c *Control) StoreAsset(ctx context.Context, assetPath string, mimetype string, content string) (int64, error) {
	assetPath, pathHash := c.normalizeAssetPath(assetPath)
	seq, err := c.nextSeq(ctx, "raw_assets")
	if err != nil {
		return 0, err
	}
	switch mimetype {
	case "text/html", "text/json", "application/json", "application/x-lua", "text/x-lua":
		if !utf8.ValidString(content) {
			return 0, InvalidTextContent{Path: assetPath, MimeType: mimetype}
		}
	}
	_, err = c.db.ExecContext(ctx, `insert into assets(asset_id, path, path_hash64, mime_type, content) values (?, ?, ?, ?, ?)`,
		seq, assetPath, pathHash, mimetype, content)
	if err != nil {
		return 0, fmt.Errorf("unable to store asset to cassette, cause %w", err)
	}
	return seq, nil
}

func (c *Control) ToggleCodebase(ctx context.Context, assetPath string, enable bool) error {
	assetPath, pathHash := c.normalizeAssetPath(assetPath)
	var mt string
	var id int64
	err := c.db.QueryRowContext(ctx, `select asset_id, mime_type from assets where path_hash64 = ? and path = ?`, pathHash, assetPath).Scan(&id, &mt)
	if errors.Is(err, sql.ErrNoRows) {
		return AssetNotFound{Path: assetPath}
	} else if err != nil {
		return fmt.Errorf("unable to load %v from cassete, cause %w", assetPath, err)
	}
	switch mt {
	case "text/x-lua", "application/x-lua":
		break
	default:
		return InvalidCodebase{MimeType: mt, Path: assetPath}
	}
	if !c.validCodebasePath(assetPath) {
		return InvalidCodebase{
			Path: assetPath, MimeType: mt, msg: "codebase assets must exist within codebase/... root and must have .lua extension",
		}
	}
	if enable {
		_, err = c.db.ExecContext(ctx, `insert into codebase(asset_id) values (?) on conflict (asset_id) do nothing`, id)
	} else {
		_, err = c.db.ExecContext(ctx, `delete from codebase where asset_id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("unable to change state of asset %v in codebase, cause %w", assetPath, err)
	}
	return nil
}

func (c *Control) nextSeq(ctx context.Context, seq string) (int64, error) {
	var val int64
	err := c.db.QueryRowContext(ctx, `insert into counters (name, val) values (?, 1) on conflict do update set val = val + 1 returning val`, seq).Scan(&val)
	if err != nil {
		return 0, fmt.Errorf("unable to increment sequence %v, cause %w", seq, err)
	}
	return val, nil
}

func (c *Control) validCodebasePath(assetPath string) bool {
	return strings.HasPrefix(assetPath, "codebase/") &&
		path.Ext(assetPath) == ".lua"
}

func (c *Control) normalizeAssetPath(assetPath string) (string, int64) {
	assetPath = path.Clean(assetPath)
	pathHash := int64(xxhash.Sum64String(assetPath))
	return assetPath, pathHash
}

func (c *Control) init(ctx context.Context) error {
	for _, cmd := range []string{
		`create table if not exists counters(
			name text not null primary key,
			val integer not null
		)`,
		`create table if not exists assets(
			asset_id integer not null primary key,
			path text not null unique,
			path_hash64 integer not null,
			mime_type string not null,
			content blob not null
		)`,
		`create index idx_assets_path_hash64
			on assets(path_hash64)
		`,
		`create table if not exists codebase(
			asset_id integer not null primary key,
			foreign key (asset_id) references assets(asset_id)
		)`,
		`create table if not exists routes(
			route text not null primary key,
			asset_id integer,
			foreign key(asset_id) references codebase(asset_id)
		)`,
	} {
		_, err := c.db.ExecContext(ctx, cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Control) Close() error {
	return c.db.Close()
}
