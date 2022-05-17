package authprogram

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/andrebq/boombox/internal/testutil"
)

func TestLogin(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	if err := tape.EnablePrivileges(); err != nil {
		t.Fatal(err)
	}
	if err := Setup(ctx, tape); err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var rootKey Key
	_, err := rand.Read(rootKey[:])
	if err != nil {
		t.Fatal(err)
	}
	keyfn := func(ctx context.Context, out *Key) error {
		copy((*out)[:], rootKey[:])
		return nil
	}
	tokens := InMemoryTokenStore()
	err = Register(ctx, tape, PlainText("user"), PlainText("password"), keyfn, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	token, err := Login(ctx, tokens, tape, PlainText("user"), PlainText("password"), keyfn, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	found, err := tokens.Lookup(ctx, token)
	if err != nil {
		t.Fatal(err)
	} else if !found {
		t.Fatal("token not found on storage")
	}
}
