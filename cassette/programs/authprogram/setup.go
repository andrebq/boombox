package authprogram

import (
	"context"
	"fmt"

	"github.com/andrebq/boombox/cassette"
)

// Setup updates the given cassette with all tables needed
// to run the auth program
func Setup(ctx context.Context, tape *cassette.Control) error {
	_, err := tape.UnsafeQuery(ctx, `create table if not exists __auth(user_id integer primary key autoincrement,
		login text not null unique,
		salt text not null unique,
		password text not null)`, false)
	if err != nil {
		return fmt.Errorf("unable to create __auth table, cause %w", err)
	}
	return nil
}
