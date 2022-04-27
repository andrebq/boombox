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
	return &C{db: conn}, nil
}

func (c *C) Close() error {
	return c.db.Close()
}
