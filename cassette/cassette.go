package cassette

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
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

		writeable          bool
		extendedPrivileges bool
	}

	Code struct {
		Methods  []string
		Route    string
		Code     string
		CodePath string
	}

	Row []interface{}

	dbLike interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	}
)

var (
	errInvalidCodebaseAsset = errors.New("codebase assets must be stored under codebase/ and must have a valid lua mime-type")
	reValidIdentifiers      = regexp.MustCompile(`^[a-zA-Z_][_a-zA-Z0-9]{0,127}$`)
	reRestrictedTables      = regexp.MustCompile("^(bb_|sqlite|pragma).*$")
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
		dataconn, _, err := openCassetteDatabase(ctx, tape, filepath.Base(c.controlPath), false)
		if err != nil {
			c.Close()
			return nil, err
		}
		c.datadb = dataconn
		err = c.initData(ctx)
		if err != nil {
			c.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *Control) Queryable() bool     { return !c.writeable }
func (c *Control) HasPrivileges() bool { return c.extendedPrivileges }

func (c *Control) ListRoutes(ctx context.Context) ([]Code, error) {
	var out []Code
	rows, err := c.db.QueryContext(ctx, `select r.route, a.path, a.content, r.methods
	from bb_routes r
	inner join bb_codebase c on r.asset_id = c.asset_id
	inner join bb_assets a on c.asset_id = a.asset_id`)
	if err != nil {
		return nil, fmt.Errorf("unable to get routes from cassette, cause %w", err)
	}
	for rows.Next() {
		var c Code
		var methodStr string
		err = rows.Scan(&c.Route, &c.CodePath, &c.Code, &methodStr)
		if err != nil {
			return nil, fmt.Errorf("unable to get routes from cassette, cause %w", err)
		}
		methodStr = strings.ToUpper(methodStr)
		c.Methods = strings.Split(methodStr, "|")
		out = append(out, c)
	}
	return out, nil
}
func (c *Control) LookupRoute(ctx context.Context, route string) (Code, error) {
	// TODO: remove this code duplication
	var code Code
	var methodStr string
	err := c.db.QueryRowContext(ctx, `select r.route, a.path, a.content, r.methods
	from bb_routes r
	inner join bb_codebase c on r.asset_id = c.asset_id
	inner join bb_assets a on c.asset_id = a.asset_id
	where r.route = ?`, route).Scan(&code.Route, &code.CodePath, &code.Code, &methodStr)
	if errors.Is(err, sql.ErrNoRows) {
		return Code{}, RouteNotFound{Route: route}
	} else if err != nil {
		return Code{}, fmt.Errorf("unable to get routes from cassette, cause %w", err)
	}
	methodStr = strings.ToUpper(methodStr)
	code.Methods = strings.Split(methodStr, "|")
	return code, nil
}

func (c *Control) MapRoute(ctx context.Context, methods []string, route string, asset string) error {
	// TODO: perform proper method validation here
	asset, _ = c.normalizeAssetPath(asset)
	id, _, err := c.lookupCodebase(ctx, asset)
	if err != nil {
		return err
	}
	for i, m := range methods {
		m = strings.ToUpper(m)
		methods[i] = m
		switch m {
		case "GET", "POST", "HEAD", "PUT", "DELETE", "OPTIONS":
			continue
		default:
			return fmt.Errorf("invalid http method: %v", m)
		}
	}
	methodStr := strings.Join(methods, "|")
	_, err = c.db.ExecContext(ctx, `insert into bb_routes (route, methods, asset_id) values (?, ?, ?) on conflict (route) do update set methods = excluded.methods, asset_id = excluded.asset_id`, route, methodStr, id)
	if err != nil {
		return fmt.Errorf("unable to configure route %v using asset %v, cause %w", route, asset, err)
	}
	return nil
}

func (c *Control) ListAssets(ctx context.Context) ([]string, error) {
	var out []string
	rows, err := c.db.QueryContext(ctx, `select path from bb_assets order by path asc`)
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
	err := c.db.QueryRowContext(ctx, `select asset_id, mime_type, content from bb_assets where path_hash64 = ? and path = ?`, pathHash, assetPath).Scan(&aid, &mt, &content)
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
	_, err = c.db.ExecContext(ctx, `insert into bb_assets(asset_id, path, path_hash64, mime_type, content) values (?, ?, ?, ?, ?) on conflict (path) do update set mime_type = EXCLUDED.mime_type, content = EXCLUDED.content`,
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
	err := c.db.QueryRowContext(ctx, `select asset_id, mime_type from bb_assets where path_hash64 = ? and path = ?`, pathHash, assetPath).Scan(&id, &mt)
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
		_, err = c.db.ExecContext(ctx, `insert into bb_codebase(asset_id) values (?) on conflict (asset_id) do nothing`, id)
	} else {
		_, err = c.db.ExecContext(ctx, `delete from bb_codebase where asset_id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("unable to change state of asset %v in codebase, cause %w", assetPath, err)
	}
	return nil
}

func (c *Control) ImportJSONDataset(ctx context.Context, tableName string, dataset io.Reader) error {
	table, err := loadTableDef(ctx, c.db, tableName)
	if err != nil {
		return err
	}
	lr := io.LimitReader(dataset, 10_000_000)
	var out []map[string]interface{}
	err = json.NewDecoder(lr).Decode(&out)
	if err != nil {
		return err
	}
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	for _, v := range out {
		err = c.saveTuple(ctx, tx, v, table)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	tx = nil
	return err
}

func (c *Control) ImportCSVDataset(ctx context.Context, table string, csvStream io.Reader) (int64, error) {
	// TODO: this method is HUGE! break it down to make things easier!
	if !c.writeable {
		return 0, ReadonlyCassette{}
	}
	if c.datadb == nil {
		return 0, DatasetNotAllowed{}
	}
	if err := validDatasetTable(table); err != nil {
		return 0, err
	}
	reader := csv.NewReader(csvStream)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var newTable TableDef
	newTable.Name = table

	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("unable to import %v, cause %w", table, err)
	}
	newTable.Columns = make([]ColumnDef, len(header))
	for i := range header {
		newTable.Columns[i] = ColumnDef{
			Name: header[i],
		}
	}
	firstRow, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("unable to import %v, cause %w", table, err)
	}
	for i, d := range firstRow {
		if _, err := strconv.ParseInt(d, 10, 64); err == nil {
			newTable.Columns[i].Datatype = "integer"
		} else if _, err := strconv.ParseFloat(d, 64); err == nil {
			newTable.Columns[i].Datatype = "real"
		} else {
			newTable.Columns[i].Datatype = "text"
		}
	}
	_, err = c.CreateTable(ctx, newTable)
	if errors.Is(err, TableAlreadyExistsError{Name: newTable.Name}) {
		err = nil
	}
	if err != nil {
		return 0, err
	}
	strToTableType := func(val string, colType string) (interface{}, error) {
		switch colType {
		case "integer":
			v, err := strconv.ParseInt(val, 10, 64)
			return v, err
		case "real":
			v, err := strconv.ParseFloat(val, 64)
			return v, err
		}
		return val, nil
	}
	castRow := func(row []string, aux []interface{}) error {
		for i, v := range row {
			var err error
			aux[i], err = strToTableType(v, newTable.Columns[i].Datatype)
			if err != nil {
				return fmt.Errorf("unable to parse %v as %v, cause %w", v, newTable.Columns[i].Datatype, err)
			}
		}
		return nil
	}
	totalRows := int64(0)
	insertStmt := fmt.Sprintf("insert into %v(%v) values(?%v)", table, strings.Join(header, ","), strings.Repeat(",?", len(header)-1))
	aux := make([]interface{}, len(header))
	insertRow := func(row []string) error {
		err := castRow(row, aux)
		if err != nil {
			return err
		}
		_, err = c.db.ExecContext(ctx, insertStmt, aux...)
		if err != nil {
			return err
		}
		totalRows++
		return nil
	}
	err = insertRow(firstRow)
	if err != nil {
		return 0, fmt.Errorf("unable to import %v, cause %w", table, err)
	}
	reader.ReuseRecord = true
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return totalRows, nil
		}
		err = insertRow(row)
		if err != nil {
			return 0, fmt.Errorf("unable to import %v, cause %w", table, err)
		}
	}
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

func (c *Control) EnablePrivileges() error {
	if !c.writeable {
		return ReadonlyCassette{}
	}
	c.extendedPrivileges = true
	return nil
}

func (c *Control) UnsafeQuery(ctx context.Context, sql string, hasOutput bool, args ...interface{}) ([]Row, error) {
	if !c.extendedPrivileges {
		return nil, MissingExtendedPrivileges{}
	}
	if !hasOutput {
		_, err := c.db.ExecContext(ctx, sql, args...)
		if err != nil {
			err = QueryError{Query: sql, Params: args, cause: err}
		}
		return nil, err
	}
	dbrows, err := c.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, QueryError{Query: sql, Params: args, cause: err}
	}
	defer dbrows.Close()
	cols, err := dbrows.Columns()
	if err != nil {
		return nil, QueryError{Query: sql, Params: args, cause: err}
	}
	var ret []Row
	for dbrows.Next() {
		r := make(Row, len(cols))
		scanTarget := make([]interface{}, len(r))
		for i := range scanTarget {
			scanTarget[i] = &r[i]
		}
		err = dbrows.Scan(scanTarget...)
		if err != nil {
			return nil, QueryError{Query: sql, cause: err, Params: args}
		}
		ret = append(ret, r)
	}
	return ret, nil
}

func (c *Control) TableDDL(ctx context.Context, table string) (string, error) {
	def, err := loadTableDef(ctx, c.db, table)
	if err != nil {
		return "", err
	}
	return c.getDDL(ctx, *def)
}

func (c *Control) getDDL(ctx context.Context, table TableDef) (string, error) {
	if err := validDatasetTable(table.Name); err != nil {
		return "", err
	}
	createTable := bytes.Buffer{}
	fmt.Fprintf(&createTable, `create table if not exists %v(`, table.Name)
	for i, h := range table.Columns {
		if err := validDatasetColumn(h.Name); err != nil {
			return "", err
		}
		if i > 0 {
			fmt.Fprintf(&createTable, ",")
		}
		fmt.Fprintf(&createTable, "%v %v", h.Name, h.Datatype)
	}
	if len(table.PrimaryKey) > 0 {
		fmt.Fprintf(&createTable, ", primary key(%v)", strings.Join(table.PrimaryKey, ","))
	}
	fmt.Fprintf(&createTable, ");\n")
	if len(table.Unique) > 0 {
		for _, uc := range table.Unique {
			fmt.Fprintf(&createTable, "create unique index uidx_%v on %v(%v);\n", uc.Name, table.Name, strings.Join(uc.Columns, ","))
		}
	}
	return createTable.String(), nil
}

func (c *Control) CreateTable(ctx context.Context, table TableDef) (string, error) {
	if !c.writeable {
		return "", ReadonlyCassette{}
	}
	if val, _ := loadTableDef(ctx, c.db, table.Name); val != nil {
		return "", TableAlreadyExistsError{Name: table.Name}
	}
	createTable, err := c.getDDL(ctx, table)
	if err != nil {
		return "", err
	}
	_, err = c.db.ExecContext(ctx, createTable)
	if err != nil {
		return "", fmt.Errorf("unable to import %v (ddl: %v), cause %w", table, createTable, err)
	}
	return createTable, nil
}

func (c *Control) saveTuple(ctx context.Context, tx dbLike, tuple map[string]interface{}, td *TableDef) error {
	// TODO: it is night and I'm writing this because I want to finish what I started
	// This code might look very confusing!
	var insertCols []string
	for _, col := range td.Columns {
		if _, has := tuple[col.Name]; !has {
			continue
		}
		insertCols = append(insertCols, col.Name)
	}
	if len(insertCols) == 0 {
		return nil
	}
	var updateCols []string
	vals := make([]interface{}, len(insertCols))
	for i, col := range insertCols {
		if td.canUpdate(col) {
			updateCols = append(updateCols, fmt.Sprintf("%v = excluded.%v", col, col))
		}
		var err error
		vals[i], err = toDatabaseType(tuple[col])
		if err != nil {
			return err
		}
	}
	insert := &bytes.Buffer{}
	fmt.Fprintf(insert, "insert into %v (%v) values (", td.Name, strings.Join(insertCols, ","))
	for i := range insertCols {
		if i != 0 {
			insert.WriteString(",")
		}
		insert.WriteString("?")
	}
	fmt.Fprintf(insert, ") on conflict do update set %v", strings.Join(updateCols, ","))
	_, err := tx.ExecContext(ctx, insert.String(), vals...)
	return err
}

func (c *Control) nextSeq(ctx context.Context, seq string) (int64, error) {
	var val int64
	err := c.db.QueryRowContext(ctx, `insert into bb_counters (name, val) values (?, 1) on conflict do update set val = val + 1 returning val`, seq).Scan(&val)
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
	err := c.db.QueryRowContext(ctx, `select a.asset_id, a.mime_type from bb_assets a inner join bb_codebase c on c.asset_id = a.asset_id
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
		`create table if not exists bb_counters(
			name text not null primary key,
			val integer not null
		)`,
		`create table if not exists bb_assets(
			asset_id integer not null primary key,
			path text not null,
			path_hash64 integer not null,
			mime_type text not null,
			content blob not null
		)`,
		`create unique index if not exists uidx_asset_path
			on bb_assets(path)`,
		`create index if not exists idx_assets_path_hash64
			on bb_assets(path_hash64)
		`,
		`create table if not exists bb_codebase(
			asset_id integer not null primary key,
			foreign key (asset_id) references bb_assets(asset_id)
		)`,
		`create table if not exists bb_routes(
			route text not null primary key,
			methods text not null,
			asset_id integer,
			foreign key(asset_id) references bb_codebase(asset_id)
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
	_, err := c.db.ExecContext(ctx, `attach database ? as 'dataset'`, c.controlPath)
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

func validDatasetTable(name string) error {
	if !reValidIdentifiers.MatchString(name) {
		return InvalidTableName{Name: name}
	}
	return nil
}

func validDatasetColumn(name string) error {
	if !reValidIdentifiers.MatchString(name) {
		return InvalidColumnName{Name: name}
	} else if reRestrictedTables.MatchString(name) {
		return RestrictedTable{Name: name}
	}
	return nil
}

func toDatabaseType(native interface{}) (interface{}, error) {
	v := reflect.ValueOf(native)
	switch {
	case v.Kind() == reflect.String:
		return native, nil
	case v.Kind() == reflect.Bool:
		return native, nil
	case v.CanInt():
		return v.Int(), nil
	case v.CanUint():
		return int64(v.Uint()), nil
	case v.CanFloat():
		return v.Float(), nil
	case v.Kind() == reflect.Slice || v.Kind() == reflect.Array:
		// treat array of bytes as blob
		if v.Elem().Kind() == reflect.Uint8 {
			return native, nil
		}
	}
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(native)
	return strings.TrimSpace(buf.String()), err
}
