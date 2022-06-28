package cassette

import (
	"context"
	"reflect"
	"testing"
)

func TestReadTableInfo(t *testing.T) {
	tape, cleanup := tempTape(t, "test")
	defer cleanup()

	ctx := context.Background()
	c, err := LoadControlCassette(ctx, tape, true, true)
	if err != nil {
		t.Fatal(err)
	}

	td, err := loadTableDef(ctx, c.db, "bb_assets")
	if err != nil {
		t.Fatal(err)
	}

	expected := tableDef{
		name: "bb_assets",
		columns: []columnDef{
			{name: "asset_id", datatype: "INTEGER"},
			{name: "content", datatype: "BLOB"},
			{name: "mime_type", datatype: "TEXT"},
			{name: "path", datatype: "TEXT"},
			{name: "path_hash64", datatype: "INTEGER"},
		},
		pk: []string{"asset_id"},
		unique: []uniqueDef{
			{name: "uidx_asset_path", columns: []string{"path"}},
		},
	}

	if !reflect.DeepEqual(expected, *td) {
		t.Fatalf("Expecting: %v\nGot: %v", expected, *td)
	}
}
