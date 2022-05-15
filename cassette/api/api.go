package api

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/lua/bindings/httplua"
	"github.com/andrebq/boombox/internal/lua/ltoj"
	"github.com/andrebq/boombox/internal/lua/luadefaults"
	"github.com/julienschmidt/httprouter"
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

func AsPrivilegedHandler(ctx context.Context, c *cassette.Control, tapemodule lua.LGFunction) (http.Handler, error) {
	if !c.Queryable() && !c.HasPrivileges() {
		return nil, cassette.MissingExtendedPrivileges{}
	}
	return asHandler(ctx, c, tapemodule)
}

func AsHandler(ctx context.Context, c *cassette.Control, tapedeckModule lua.LGFunction) (http.Handler, error) {
	if !c.Queryable() {
		// even though we don't expose query capabilities directly,
		// AsHandler will not work with cassettes that could be written
		// as it is not safe to exposed them directly.
		//
		// TODO: add the AsPrivilegedHandler to enable a writable cassette
		// to be queried
		return nil, cassette.CannotQuery{}
	}
	return asHandler(ctx, c, tapedeckModule)
}

func asHandler(ctx context.Context, c *cassette.Control, tapemodule lua.LGFunction) (http.Handler, error) {
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
			router.HandlerFunc(m, apiRoute, serveDynamicCode(r.Code, tapemodule))
		}
	}

	return router, nil
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

func serveDynamicCode(code string, tapedeckModule lua.LGFunction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeoutCtx, cancel := context.WithTimeout(r.Context(), time.Second*10)
		defer cancel()
		r = r.WithContext(timeoutCtx)
		L := lua.NewState(lua.Options{
			SkipOpenLibs: true,
		})
		L.SetContext(r.Context())
		defer L.Close()
		luadefaults.InjectDynamicCodeLibs(L)
		L.PreloadModule("ctx", httplua.OpenServer(w, r))
		L.PreloadModule("json", ltoj.OpenModule())
		if tapedeckModule != nil {
			L.PreloadModule("tapedeck", tapedeckModule)
		}
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
