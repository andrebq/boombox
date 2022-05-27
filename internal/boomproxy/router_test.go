package boomproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	authapi "github.com/andrebq/boombox/cassette/programs/authprogram/api"
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
	handler, _ := AsHandler(ctx, apiCalls, queryCalls, nil)

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

func TestAuthRouter(t *testing.T) {
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

	var authenticatedCount int
	authenticatedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedCount++
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.Background()
	queryCalls, _ := url.Parse(queryServer.URL)
	apiCalls, _ := url.Parse(apiServer.URL)
	authenticatedCalls, _ := url.Parse(authenticatedServer.URL + "/.admin/")
	handler, _ := AsHandler(ctx, apiCalls, queryCalls, authenticatedCalls)

	apitest.Handler(handler).Get("/hello/.query").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/index.html").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/hello/index.html").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/.admin/index.html").Expect(t).Status(http.StatusOK).End()

	if queryCount != 1 {
		t.Fatal("Invalid query count: ", queryCount)
	}
	if apiCount != 2 {
		t.Fatal("Invalid api count: ", apiCount)
	}
	if authenticatedCount != 1 {
		t.Fatal("Invalid authenticated count: ", authenticatedCount)
	}
}

func TestAuthWithoutPath(t *testing.T) {
	publicURL, _ := url.Parse("http://example.com")
	invalidAuthURL, _ := url.Parse("http://example.com/")

	_, err := AsHandler(context.Background(), publicURL, publicURL, invalidAuthURL)
	if !errors.Is(err, authapi.AuthURLWithoutPath{Prefix: invalidAuthURL.String()}) {
		t.Fatalf("Unexpected error for invalid auth url: %v", err)
	}
}
