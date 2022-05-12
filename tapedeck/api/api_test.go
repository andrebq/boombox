package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/cassette/api"
	tplua "github.com/andrebq/boombox/internal/lua/bindings/tapedeck"
	"github.com/andrebq/boombox/internal/testutil"
	"github.com/andrebq/boombox/tapedeck"
	"github.com/steinfletcher/apitest"
)

func TestTapedeck(t *testing.T) {
	ctx := context.Background()
	index, indexDone := tempCassette(ctx, t, "index")
	defer indexDone()
	about, aboutDone := tempCassette(ctx, t, "about")
	defer aboutDone()
	deck := tapedeck.New()
	deck.Load("index", index)
	deck.Load("about", about)

	if _, err := index.StoreAsset(ctx, "index.html", "text/html", `<h1>it works</h1>`); err != nil {
		t.Fatal(err)
	}
	if _, err := about.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", `
	local res = require('ctx').res
	res:write_body('from lua')
	`); err != nil {
		t.Fatal(err)
	}
	if err := about.ToggleCodebase(ctx, "codebase/index.lua", true); err != nil {
		t.Fatal(err)
	}
	if err := about.MapRoute(ctx, []string{"GET"}, "/index", "codebase/index.lua"); err != nil {
		t.Fatal(err)
	}

	handler, err := AsHandler(ctx, deck, nil, api.AsHandler)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).
		Get("/index/").
		Expect(t).
		Status(http.StatusOK).
		Body(`<h1>it works</h1>`).
		End()
	apitest.Handler(handler).
		Get("/about/api/index").
		Expect(t).
		Status(http.StatusOK).
		Body(`from lua`).
		End()
}

func TestDynamicPageCasseteQuery(t *testing.T) {
	ctx := context.Background()
	deck, cleanup := testutil.AcquirePopulatedTapedeck(ctx, t, func(ctx context.Context, name string, c *cassette.Control) error {
		if name != "people" {
			return nil
		}

		if _, err := c.StoreAsset(ctx, "codebase/peeps.lua", "application/x-lua", `
	local deck = require('tapedeck')
	local json = require('json')
	local res = require('ctx').res
	local result = deck.load('people'):query('select name from dataset.people')
	res:write_body(json.to_json(result))
	`); err != nil {
			return err
		}
		if err := c.ToggleCodebase(ctx, "codebase/peeps.lua", true); err != nil {
			return err
		}
		if err := c.MapRoute(ctx, []string{"GET"}, "/list", "codebase/peeps.lua"); err != nil {
			t.Fatal(err)
		}
		return nil
	})
	defer cleanup()
	handler, err := AsHandler(ctx, deck, tplua.OpenModule(deck), api.AsHandler)
	if err != nil {
		t.Fatal(err)
	}
	apitest.Handler(handler).
		Get("/people/api/list").
		Expect(t).
		Status(http.StatusOK).
		Body(`{"columns":["name"],"rows":[["bob"],["charlie"],["ana"]]}`).
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
	ctl, err := cassette.LoadControlCassette(ctx, abspath, true, true)
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
