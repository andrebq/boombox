package boomproxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/julienschmidt/httprouter"
)

func AsHandler(ctx context.Context, apiCalls *url.URL, queryCalls *url.URL, authenticatedCalls *url.URL) http.Handler {
	router := httprouter.New()

	apiProxy := httputil.NewSingleHostReverseProxy(apiCalls)
	queryProxy := httputil.NewSingleHostReverseProxy(queryCalls)
	var authProxy http.Handler
	if authenticatedCalls != nil {
		authProxy = httputil.NewSingleHostReverseProxy(authenticatedCalls)
	}

	router.Handler("GET", "/:cassette/.query", queryProxy)

	// delegate to apiProxy if not found
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			if authProxy == nil {
				http.Error(w, "Boombox does not allow authenticated calls", http.StatusForbidden)
				return
			}
			authProxy.ServeHTTP(w, r)
			return
		}
		apiProxy.ServeHTTP(w, r)
	})

	return router
}
