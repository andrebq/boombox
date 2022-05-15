package authprogram

import (
	"context"
	"errors"

	"github.com/andrebq/boombox/cassette"
)

type (
	PlainText  []byte
	HashText   []byte
	CipherText []byte
	Key        [32]byte

	KeyFn func(context.Context) (*Key, error)
)

func (p PlainText) Zero() {
	for i := range p {
		p[i] = 0
	}
}

func (k *Key) Zero() {
	for i := range k {
		k[i] = 0
	}
}

func Register(ctx context.Context, tape *cassette.Control, user, passwd PlainText, rootKey func(context.Context) (*Key, error)) error {
	return errors.New("not implemented")
}
