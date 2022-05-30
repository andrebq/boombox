package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/testutil"
	"github.com/steinfletcher/apitest"
)

func TestPrivilegedApp(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	defer cleanup()
	_, err := tape.StoreAsset(ctx, "index.html", "text/html", "<h1>it works</h1>")
	if err != nil {
		t.Fatal(err)
	}
	_, err = AsPrivilegedHandler(ctx, tape, nil)
	if !errors.Is(err, cassette.MissingExtendedPrivileges{}) {
		t.Fatalf("Error should be %v got %v", cassette.MissingExtendedPrivileges{}, err)
	}
	err = tape.EnablePrivileges()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AsPrivilegedHandler(ctx, tape, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Get("/index.html").Expect(t).Body("<h1>it works</h1>").Status(http.StatusOK).End()
}

func TestFormParsing(t *testing.T) {
	ctx := context.Background()
	var err error
	cassette, cleanup := testutil.AcquireCassette(ctx, t, "test", func(ctx context.Context, c *cassette.Control) error {
		_, err = c.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local to_json = require('json').to_json
	local res = ctx.res
	local req = ctx.req
	local route = req.route_param("route") or "missing"
	local qs = req.param("qs") or "missing"
	local form = req.param("form") or "missing"
	res:write_body(route .. "/" .. qs .. "/" .. form)
	`)
		if err != nil {
			t.Fatal(err)
		}
		err = c.ToggleCodebase(ctx, "codebase/index.lua", true)
		if err != nil {
			t.Fatal(err)
		}
		err = c.MapRoute(ctx, []string{"POST"}, "/:route/hello-from-lua", "codebase/index.lua")
		if err != nil {
			t.Fatal(err)
		}
		return nil
	})
	defer cleanup()
	handler, err := AsHandler(ctx, cassette, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.New().
		Handler(handler).
		Post("/api/from-route/hello-from-lua"). // request
		Query("qs", "from-qs").
		FormData("form", "from-form").
		Expect(t). // expectations
		Body(`from-route/from-qs/from-form`).
		Status(http.StatusOK).
		End()
}

func TestRequestParsing(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireCassette(ctx, t, "test", func(ctx context.Context, c *cassette.Control) error {
		_, err := c.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local to_json = require('json').to_json
	local res = ctx.res
	local req = ctx.req
	local route = req.route_param("route") or "missing"
	local qs = req.param("qs") or "missing"
	local body = req.parse_body() or "missing"
	res:write_body(route .. "/" .. qs .. "/" .. to_json(body))
	`)
		if err != nil {
			t.Fatal(err)
		}
		err = c.ToggleCodebase(ctx, "codebase/index.lua", true)
		if err != nil {
			t.Fatal(err)
		}
		err = c.MapRoute(ctx, []string{"POST"}, "/:route/hello-from-lua", "codebase/index.lua")
		if err != nil {
			t.Fatal(err)
		}
		return nil
	})
	defer cleanup()
	var err error
	handler, err := AsHandler(ctx, tape, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.New().
		Handler(handler).
		Post("/api/from-route/hello-from-lua"). // request
		Query("qs", "from-qs").
		Body(`{"from": ["body", 123, 123.5]}`).
		Expect(t). // expectations
		Body(`from-route/from-qs/{"from":["body",123,123.5]}`).
		Status(http.StatusOK).
		End()
}

func TestApi(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireCassette(ctx, t, "test", func(ctx context.Context, c *cassette.Control) error {
		_, err := c.StoreAsset(ctx, "index.html", "text/html", `<h1>it works</h1>`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.StoreAsset(ctx, "nested/folder/index.html", "text/html", `<h1>it works</h1>`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local res = ctx.res
	res:write_body('hello from lua')
	`)
		if err != nil {
			t.Fatal(err)
		}
		err = c.ToggleCodebase(ctx, "codebase/index.lua", true)
		if err != nil {
			t.Fatal(err)
		}
		err = c.MapRoute(ctx, []string{"GET"}, "/hello-from-lua", "codebase/index.lua")
		if err != nil {
			t.Fatal(err)
		}
		return nil
	})
	defer cleanup()
	handler, err := AsHandler(ctx, tape, nil)
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
		Body("<a href=\"/index.html\">See Other</a>.\n\n").
		Status(http.StatusSeeOther).
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
		Body("<a href=\"/nested/folder/index.html\">See Other</a>.\n\n").
		Status(http.StatusSeeOther).
		End()

	apitest.New().
		Handler(handler).
		Get("/api/hello-from-lua").
		Expect(t).
		Body(`hello from lua`).
		Status(http.StatusOK).
		End()
}
