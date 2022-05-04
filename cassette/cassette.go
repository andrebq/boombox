package cassette

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/cespare/xxhash/v2"
	_ "github.com/mattn/go-sqlite3"
)

type (
	limitedWriter struct {
		dest       io.Writer
		maxBytes   int
		totalBytes int
	}

	Control struct {
		db     *sql.DB
		datadb *sql.DB

		controlPath string
		dataPath    string

		writeable bool
	}

	Code struct {
		Methods []string
		Route   string
		Code    string
	}

	Row []interface{}
)

var (
	errInvalidCodebaseAsset = errors.New("codebase assets must be stored under codebase/ and must have a valid lua mime-type")
)

func openCassetteDatabase(ctx context.Context, tape string, dbname string, readwrite bool) (*sql.DB, string, error) {
	tape = filepath.Join(tape, dbname)
	if readwrite {
		err := os.MkdirAll(filepath.Dir(tape), 0755)
		if err != nil {
			return nil, tape, fmt.Errorf("unable to create directory %v to store cassette, cause %w", tape, err)
		}
	}
	var connstr string
	if readwrite {
		connstr = fmt.Sprintf("file:%v?_writable_schema=false&_journal=wal&mode=rwc", tape)
	} else {
		connstr = fmt.Sprintf("file:%v?_writable_schema=false&mode=ro", tape)
	}
	conn, err := sql.Open("sqlite3", connstr)
	if err != nil {
		return nil, tape, fmt.Errorf("unable to open %v, cause %v", tape, err)
	}
	err = conn.PingContext(ctx)
	if err != nil {
		return nil, tape, fmt.Errorf("unable to ping cassette %v, cause %v", tape, err)
	}
	return conn, tape, nil
}

func LoadControlCassette(ctx context.Context, tape string, readwrite bool, enableData bool) (*Control, error) {
	conn, controlPath, err := openCassetteDatabase(ctx, tape, "k7.db", readwrite)
	if err != nil {
		return nil, err
	}
	c := &Control{db: conn, writeable: readwrite, controlPath: controlPath}
	err = c.init(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to init cassette %v, cause %v", tape, err)
	}
	if enableData {
		dataconn, dataPath, err := openCassetteDatabase(ctx, tape, "datak7.db", readwrite)
		if err != nil {
			c.Close()
			return nil, err
		}
		err = c.initData(ctx)
		if err != nil {
			c.Close()
			return nil, err
		}
		c.datadb = dataconn
		c.dataPath = dataPath
	}
	return c, nil
}

func (c *Control) Queryable() bool { return !c.writeable }

func (c *Control) ListRoutes(ctx context.Context) ([]Code, error) {
	var out []Code
	rows, err := c.db.QueryContext(ctx, `select r.route, a.content, r.methods
	from routes r
	inner join codebase c on r.asset_id = c.asset_id
	inner join assets a on c.asset_id = a.asset_id`)
	if err != nil {
		return nil, fmt.Errorf("unable to get routes from cassette, cause %w", err)
	}
	for rows.Next() {
		var c Code
		var methodStr string
		err = rows.Scan(&c.Route, &c.Code, &methodStr)
		if err != nil {
			return nil, fmt.Errorf("unable to get routes from cassette, cause %w", err)
		}
		methodStr = strings.ToUpper(methodStr)
		c.Methods = strings.Split(methodStr, "|")
		out = append(out, c)
	}
	return out, nil
}

