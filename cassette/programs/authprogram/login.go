package authprogram

import (
	"context"
	"errors"

	"github.com/andrebq/boombox/cassette"
)

type (
	TokenStore interface {
		Save(ctx context.Context, token string) error
		Lookup(ctx context.Context, token string) (bool, error)
	}
)

func Login(ctx context.Context, tokens TokenStore, tape *cassette.Control, user, passwd PlainText, keyfn KeyFn) (token string, err error) {
	err = errors.New("not implemented")
	return
}
