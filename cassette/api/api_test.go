package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

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

func TestDynamicRouteUpdate(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	_, err := tape.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local res = ctx.res
	res:write_body('hello from lua')
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = tape.ToggleCodebase(ctx, "codebase/index.lua", true)
	if err != nil {
		t.Fatal(err)
	}
	err = tape.MapRoute(ctx, []string{"GET"}, "/api/index", "codebase/index.lua")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	err = tape.EnablePrivileges()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AsPrivilegedHandler(ctx, tape, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Get("/api/index").Expect(t).Body("hello from lua").Status(http.StatusOK).End()
	_, err = tape.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local ctx = require('ctx')
	local res = ctx.res
	res:write_body('hello from updated lua')
	`)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: very clumsy, but here we wait for the update
	// Since the server is recompiling the router every second, by waiting for 2 seconds
	// we defintely should pick up the new values
	time.Sleep(time.Second * 2)
	apitest.Handler(handler).Get("/api/index").Expect(t).Body("hello from updated lua").Status(http.StatusOK).End()
}

func TestUploadAsset(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	defer cleanup()
	_, err := tape.StoreAsset(ctx, "index.html", "text/html", "<h1>it works</h1>")
	if err != nil {
		t.Fatal(err)
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
	apitest.Handler(handler).Put("/.internals/write-asset/index.html").Body("<h1>it also works</h1>").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/index.html").Expect(t).Body("<h1>it also works</h1>").Status(http.StatusOK).End()
	apitest.Handler(handler).Put("/.internals/write-asset/index2.html").Body("<h1>it also works</h1>").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Get("/index2.html").Expect(t).Status(http.StatusOK).Body("<h1>it also works</h1>").End()
}

func TestUploadCode(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	defer cleanup()
	err := tape.EnablePrivileges()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AsPrivilegedHandler(ctx, tape, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Put("/.internals/write-asset/codebase/index.lua").Body(`
	local ctx = require('ctx')
	local res = ctx.res
	res:write_body('hello from lua')
	`).Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Put("/.internals/enable-code/codebase/index.lua").Query("enabled", "true").Expect(t).Status(http.StatusOK).End()
	apitest.Handler(handler).Post("/.internals/set-route").Body(`{"route": "/api/index", "asset":"codebase/index.lua", "methods":["GET"]}`).Expect(t).Status(http.StatusOK).End()
	// do the silly sleep to allow an internal refresh
	time.Sleep(time.Second * 2)
	apitest.Handler(handler).Get("/api/index").Expect(t).Status(http.StatusOK).Body("hello from lua").End()
}

func TestCreateTable(t *testing.T) {
	ctx := context.Background()
	tape, cleanup := testutil.AcquireWritableCassette(ctx, t, "test")
	defer cleanup()
	err := tape.EnablePrivileges()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AsPrivilegedHandler(ctx, tape, nil)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).Post("/.internals/ddl/create/table/objects").Body(
		`{"columns": [{"name": "key1", "datatype": "text"}, {"name": "key2", "datatype": "text"}], "primaryKey": ["key1"]}`).
		Expect(t).Status(http.StatusOK).End()
	// do the silly sleep to allow an internal refresh
	time.Sleep(time.Second * 2)
	ddl, err := tape.TableDDL(ctx, "objects")
	if err != nil {
		t.Fatal(err)
	}
	expectedDDL := `create table if not exists objects(key1 TEXT,key2 TEXT, primary key(key1));
create unique index uidx_sqlite_autoindex_objects_1 on objects(key1);
`
	if ddl != expectedDDL {
		t.Errorf("Expecting ddl [%q]\ngot [%q]", expectedDDL, ddl)
	}
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
		err = c.MapRoute(ctx, []string{"POST"}, "/api/:route/hello-from-lua", "codebase/index.lua")
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
		err = c.MapRoute(ctx, []string{"POST"}, "/api/:route/hello-from-lua", "codebase/index.lua")
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
		err = c.MapRoute(ctx, []string{"GET"}, "/api/hello-from-lua", "codebase/index.lua")
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
		Body("<h1>it works</h1>").
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
		Body("<h1>it works</h1>").
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
