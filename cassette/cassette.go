package cassette

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type (
	C struct {
		db *sql.DB
	}
)

func LoadControlCassette(ctx context.Context, tape string) (*C, error) {
	conn, err := sql.Open("sqlite3", fmt.Sprintf("file:%v?_writable_schema=false&_journal=wal&mode=rwc", tape))
	if err != nil {
		return nil, fmt.Errorf("unable to open %v, cause %v", tape, err)
	}
	err = conn.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to ping cassette %v, cause %v", tape, err)
	}
	c := &C{db: conn}
	err = c.init(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to init cassette %v, cause %v", tape, err)
	}
	return c, nil
}

func (c *C) init(ctx context.Context) error {
	for _, cmd := range []string{
		`create table if not exists assets(
			asset_id integer not null,
			path text not null,
			path_hash integer not null,
			content blob not null
		)`,
	} {
		_, err := c.db.ExecContext(ctx, cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *C) Close() error {
	return c.db.Close()
}