func (c *Control) MapRoute(ctx context.Context, methods []string, route string, asset string) error {
	// TODO: perform proper method validation here
	asset, _ = c.normalizeAssetPath(asset)
	id, _, err := c.lookupCodebase(ctx, asset)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `insert into routes (route, methods, asset_id) values (?, ?, ?) on conflict (route) do update set methods = EXCLUDED.methods and asset_id = EXCLUDED.asset_id`, route, strings.ToUpper(strings.Join(methods, "|")), id)
	if err != nil {
		return fmt.Errorf("unable to configure route %v using asset %v, cause %w", route, asset, err)
	}
	return nil
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
	_, err = c.db.ExecContext(ctx, `insert into assets(asset_id, path, path_hash64, mime_type, content) values (?, ?, ?, ?, ?) on conflict (path) do update set mime_type = EXCLUDED.mime_type, content = EXCLUDED.content`,
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
	if err := c.validCodebasePath(assetPath); err != nil {
		return InvalidCodebase{
			Path: assetPath, MimeType: mt, cause: err,
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

func (c *Control) Query(ctx context.Context, out io.Writer, maxSize int, query string, args ...interface{}) error {
	const OneMegabyte = 1_000_000
	if c.writeable {
		// TODO: querying a writable cassette is SOOOO wrong that I am considering a panic instead!
		// afterall, a Queryable Cassette should run alone on its own process...
		return CannotQuery{}
	}
	if maxSize > OneMegabyte || maxSize < 0 {
		maxSize = OneMegabyte
	}
	out = &limitedWriter{dest: out, maxBytes: maxSize}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return QueryError{Query: query, cause: err, Params: args}
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return QueryError{Query: query, cause: err, Params: args}
	}
	_, err = io.WriteString(out, `{"columns":`)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(out)
	err = enc.Encode(columns)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, `,"rows": [`)
	if err != nil {
		return err
	}

	first := true
	for rows.Next() {
		r := make(Row, len(columns))
		scanTarget := make([]interface{}, len(r))
		for i := range scanTarget {
			scanTarget[i] = &r[i]
		}
		err = rows.Scan(scanTarget...)
		if err != nil {
			return QueryError{Query: query, cause: err, Params: args}
		}
		if !first {
			_, err = io.WriteString(out, ",")
			if err != nil {
				return err
			}
		}
		first = false
		err = enc.Encode(r)
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, "]}")
	return err
}

func (c *Control) nextSeq(ctx context.Context, seq string) (int64, error) {
	var val int64
	err := c.db.QueryRowContext(ctx, `insert into counters (name, val) values (?, 1) on conflict do update set val = val + 1 returning val`, seq).Scan(&val)
	if err != nil {
		return 0, fmt.Errorf("unable to increment sequence %v, cause %w", seq, err)
	}
	return val, nil
}

func (c *Control) lookupCodebase(ctx context.Context, assetPath string) (int64, string, error) {
	assetPath, hash := c.normalizeAssetPath(assetPath)
	if err := c.validCodebasePath(assetPath); err != nil {
		return 0, "", InvalidCodebase{
			Path:  assetPath,
			cause: err,
		}
	}
	var id int64
	var mt string
	err := c.db.QueryRowContext(ctx, `select a.asset_id, a.mime_type from assets a inner join codebase c on c.asset_id = a.asset_id
	where a.path_hash64 = ? and a.path = ?`, hash, assetPath).Scan(&id, &mt)
	if err != nil {
		return 0, "", fmt.Errorf("unable to lookup codebase on path %v, cause %w", assetPath, err)
	}
	return id, mt, nil
}

func (c *Control) validCodebasePath(assetPath string) error {
	valid := strings.HasPrefix(assetPath, "codebase/") &&
		path.Ext(assetPath) == ".lua"
	if !valid {
		return errInvalidCodebaseAsset
	}
	return nil
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
		`create index if not exists idx_assets_path_hash64
			on assets(path_hash64)
		`,
		`create table if not exists codebase(
			asset_id integer not null primary key,
			foreign key (asset_id) references assets(asset_id)
		)`,
		`create table if not exists routes(
			route text not null primary key,
			methods text not null,
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

func (c *Control) initData(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, `attach database ? as 'dataset'`, c.dataPath)
	return err
}

func (c *Control) Close() error {
	err := c.db.Close()
	if c.datadb != nil {
		c.datadb.Close()
	}
	return err
}

func (lw *limitedWriter) Write(buf []byte) (int, error) {
	if overshoot := lw.overshoot(len(buf)); overshoot > 0 {
		return 0, WriteOverflow{
			Total: lw.totalBytes,
			Max:   lw.maxBytes,
			Next:  len(buf),
		}
	}
	n, err := lw.dest.Write(buf)
	lw.totalBytes += n
	return 0, err
}

func (lw limitedWriter) overshoot(n int) int {
	return (lw.totalBytes + n) - lw.maxBytes
}
