package authprogram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"runtime"

	"github.com/andrebq/boombox/cassette"
	"golang.org/x/crypto/argon2"
)

type (
	PlainText        []byte
	HashText         []byte
	CipherText       []byte
	Key              [32]byte
	ExtendedPassword [48]byte
	PasswdSecret     [16]byte

	KeyFn func(context.Context, *Key) error
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

func (e *ExtendedPassword) Zero() {
	for i := range e {
		e[i] = 0
	}
}

func (e *ExtendedPassword) Prefix() HashText {
	return (*e)[:32]
}

func (e *ExtendedPassword) Suffix() HashText {
	return (*e)[32:]
}

var (
	userNameRatchetStep = []byte("username")
)

func Register(ctx context.Context, tape *cassette.Control, user, passwd PlainText, rootKeyFn KeyFn, randomReader io.Reader) error {
	var passwdSalt [8]byte
	_, err := io.ReadFull(randomReader, passwdSalt[:])
	if err != nil {
		return err
	}

	var rootKey Key
	defer rootKey.Zero()
	err = rootKeyFn(ctx, &rootKey)
	if err != nil {
		return err
	}
	r := Ratchet{key: &rootKey}
	// advance the ratchet to generate the username hmac key
	defer r.Next(userNameRatchetStep).Zero()
	userHash := computeHmac(r.key, user)

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
	passwdHash := computeHmac(r.key, PlainText(exPasswd.Suffix()))

	_, err = tape.UnsafeQuery(ctx, "insert into __auth(login, salt, password) values (?, ?, ?) on conflict (login) do update set password = EXCLUDED.password, salt = EXCLUDED.salt", false,
		base64.URLEncoding.EncodeToString(userHash),
		base64.URLEncoding.EncodeToString(passwdSalt[:]),
		base64.URLEncoding.EncodeToString(passwdHash))
	if err != nil {
		return fmt.Errorf("unable to save record to __auth table, cause %w", err)
	}
	return nil
}

func extendPassword(out *ExtendedPassword, passwd PlainText, salt PlainText) {
	buf := argon2.IDKey(passwd, salt, 64, 1024, uint8(runtime.NumCPU()), uint32(len(*out)))
	copy((*out)[:], buf)
}

func computeHmac(key *Key, buf PlainText) HashText {
	mac := hmac.New(sha256.New, (*key)[:])
	mac.Write(buf)
	return HashText(mac.Sum(nil))
}
