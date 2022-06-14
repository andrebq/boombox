package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"

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
	mux.Handle(fmt.Sprintf("%v/.login", prefix), http.HandlerFunc(s.performLogin))
	mux.Handle(fmt.Sprintf("%v/", prefix), http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		sensitive.ServeHTTP(w, r)
	})))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logutil.GetOrDefault(r.Context())
		log.Info().Stringer("url", r.URL).Send()
		mux.ServeHTTP(w, r)
	}), nil
}

func (s *SecurityRealm) performLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Please use POST", http.StatusMethodNotAllowed)
		return
	}
	user, passwd, ok := req.BasicAuth()
	if !ok {
		http.Error(w, "Missing username/password credentials", http.StatusUnauthorized)
		return
	}
	token, err := authprogram.Login(req.Context(), s.tokenset, s.tape, authprogram.PlainText(user),
		authprogram.PlainText(passwd), s.keyfn, rand.Reader)
	if err != nil {
		log := logutil.GetOrDefault(req.Context())
		log.Error().Err(err).Msg("Unable to authenticate user")
	}
	buf, _ := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})

	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Content-Length", strconv.Itoa(len(buf)))
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
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
