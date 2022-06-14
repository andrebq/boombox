package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/andrebq/boombox/cassette/programs/authprogram"
	"github.com/andrebq/boombox/internal/testutil"
	"github.com/steinfletcher/apitest"
	jsonpath "github.com/steinfletcher/apitest-jsonpath"
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
	apitest.Handler(protected).Get("/.auth/.health").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusUnauthorized).End()
	ts.Save(context.Background(), "abc123")
	apitest.Handler(protected).Get("/.auth/.health").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusOK).End()
	apitest.Handler(protected).Get("/.auth/something").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusOK).End()
	if count != 1 {
		t.Fatalf("Protected endpoing should have been called only once, got %v", count)
	}
	apitest.Handler(protected).Get("/").Header("Authorization", fmt.Sprintf("Bearer %v", "abc123")).Expect(t).Status(http.StatusNotFound).End()
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	ts := authprogram.InMemoryTokenStore()
	os.Setenv(authprogram.RootKeyEnvVar, "blmHX4evD5FygUEa3EWxjzuAPF7lC4sKuWBrhgti/20=")
	keyfn, err := authprogram.KeyFNFromEnv(authprogram.RootKeyEnvVar, os.Getenv, os.Setenv)
	if err != nil {
		t.Fatal(err)
	}
	c, done := testutil.AcquireWritableCassette(ctx, t, "auth")
	_ = c.EnablePrivileges()
	_ = authprogram.Setup(ctx, c)
	defer done()
	sr := NewRealm(c, ts, keyfn, true)
	var count int32
	handler, err := sr.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusOK)
	}), "/.auth/")
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Get("/.auth/.login").Expect(t).Status(http.StatusMethodNotAllowed).End()
	apitest.Handler(handler).Post("/.auth/.login").Body("{}").Expect(t).Status(http.StatusUnauthorized).End()
	err = authprogram.Register(ctx, c, authprogram.PlainText("user"), authprogram.PlainText("password"), keyfn, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Post("/.auth/.login").Body("{}").BasicAuth("user", "password").Expect(t).Status(http.StatusOK).Assert(
		jsonpath.Present("token")).End()
}
