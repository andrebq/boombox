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

func KeyFNFromEnv(varname string) (KeyFn, error) {
	val := os.Getenv(varname)
	os.Setenv(varname, "")
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
