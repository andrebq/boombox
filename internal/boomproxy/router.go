package boomproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"

	authapi "github.com/andrebq/boombox/cassette/programs/authprogram/api"
	"github.com/julienschmidt/httprouter"
)

var (
	methods = []string{
		"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD",
	}
)

func AsHandler(ctx context.Context, apiCalls *url.URL, queryCalls *url.URL, authenticatedCalls *url.URL) (http.Handler, error) {
	router := httprouter.New()

	apiProxy := httputil.NewSingleHostReverseProxy(apiCalls)
	queryProxy := httputil.NewSingleHostReverseProxy(queryCalls)
	var authProxy http.Handler
	var authPrefix string
	if authenticatedCalls != nil {
		authPrefix = authenticatedCalls.Path
		if authPrefix == "/" || len(authPrefix) == 0 {
			return nil, authapi.AuthURLWithoutPath{Prefix: authenticatedCalls.String()}
		}
		authProxy = httputil.NewSingleHostReverseProxy(authenticatedCalls)
	}

	apiRouter := httprouter.New()

	if authProxy != nil {
		authPrefix = fmt.Sprintf("%v/*tail", path.Clean(authPrefix))
		for _, v := range methods {
			apiRouter.Handler(v, authPrefix, authProxy)
		}
		apiRouter.NotFound = apiProxy
	} else {
		for _, v := range methods {
			apiRouter.Handler(v, "/*tail", apiProxy)
		}
	}

	router.Handler("GET", "/:cassette/.query", queryProxy)

	// delegate to apiProxy if not found
	router.NotFound = apiRouter

	return router, nil
}
