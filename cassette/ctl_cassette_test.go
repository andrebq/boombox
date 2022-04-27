package cassette

import (
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
	c, err := LoadControlCassette(ctx, tape)
	if err != nil {
		t.Fatal(err)
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
