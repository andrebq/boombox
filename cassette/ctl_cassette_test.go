package cassette

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentifiers(t *testing.T) {
	type testCase struct {
		ident string
		valid bool
	}
	for _, tc := range []testCase{
		{"wind", true},
		{"_abc1234", true},
		{"abc-123", false},
		{"1234", false},
		{"te st", false},
	} {
		valid := reValidIdentifiers.MatchString(tc.ident)
		if valid != tc.valid {
			t.Errorf("identifier reValidIdentifiers.MatchString(%v) should return %v but got %v", tc.ident, tc.valid, valid)
		}
	}
}

func TestImportCSV(t *testing.T) {
	csv := `"text","integer","real"
"text", 1234, 1234.5`
	tape, cleanup := tempTape(t, "test")
	t.Logf("Tape: %v", tape)
	_ = cleanup
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	expectedCreateStmt := "create table if not exists sample_data(text text,integer integer,real real)"
	createStmt, rows, err := c.ImportCSVDataset(ctx, "sample_data", bytes.NewBufferString(csv))
	if err != nil {
		t.Fatal(err)
	} else if rows != 1 {
		t.Fatalf("should have imported %v rows, got %v", 1, rows)
	} else if createStmt != expectedCreateStmt {
		t.Errorf("Create stmt should be (%v) got (%v)", expectedCreateStmt, createStmt)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = LoadControlCassette(ctx, tape, false, true)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON := `{"columns":["text","integer","real"]
,"rows": [["text", 1234, 1234.5]]}`
	var buf bytes.Buffer
	err = c.Query(ctx, &buf, -1, "select text, integer, real from dataset.sample_data")
	if err != nil {
		t.Fatal(err)
	} else {
		require.JSONEq(t, expectedJSON, buf.String(), "JSON objects should be equal")
	}
}

func TestQueryCassette(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.StoreAsset(ctx, "index.html", "text/html", "<h1>it works</h1>")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.StoreAsset(ctx, "info.html", "text/html", "<h1>it works</h1>")
	if err != nil {
		t.Fatal(err)
	}
	err = c.Query(ctx, io.Discard, 0, "select path, mime_type from assets")
	if err == nil {
		t.Fatal("A writable Cassette cannot be queried, but it was!")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = LoadControlCassette(ctx, tape, false, true)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON := `{"columns":["path","mime_type"]
,"rows": [["index.html","text/html"]
,["info.html","text/html"]
]}`
	var buf bytes.Buffer
	err = c.Query(ctx, &buf, -1, "select path, mime_type from assets")
	if err != nil {
		t.Fatal(err)
	} else {
		require.JSONEq(t, expectedJSON, buf.String(), "JSON objects should be equal")
	}
	err = c.Query(ctx, io.Discard, 11, "select path, mime_type from assets")
	if !errors.Is(err, WriteOverflow{Total: 11, Max: 11, Next: 21}) {
		t.Fatalf("Error should be WriteOverflow, got: %#v", err)
	}
	c.Close()
}

func TestCodebaseCassette(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true, true)
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
	c, err := LoadControlCassette(ctx, tape, true, true)
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
	c, err := LoadControlCassette(ctx, tape, true, true)
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
