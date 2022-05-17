package authprogram

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/andrebq/boombox/cassette"
)

type (
	CredMismatch struct{}
	TokenStore   interface {
		Save(ctx context.Context, token string) error
		Lookup(ctx context.Context, token string) (bool, error)
	}
)

func (c CredMismatch) Error() string {
	return "authprogram: credentials do not match"
}

func Login(ctx context.Context, tokens TokenStore, tape *cassette.Control, user, passwd PlainText, keyfn KeyFn, randRead io.Reader) (string, error) {
	var err error
	var rootKey Key
	if err = keyfn(ctx, &rootKey); err != nil {
		return "", err
	}
	r := Ratchet{key: &rootKey}

	// advance the ratchet to generate the username hmac key
	defer r.Next(userNameRatchetStep).Zero()
	userHash := computeHmac(r.key, user)

	rows, err := tape.UnsafeQuery(ctx, `select password, salt from __auth where login = ?`, true, base64.URLEncoding.EncodeToString(userHash))
	if err != nil {
		return "", fmt.Errorf("unable to loookup user in __auth table, cause %w", err)
	} else if len(rows) == 0 {
		return "", errors.New("user not found")
	}

	passwdHash, err := base64.URLEncoding.DecodeString(fmt.Sprintf("%v", rows[0][0]))
	if err != nil {
		return "", errors.New("corrupted database, password is not base64 url encoded")
	}
	passwdSalt, err := base64.URLEncoding.DecodeString(fmt.Sprintf("%v", rows[0][1]))
	if err != nil {
		return "", errors.New("corrupted database, salt is not base64 url encoded")
	}

	// advance the ratchet using the previous hmac
	// meaning the same key will never be used by
	// two distinct users
	defer r.Next(userHash).Zero()

	// extend the password to the required length
	var exPasswd ExtendedPassword
	defer exPasswd.Zero()
	extendPassword(&exPasswd, passwd, passwdSalt[:])

	// advanced the ratchet using the
	// extended password key
	defer r.Next(exPasswd.Prefix()).Zero()

	// compute the hmac key of the password from the
	// extended password suffix
	passwdHashFromDB := computeHmac(r.key, PlainText(exPasswd.Suffix()))
	if !hmac.Equal(passwdHash, passwdHashFromDB) {
		return "", CredMismatch{}
	}
	var token [32]byte
	_, err = io.ReadFull(randRead, token[:])
	if err != nil {
		return "", fmt.Errorf("unable to compute new token: %w", err)
	}
	tokenStr := base64.URLEncoding.EncodeToString(token[:])
	err = tokens.Save(ctx, tokenStr)
	if err != nil {
		return "", fmt.Errorf("unable to save token to database: %w", err)
	}
	return tokenStr, nil
}
