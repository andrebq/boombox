package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrebq/boombox/cassette"
	"github.com/steinfletcher/apitest"
)

func TestApi(t *testing.T) {
	ctx := context.Background()
	cassette, cleanup := tempCassette(ctx, t, "test")
	defer cleanup()
	_, err := cassette.StoreAsset(ctx, "index.html", "text/html", `<h1>it works</h1>`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cassette.StoreAsset(ctx, "nested/folder/index.html", "text/html", `<h1>it works</h1>`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cassette.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local res = ctx.res
	res:write_body('hello from lua')
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = cassette.ToggleCodebase(ctx, "codebase/index.lua", true)
	if err != nil {
		t.Fatal(err)
	}
	err = cassette.MapRoute(ctx, []string{"GET"}, "/hello-from-lua", "codebase/index.lua")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AsHandler(ctx, cassette)
	if err != nil {
		t.Fatal(err)
	}

	apitest.New().
		Handler(handler).
		Get("/index.html"). // request
		Expect(t).          // expectations
		Body(`<h1>it works</h1>`).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(handler).
		Get("/").  // request
		Expect(t). // expectations
		Body(`<h1>it works</h1>`).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(handler).
		Get("/nested/folder/index.html"). // request
		Expect(t).                        // expectations
		Body(`<h1>it works</h1>`).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(handler).
		Get("/nested/folder/"). // request
		Expect(t).              // expectations
		Body(`<h1>it works</h1>`).
		Status(http.StatusOK).
		End()

	apitest.New().
		Handler(handler).
		Get("/api/hello-from-lua").
		Expect(t).
		Body(`hello from lua`).
		Status(http.StatusOK).
		End()
}

func tempCassette(ctx context.Context, t interface {
	Fatal(...interface{})
	Log(...interface{})
}, name string) (c *cassette.Control, cleanup func()) {
	dir, err := ioutil.TempDir("", "boombox-tests")
	if err != nil {
		t.Fatal(err)
	}
	abspath := filepath.Join(dir, name)
	ctl, err := cassette.LoadControlCassette(ctx, abspath, true)
	if err != nil {
		t.Fatal(err)
	}
	return ctl, func() {
		err := ctl.Close()
		if err != nil {
			t.Log("unable to close cassette", err)
		}
		err = os.RemoveAll(dir)
		if err != nil {
			t.Log("unable to cleanup temp dir", dir)
		}
	}
}
