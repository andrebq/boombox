package api

import (
	"fmt"
	"net/http"
	"path"
	"regexp"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/cassette/programs/authprogram"
	"github.com/andrebq/boombox/internal/logutil"
)

type (
	SecurityRealm struct {
		tape           *cassette.Control
		tokenset       authprogram.TokenStore
		keyfn          authprogram.KeyFn
		insecureCookie bool
	}
	AuthURLWithoutPath struct {
		Prefix string
	}
)

var (
	bearerTokenRE = regexp.MustCompile(`^Bearer ([^\s]+)$`)
)

func (m AuthURLWithoutPath) Error() string {
	return fmt.Sprintf("authenticated urls must have a non-empty path, got %v", m.Prefix)
}

func NewRealm(tape *cassette.Control, tokens authprogram.TokenStore, keyfn authprogram.KeyFn, allowHTTPCookie bool) *SecurityRealm {
	return &SecurityRealm{
		tape:           tape,
		tokenset:       tokens,
		keyfn:          keyfn,
		insecureCookie: allowHTTPCookie,
	}
}

func (s *SecurityRealm) Protect(sensitive http.Handler, prefix string) (http.Handler, error) {
	prefix = path.Clean(prefix)
	if len(prefix) == 0 || prefix == "/" {
		return nil, AuthURLWithoutPath{Prefix: prefix}
	}
	mux := http.NewServeMux()
	mux.Handle(fmt.Sprintf("%v/", prefix), sensitive)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}

func (s *SecurityRealm) checkToken(r *http.Request) bool {
	ctx := r.Context()
	log := logutil.GetOrDefault(ctx)
	hdrVal := r.Header.Get("Authorization")
	groups := bearerTokenRE.FindStringSubmatch(hdrVal)
	if len(groups) == 0 {
		return false
	}
	tk := groups[1]
	found, err := s.tokenset.Lookup(ctx, tk)
	if err != nil {
		log.Error().Err(err).Msg("Unexpected error when checking for token in Token set")
		return false
	}
	return found
}
