package cassette

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

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
