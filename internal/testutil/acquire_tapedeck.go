package testutil

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/tapedeck"
)

type (
	TestLog interface {
		Fatal(...interface{})
		Log(...interface{})
	}
)

func AcquireWritableCassette(ctx context.Context, t TestLog, name string) (*cassette.Control, func()) {
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

func AcquireCassette(ctx context.Context, t TestLog, name string, loader func(context.Context, *cassette.Control) error) (*cassette.Control, func()) {
	dir, err := ioutil.TempDir("", "boombox-tests")
	if err != nil {
		t.Fatal(err)
	}
	abspath := filepath.Join(dir, name)
	ctl, err := cassette.LoadControlCassette(ctx, abspath, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if loader != nil {
		err = loader(ctx, ctl)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctl.Close()
	// re-open as read-only (aka queryable)
	ctl, err = cassette.LoadControlCassette(ctx, abspath, false, true)
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

func AcquirePopulatedTapedeck(ctx context.Context, t TestLog, loader func(context.Context, string, *cassette.Control) error) (*tapedeck.D, func()) {
	people, cleanupPeople := AcquireCassette(ctx, t, "people", func(ctx context.Context, c *cassette.Control) error {
		_, _, err := c.ImportCSVDataset(ctx, "people", bytes.NewBufferString(`"name","age"
"bob",22
"charlie",44
"ana",66`))
		if err != nil {
			return err
		}
		if loader != nil {
			err = loader(ctx, "people", c)
		}
		return err
	})
	index, cleanupIndex := AcquireCassette(ctx, t, "index", func(ctx context.Context, c *cassette.Control) error {
		_, err := c.StoreAsset(ctx, "index.html", "text/html", `<h1>it works</h1>`)
		if err != nil {
			return err
		}
		if loader != nil {
			err = loader(ctx, "index", c)
		}
		return err
	})
	d := tapedeck.New()
	d.Load("index", index)
	d.Load("people", people)
	return d, func() {
		err := d.Close()
		if err != nil {
			t.Log("Unable to close tapedeck", err)
		}
		cleanupIndex()
		cleanupPeople()
	}
}
