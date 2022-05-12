package boomproxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/julienschmidt/httprouter"
)

var (
	methods = []string{
		"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD",
	}
)

func AsHandler(ctx context.Context, apiCalls *url.URL, queryCalls *url.URL) http.Handler {
	router := httprouter.New()

	apiProxy := httputil.NewSingleHostReverseProxy(apiCalls)
	queryProxy := httputil.NewSingleHostReverseProxy(queryCalls)

	router.Handler("GET", "/:cassette/.query", queryProxy)

	// delegate to apiProxy if not found
	router.NotFound = apiProxy

	return router
}
