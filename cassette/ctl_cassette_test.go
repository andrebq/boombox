package cassette

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCodebaseCassette(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true)
	if err != nil {
		t.Fatal(err)
	}
	indexLuaCode := `local ctx = require('http/ctx')
	ctx.writeBody('<h1>it works</h1>')`
	_, err = c.StoreAsset(ctx, "codebase/index.lua", "text/x-lua", indexLuaCode)
	if err != nil {
		t.Fatal(err)
	}
	err = c.ToggleCodebase(ctx, "codebase/index.lua", true)
	if err != nil {
		t.Fatal(err)
	}
	err = c.MapRoute(ctx, []string{"get", "post"}, "/index", "codebase/index.lua")
	if err != nil {
		t.Fatal(err)
	}
	routes, err := c.ListRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(routes, []Code{{Route: "/index", Code: indexLuaCode, Methods: []string{"GET", "POST"}}}) {
		t.Fatalf("Unexpected routes found: %#v", routes)
	}
	err = c.ToggleCodebase(ctx, "codebase/index.lua", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInvalidCodebaseCassette(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, assetPath := range []string{
		"not-codebase/index.lua",
		"codebase/index.not_lua_extension",
	} {
		_, err = c.StoreAsset(ctx, assetPath, "text/x-lua", `local ctx = require('http/ctx')
		ctx.writeBody('<h1>it works</h1>')
		`)
		if err != nil {
			t.Fatal(err)
		}
		err = c.ToggleCodebase(ctx, "codebase/index.lua", true)
		if err == nil {
			t.Fatalf("asset %v should never be considered a valid codebase", assetPath)
		}
	}
}

func TestControlCassette(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true)
	if err != nil {
		t.Fatal(err)
	}
	assetID, err := c.StoreAsset(ctx, "index.html", "text/html", `<h1>it works</h1>`)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.Buffer{}
	actualID, mt, err := c.CopyAsset(ctx, &out, "index.html")
	if err != nil {
		t.Fatal(err)
	} else if mt != "text/html" {
		t.Fatalf("Unexpected mime-type: %v", mt)
	} else if actualID != assetID {
		t.Fatalf("Copying asset should return ID %v got %v", assetID, actualID)
	} else if out.String() != "<h1>it works</h1>" {
		t.Fatalf("Invalid content from asset, got %v", out.String())
	}
	err = c.ToggleCodebase(ctx, "index.html", true)
	if !errors.Is(err, InvalidCodebase{Path: "index.html", MimeType: "text/html"}) {
		t.Fatalf("Codebase validation failed: %#v", err)
	}
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func tempTape(t interface {
	Fatal(...interface{})
	Log(...interface{})
}, name string) (abspath string, cleanup func()) {
	dir, err := ioutil.TempDir("", "boombox-tests")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name), func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Log("unable to cleanup temp dir", dir)
		}
	}
}
