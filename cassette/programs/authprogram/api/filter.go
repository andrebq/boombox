package api

import (
	"net/http"
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
)

var (
	bearerTokenRE = regexp.MustCompile(`^Bearer ([^\s]+)$`)
)

func NewRealm(tape *cassette.Control, tokens authprogram.TokenStore, keyfn authprogram.KeyFn, allowHTTPCookie bool) *SecurityRealm {
	return &SecurityRealm{
		tape:           tape,
		tokenset:       tokens,
		keyfn:          keyfn,
		insecureCookie: allowHTTPCookie,
	}
}

func (s *SecurityRealm) Protect(sensitive http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		sensitive.ServeHTTP(w, r)
	})
}

func (s *SecurityRealm) checkToken(r *http.Request) bool {
	ctx := r.Context()
	log := logutil.GetOrDefault(ctx)
	hdrVal := r.Header.Get("Authorization")
	groups := bearerTokenRE.FindStringSubmatch(hdrVal)
	if len(groups) == 0 {
		println("token not found", hdrVal)
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
