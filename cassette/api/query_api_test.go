package api

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/testutil"
	"github.com/steinfletcher/apitest"
)

func TestQueryApi(t *testing.T) {
	ctx := context.Background()
	cassette, cleanup := testutil.AcquireCassette(ctx, t, "test", func(ctx context.Context, c *cassette.Control) error {
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
