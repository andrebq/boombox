package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/logutil"
	"github.com/andrebq/boombox/internal/lua/bindings/httplua"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	lua "github.com/yuin/gopher-lua"
)

var (
	assetListTemplate = template.Must(template.New("__root__").Parse(`
<!doctype html>
<html>
  <head>
	<title>Cassette assets</title>
  </head>
  <body>
    <h1>List of assets from cassette</h1>
    <ul>
      {{ range .Assets }}
      <li><a href="../{{.}}" target="_self">{{.}}</a></li>
      {{ end }}
    </ul>
  </body>
</html>
`))
)

type (
	assetListTemplateModel struct {
		Assets []string
	}
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

	router.HandlerFunc("GET", "/.internals/asset-list", listAssets(c))

	routes, err := c.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range routes {
		apiRoute := path.Join("/api", r.Route)
		for _, m := range r.Methods {
			router.HandlerFunc(m, apiRoute, serveDynamicCode(r.Code))
		}
	}

	if c.Queryable() {
		router.HandlerFunc("GET", "/.query", queryCassette(ctx, c))
	}
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
		// TODO: 10 seconds might be considered too generous for a sqlite query
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sql := r.FormValue("sql")
		if len(sql) == 0 {
			http.Error(w, "missing sql parameter", http.StatusBadRequest)
			return
		}
		userMaxBuffer, err := strconv.Atoi(r.FormValue("maxBuffer"))
		if err != nil || userMaxBuffer > MaxBuffer {
			userMaxBuffer = MaxBuffer
		}
		// TODO: handle sql query parameters, for now, deal with unparameterized queries
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

func listAssets(c *cassette.Control) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.ListAssets(r.Context())
		if err != nil {
			http.Error(w, "Unable to fetch list of assets, please try again later", http.StatusInternalServerError)
			return
		}
		assetListTemplate.Execute(w, assetListTemplateModel{Assets: items})
	}
}

func serveDynamicCode(code string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeoutCtx, cancel := context.WithTimeout(r.Context(), time.Second*10)
		defer cancel()
		r = r.WithContext(timeoutCtx)
		L := lua.NewState(lua.Options{
			SkipOpenLibs: true,
		})
		L.SetContext(r.Context())
		defer L.Close()
		for _, pair := range []struct {
			n string
			f lua.LGFunction
		}{
			{lua.LoadLibName, lua.OpenPackage}, // Must be first
			{lua.BaseLibName, lua.OpenBase},
			{lua.TabLibName, lua.OpenTable},
		} {
			if err := L.CallByParam(lua.P{
				Fn:      L.NewFunction(pair.f),
				NRet:    0,
				Protect: true,
			}, lua.LString(pair.n)); err != nil {
				panic(err)
			}
		}
		L.PreloadModule("ctx", httplua.OpenServer(w, r))
		err := L.DoString(code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Dynamic page failed with unexpected error:\n%v\n\n\n----\n\n\n%v", err, code), http.StatusBadGateway)
		}
	}
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
		if mt == "text/x-lua" {
			mt = "text/plain"
		}
		// TODO: this is absurdly stupid since we could extract this info directly from the database
		// but I'm on a rush to get this working...
		if utf8.Valid(buf.Bytes()) {
			w.Header().Add("Content-Type", fmt.Sprintf("%v; charset=utf-8", mt))
		} else {
			w.Header().Add("Content-Type", mt)
		}
		switch mt {
		case "text/plain":
			w.Header().Add("Content-Disposition", "inline")
		}
		w.Header().Add("Content-Length", strconv.Itoa(buf.Len()))
		// TODO: think about some caching with ETags and stuff
		// the data is already here and the database lock was released a long time ago
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}
}
