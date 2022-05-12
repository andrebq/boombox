package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/logutil"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

// AsQueryHandler allows arbitrary queries to cassettes
func AsQueryHandler(ctx context.Context, c *cassette.Control) (http.Handler, error) {
	router := httprouter.New()
	if !c.Queryable() {
		return nil, cassette.CannotQuery{}
	}
	router.HandlerFunc("GET", "/.query", queryCassette(ctx, c))
	return router, nil
}

func queryCassette(ctx context.Context, c *cassette.Control) http.HandlerFunc {
	const (
		OneMegabyte = 1_000_000
		MaxBuffer   = OneMegabyte
	)
	log := logutil.GetOrDefault(ctx).Sample(zerolog.Often)
	// TODO: this endpoint should ran under a separate user and process
	// but for now, let's make everything available under the same process (everything is readonly so far...)
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// TODO: handle sql query parameters, for now, deal with unparameterized queries
		sql := r.FormValue("sql")
		if len(sql) == 0 {
			http.Error(w, "missing sql parameter", http.StatusBadRequest)
			return
		}
		userMaxBuffer, err := strconv.Atoi(r.FormValue("maxBuffer"))
		if err != nil || userMaxBuffer > MaxBuffer {
			userMaxBuffer = MaxBuffer
		}
		// TODO: 10 seconds might be considered too generous for a sqlite query
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var buf bytes.Buffer
		err = c.Query(ctx, &buf, userMaxBuffer, sql)
		if err != nil {
			log.Warn().Err(err).Str("sql", sql).Msg("unable to perform query")
			var writeOverflow cassette.WriteOverflow
			if errors.As(err, &writeOverflow) {
				// TODO: in theory, the request is small but the response is too big, not good but also not horribly incorrect
				http.Error(w, "unable to perform query, your query returns too much data", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "unable to perform query, check logs for more information", http.StatusBadRequest)
			}
			return
		}
		w.Header().Add("Content-Length", strconv.Itoa(len(buf.Bytes())))
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		io.Copy(w, &buf)
	}
}
