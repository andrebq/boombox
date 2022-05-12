package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/andrebq/boombox/cassette"
	"github.com/steinfletcher/apitest"
)

func TestQueryApi(t *testing.T) {
	ctx := context.Background()
	cassette, cleanup := tempQueryCassette(ctx, t, "test", func(ctx context.Context, c *cassette.Control) error {
		_, _, err := c.ImportCSVDataset(ctx, "names", bytes.NewBufferString(`"name","age"
"bob",30
"charlie", 31
`))
		return err
	})
	defer cleanup()
	handler, err := AsQueryHandler(ctx, cassette, nil)
	if err != nil {
		t.Fatal(err)
	}

	apitest.New().
		Handler(handler).
		Get("/.query").
		Query("sql", "select name, age from names order by age asc").
		Expect(t).
		Body(`{"columns":["name","age"]
,"rows": [["bob",30]
,["charlie",31]
]}`).
		Status(http.StatusOK).
		End()
}

func tempQueryCassette(ctx context.Context, t interface {
	Fatal(...interface{})
	Log(...interface{})
}, name string, loader func(ctx context.Context, c *cassette.Control) error) (c *cassette.Control, cleanup func()) {
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
