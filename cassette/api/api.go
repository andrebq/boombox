package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/cassette/importer"
	"github.com/andrebq/boombox/internal/logutil"
	"github.com/andrebq/boombox/internal/lua/bindings/httplua"
	"github.com/andrebq/boombox/internal/lua/ltoj"
	"github.com/andrebq/boombox/internal/lua/luadefaults"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	routeListTemplate = template.Must(template.New("__root__").Parse(`
<!doctype html>
<html>
  <head>
	<title>Cassette routes</title>
  </head>
  <body>
    <h1>List of routes from cassette</h1>
    <ul>
      {{ range .Routes }}
      <li>
        <dl>
          <dt>Route</dt>
          <dt>{{ .Route }}</dt>
          <dt>Methods</dt>
          {{ range $idx, $value := .Methods }}{{ $value }}{{ end }}
          <dt>Code path</dt>
          <dt>{{ .CodePath }}</dt>
        </dl>
      </li>
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

	routeListTemplateModel struct {
		Routes []cassette.Code
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
	// Logic
	// 1- Register /.internals/ paths as those have priority
	// 2- If not found, lookup the API
	// 3- If not found, lookup assets

	router := httprouter.New()
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false

	router.HandlerFunc("GET", "/.internals/asset-list", listAssets(c))
	router.HandlerFunc("GET", "/.internals/dynamic-routes", listRoutes(c))
	if c.HasPrivileges() {
		router.HandlerFunc("PUT", "/.internals/write-asset/*assetPath", writeAsset(c))
		router.HandlerFunc("PUT", "/.internals/enable-code/*assetPath", enableCodebase(ctx, c))
		router.HandlerFunc("POST", "/.internals/set-route", setRoute(ctx, c))
		router.HandlerFunc("POST", "/.internals/ddl/create/table/:table", createTable(ctx, c))
	}
	router.NotFound = apiOrAssetHandler(ctx, c, tapemodule)
	return router, nil
}

func createTable(ctx context.Context, c *cassette.Control) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tableName := httprouter.ParamsFromContext(r.Context()).ByName("table")
		var payload cassette.TableDef
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if tableName != payload.Name && payload.Name != "" {
			http.Error(w, "table name from payload does not match url path", http.StatusBadRequest)
			return
		}
		payload.Name = tableName
		_, err = c.CreateTable(r.Context(), payload)
		var tae cassette.TableAlreadyExistsError
		if errors.As(err, &tae) {
			http.Error(w, "table already exists", http.StatusConflict)
			return
		} else if err != nil {
			log.Error().Err(err).Interface("tdf", payload).Msg("Unable to create table")
			http.Error(w, "Unexpected internal error, please check logs for more information", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func setRoute(ctx context.Context, c *cassette.Control) http.HandlerFunc {
	log := logutil.GetOrDefault(ctx).With().Str("action", "set-route").Logger().Sample(zerolog.Sometimes)
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Route   string   `json:"route"`
			Asset   string   `json:"asset"`
			Methods []string `json:"methods"`
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.Asset) == 0 {
			http.Error(w, "Missing asset path", http.StatusBadRequest)
			return
		}
		payload.Asset = strings.TrimLeft(path.Clean(payload.Asset), "/")
		err = c.MapRoute(r.Context(), payload.Methods, payload.Route, payload.Asset)
		if err != nil {
			log.Error().Err(err).Str("assetPath", payload.Asset).Strs("methods", payload.Methods).Str("route", payload.Route).Msg("Unable to set route")
			http.Error(w, "Unexpected internal error, please check logs for more information", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func enableCodebase(ctx context.Context, c *cassette.Control) http.HandlerFunc {
	log := logutil.GetOrDefault(ctx).With().Str("action", "enableCodebase").Logger().Sample(zerolog.Sometimes)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ap := httprouter.ParamsFromContext(ctx).ByName("assetPath")
		if len(ap) == 0 {
			http.Error(w, "Missing asset path", http.StatusBadRequest)
			return
		}
		ap = strings.TrimLeft(ap, "/")
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var enable bool
		switch val := r.FormValue("enabled"); val {
		case "y", "yes", "1", "true", "t":
			enable = true
		case "n", "no", "0", "false", "f":
			enable = false
		default:
			http.Error(w, fmt.Sprintf("Invalid value [%v] for enabled parameter. Must be one of [y/n]", val), http.StatusBadRequest)
			return
		}
		err = c.ToggleCodebase(ctx, ap, enable)
		if err != nil {
			// TODO: handle not found gracefully
			log.Error().Err(err).Str("assetPath", ap).Bool("enable", enable).Msg("Unable to change codebase config")
			http.Error(w, "Unexpected internal error, please check logs for more information", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func apiOrAssetHandler(rootCtx context.Context, c *cassette.Control, tape lua.LGFunction) http.Handler {
	serveStaticAsset := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasSuffix(p, "/") {
			// Cassettes do not support directory assets
			// if the prefix ends with a `/`, then serve
			// index.html instead
			p = fmt.Sprintf("%v/index.html", p)
		}
		// All assets in a Cassette are relative
		p = strings.TrimLeft(p, "/")
		serveAsset(c, p).ServeHTTP(w, r)
	})
	newRouter := make(chan *httprouter.Router)
	go monitorDynamicRoutes(rootCtx, c, tape, newRouter, serveStaticAsset)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case router, open := <-newRouter:
			if !open {
				http.Error(w, "Server went is in a invalid state, please try again later", http.StatusInternalServerError)
				return
			}
			router.ServeHTTP(w, r)
		case <-r.Context().Done():
		}
	})
}

func writeAsset(c *cassette.Control) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		assetPath := httprouter.ParamsFromContext(ctx).ByName("assetPath")
		assetPath = path.Clean(assetPath)
		switch {
		case len(assetPath) == 0:
			http.Error(w, "Missing assetPath information", http.StatusBadRequest)
			return
		case assetPath[len(assetPath)-1] == '/':
			http.Error(w, "Cannot write a directory, upload files individually", http.StatusBadRequest)
			return
		}
		ext := path.Ext(assetPath)
		if len(ext) == 0 {
			http.Error(w, "Extension is required for assets", http.StatusBadRequest)
			return
		}
		mt := importer.MimetypeFromExtension(ext)
		content, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "Unable to read the whole request body", http.StatusBadRequest)
			return
		}
		assetPath = strings.TrimLeft(assetPath, "/")
		_, err = c.StoreAsset(ctx, assetPath, mt, string(content))
		if err != nil {
			http.Error(w, "Unable to store asset in database", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
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

func listRoutes(c *cassette.Control) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routes, err := c.ListRoutes(r.Context())
		if err != nil {
			http.Error(w, "Unable to fetch list of routes, please try again later", http.StatusInternalServerError)
			return
		}
		routeListTemplate.Execute(w, routeListTemplateModel{Routes: routes})
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
		var notFound cassette.AssetNotFound
		if errors.As(err, &notFound) {
			http.Error(w, fmt.Sprintf("not found: %v", assetPath), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "unable to fetch desired asset, server is mis-behaving", http.StatusInternalServerError)
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

func monitorDynamicRoutes(ctx context.Context, c *cassette.Control, tapedeck lua.LGFunction, output chan<- *httprouter.Router, notfound http.Handler) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	loadRouter := func(ctx context.Context, c *cassette.Control, deck lua.LGFunction) (*httprouter.Router, error) {
		routes, err := c.ListRoutes(ctx)
		if err != nil {
			return nil, err
		}
		router := httprouter.New()
		router.NotFound = notfound
		for _, r := range routes {
			for _, method := range r.Methods {
				router.HandlerFunc(method, r.Route, serveDynamicCode(r.Code, tapedeck))
			}
		}
		return router, nil
	}
	defer close(output)
	t := time.NewTicker(time.Second)
	defer t.Stop()
	currentRouter, err := loadRouter(ctx, c, tapedeck)
	if err != nil {
		return
	}
	newRouter := make(chan *httprouter.Router, 1)
	for {
		select {
		case <-ctx.Done():
		case <-t.C:
			// TODO: make this less stupid!!!!
			// This will reload all routes every 1 second... even if there were no changes
			// find a way to detect a change and only update routes if a change is dected,
			// but as everything else in this project, I'm to lazy to do it properly now
			go func(ctx context.Context) {
				r, err := loadRouter(ctx, c, tapedeck)
				if err != nil {
					cancel()
				}
				select {
				case newRouter <- r:
				case <-ctx.Done():
				default:
					// TODO: since we are blindingly writing to the channel, at least avoid go-routine explosion
				}
			}(ctx)
		case output <- currentRouter:
			// send the current output, in case anyone is waiting for it
		case currentRouter = <-newRouter:
		}
	}
}
