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
	defer cleanup()
	keyfn := func(context.Context) (*Key, error) {
		var k Key
		_, err := rand.Read(k[:])
		if err != nil {
			return nil, err
		}
		return &k, nil
	}
	tokens := InMemoryTokenStore()
	err := Register(ctx, tape, PlainText("user"), PlainText("password"), keyfn)
	if err != nil {
		t.Fatal(err)
	}
	token, err := Login(ctx, tokens, tape, PlainText("user"), PlainText("password"), keyfn)
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
