package importer

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrebq/boombox/cassette"
)

func TestImportDirectory(t *testing.T) {
	ctx := context.Background()
	c, done := tempCassette(ctx, t, "test")
	defer done()
	err := Directory(ctx, c, filepath.Join("testdata", "fixtures", "test-cassette"), true)
	if err != nil {
		t.Fatal(err)
	}
	actualAssets, err := c.ListAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedAssets := []string{
		"codebase/add_numbers.lua",
		"codebase/index.lua",
		"data.json",
		"dataset/dataset.lua",
		"index.html",
		"script.js",
		"style.css",
	}
	if !reflect.DeepEqual(actualAssets, expectedAssets) {
		t.Fatalf("Expecting assets: %v got %v", expectedAssets, actualAssets)
	}
}

func TestRouting(t *testing.T) {
	ctx := context.Background()
	c, done := tempCassette(ctx, t, "test")
	defer done()
	basedir := filepath.Join("testdata", "fixtures", "test-cassette")
	err := Directory(ctx, c, basedir, true)
	if err != nil {
		t.Fatal(err)
	}
	actualRoutes, err := c.ListRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	expectedRoutes := []cassette.Code{
		loadRouteCode(t, "/index", "GET", "codebase/index.lua", basedir),
		loadRouteCode(t, "/add-numbers", "GET|POST", "codebase/add_numbers.lua", basedir),
	}

	if !reflect.DeepEqual(actualRoutes, expectedRoutes) {
		t.Fatalf("Should get %v routes got %v", expectedRoutes, actualRoutes)
	}
}

func TestMimetypeHeuristic(t *testing.T) {
	type testCase struct {
		ext string
		mt  string
	}
	for _, tc := range []testCase{
		{".html", "text/html"},
		{".js", "application/javascript"},
		{".json", "application/json"},
		{".css", "text/css"},
		{".lua", "text/x-lua"},
		{"", "application/octet-stream"},
		{".exe", "application/octet-stream"},
	} {
		actual := MimetypeFromExtension(tc.ext)
		if actual != tc.mt {
			t.Fatalf("Extension %v should give mime-type %v got %v", tc.ext, tc.mt, actual)
		}
	}
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

func loadRouteCode(t interface {
	Fatal(...interface{})
}, route string, methods string, codebase string, basedir string) cassette.Code {
	srcfile := filepath.Join(basedir, filepath.FromSlash(codebase))
	data, err := ioutil.ReadFile(srcfile)
	if err != nil {
		t.Fatal(err)
	}
	return cassette.Code{
		Route:   route,
		Methods: strings.Split(strings.ToUpper(methods), "|"),
		Code:    string(data),
	}
}
