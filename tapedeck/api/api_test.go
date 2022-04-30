package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrebq/boombox/cassette"
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

	handler, err := AsHandler(ctx, deck)
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
