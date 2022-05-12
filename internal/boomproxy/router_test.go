package boomproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/steinfletcher/apitest"
)

func TestRouter(t *testing.T) {
	var queryCount int
	queryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer queryServer.Close()

	var apiCount int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	ctx := context.Background()
	queryCalls, _ := url.Parse(queryServer.URL)
	apiCalls, _ := url.Parse(apiServer.URL)
	handler := AsHandler(ctx, apiCalls, queryCalls)

	apitest.Handler(handler).Get("/hello/.query").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/index.html").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/hello/index.html").Expect(t).Status(http.StatusOK).End()

	if queryCount != 1 {
		t.Fatal("Invalid query count: ", queryCount)
	}
	if apiCount != 2 {
		t.Fatal("Invalid api count: ", apiCount)
	}
}
