package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/andrebq/boombox/cassette/programs/authprogram"
	"github.com/steinfletcher/apitest"
)

func TestProtect(t *testing.T) {
	ts := authprogram.InMemoryTokenStore()
	os.Setenv(authprogram.RootKeyEnvVar, "blmHX4evD5FygUEa3EWxjzuAPF7lC4sKuWBrhgti/20=")
	keyfn, err := authprogram.KeyFNFromEnv(authprogram.RootKeyEnvVar, os.Getenv, os.Setenv)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(authprogram.RootKeyEnvVar) != "" {
		t.Fatal("reading the key should remove it from the environment")
	}
	sr := NewRealm(nil, ts, keyfn, true)
	var count uint32
	protected, err := sr.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint32(&count, 1)
		http.Error(w, "OK", http.StatusOK)
	}), "/.auth")
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(protected).Get("/.auth/").Expect(t).Status(http.StatusUnauthorized).End()
	ts.Save(context.Background(), "abc123")
	apitest.Handler(protected).Get("/.auth/").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusOK).End()
	if count != 1 {
		t.Fatal("Protected endpoing should have been called only once")
	}
	apitest.Handler(protected).Get("/").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusNotFound).End()
}
