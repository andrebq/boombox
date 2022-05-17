package authprogram

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
)

const (
	RootKeyEnvVar = "BOOMBOX_AUTH_ROOTKEY"
)

func KeyFNFromEnv(varname string, getfn func(string) string, setfn func(string, string) error) (KeyFn, error) {
	if getfn == nil {
		getfn = os.Getenv
	}
	if setfn == nil {
		setfn = os.Setenv
	}
	val := getfn(varname)
	setfn(varname, "")
	var rootKey Key
	sz, err := base64.StdEncoding.Decode(rootKey[:], []byte(val))
	if err != nil {
		return nil, fmt.Errorf("authprogram: cannot decode string to valid key, cause %v", err)
	} else if sz != len(rootKey) {
		return nil, fmt.Errorf("authprogram: decoded key too short got %v expecting %v bytes", sz, len(rootKey))
	}
	return func(_ context.Context, k *Key) error {
		copy((*k)[:], rootKey[:])
		return nil
	}, nil
}
