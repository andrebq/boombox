package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/andrebq/boombox/cassette"
	"github.com/julienschmidt/httprouter"
)

func AsHandler(ctx context.Context, c *cassette.Control) (http.Handler, error) {
	assets, err := c.ListAssets(ctx)
	if err != nil {
		return nil, err
	}

	// using reverse sort makes longer paths to appear before smaller ones
	// which makes handling `index.html` default much simpler
	sort.Sort(sort.Reverse(sort.StringSlice(assets)))

	router := httprouter.New()
	for _, s := range assets {
		router.HandlerFunc("GET", fmt.Sprintf("/%v", s), serveAsset(c, s))
		if path.Base(s) == "index.html" {
			dir := path.Dir(s)
			if dir == "." {
				dir = "/"
			}
			switch {
			case !strings.HasPrefix(dir, "/") && !strings.HasSuffix(dir, "/"):
				dir = fmt.Sprintf("/%v/", dir)
			case !strings.HasPrefix(dir, "/"):
				dir = fmt.Sprintf("/%v", dir)
			case !strings.HasSuffix(dir, "/"):
				dir = fmt.Sprintf("%v/", dir)
			}
			router.HandlerFunc("GET", dir, serveAsset(c, s))
		}
	}
	return router, nil
}

func serveAsset(c *cassette.Control, assetPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		// TODO: copying to memory instead of directly to the network stream
		// while it may sound stupid, releasing the lock is more important than client latency,
		// it is fine if the client needs to wait a couple of extra millis if that means
		// less lock contention at the database layer
		_, mt, err := c.CopyAsset(r.Context(), &buf, assetPath)
		if err != nil {
			http.Error(w, "unable to fetch desired asset, server is mis-behaving", http.StatusBadGateway)
		}
		// TODO: this is absurdly stupid since we could extract this info directly from the database
		// but I'm on a rush to get this working...
		if utf8.Valid(buf.Bytes()) {
			w.Header().Add("Content-Type", fmt.Sprintf("%v; charset=utf-8", mt))
		} else {
			w.Header().Add("Content-Type", mt)
		}
		w.Header().Add("Content-Length", strconv.Itoa(buf.Len()))
		// TODO: think about some caching with ETags and stuff
		// the data is already here and the database lock was released a long time ago
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}
}
