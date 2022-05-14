package cassette

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPrivilegedCassette(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := tempTape(t, "priv")
	defer cleanup()
	c, err := LoadControlCassette(ctx, tape, true, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.UnsafeQuery(ctx, "create table dataset.users(user_id integer autoincrement primary key, login text not null, token text not null)", false)
	if !errors.Is(err, MissingExtendedPrivileges{}) {
		t.Fatalf("Error should be %v got %v", MissingExtendedPrivileges{}, err)
	}
	err = c.EnablePrivileges()
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.UnsafeQuery(ctx, "create table dataset.users(user_id integer primary key autoincrement, login text not null, token text not null)", false)
	if err != nil {
		t.Fatalf("UnsafeQuery should be allowed at this point, but got %v", err)
	}
	rows, err := c.UnsafeQuery(ctx, "insert into dataset.users(login, token) values (?, ?) returning user_id", true, "bob", "bob-super-secret")
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(rows, []Row{{int64(1)}}) {
		t.Fatalf("Unexpected data from query, got %v", rows)
	}
	defer c.Close()
}
