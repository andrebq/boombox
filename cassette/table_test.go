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

	expected := TableDef{
		Name: "bb_assets",
		Columns: []ColumnDef{
			{Name: "asset_id", Datatype: "INTEGER"},
			{Name: "content", Datatype: "BLOB"},
			{Name: "mime_type", Datatype: "TEXT"},
			{Name: "path", Datatype: "TEXT"},
			{Name: "path_hash64", Datatype: "INTEGER"},
		},
		PrimaryKey: []string{"asset_id"},
		Unique: []UniqueDef{
			{Name: "uidx_asset_path", Columns: []string{"path"}},
		},
	}

	if !reflect.DeepEqual(expected, *td) {
		t.Fatalf("Expecting: %v\nGot: %v", expected, *td)
	}
}
