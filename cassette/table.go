package cassette

import (
	"context"
	"database/sql"
)

type (
	TableDef struct {
		Name       string      `json:"name"`
		Columns    []ColumnDef `json:"columns"`
		PrimaryKey []string    `json:"primaryKey"`
		Unique     []UniqueDef `json:"unique"`
	}

	UniqueDef struct {
		Name    string   `json:"name"`
		Columns []string `json:"columns"`
	}

	ColumnDef struct {
		Name     string `json:"name"`
		Datatype string `json:"datatype"`
	}
)

func (td *TableDef) canUpdate(name string) bool {
	for _, p := range td.PrimaryKey {
		if p == name {
			return false
		}
	}
	for _, uc := range td.Unique {
		for _, c := range uc.Columns {
			if c == name {
				return false
			}
		}
	}
	return true
}

func loadTableDef(ctx context.Context, db *sql.DB, name string) (*TableDef, error) {
	td := TableDef{
		Name: name,
	}

	type tableInfoRow struct {
		name     string
		datatype string
		pk       bool
	}
	rows, err := db.QueryContext(ctx, `select name, type, pk from pragma_table_info(?) order by name`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row tableInfoRow
		err = rows.Scan(&row.name, &row.datatype, &row.pk)
		if err != nil {
			return nil, err
		}
		td.Columns = append(td.Columns, ColumnDef{Name: row.name, Datatype: row.datatype})
		if row.pk {
			td.PrimaryKey = append(td.PrimaryKey, row.name)
		}
	}
	if len(td.Columns) == 0 {
		return nil, sql.ErrNoRows
	}
	uniqueIdx, err := listUniqueIndexes(ctx, db, name)
	if err != nil {
		return nil, err
	}
	for _, v := range uniqueIdx {
		udef, err := loadUniqueDef(ctx, db, v)
		if err != nil {
			return nil, err
		}
		td.Unique = append(td.Unique, udef)
	}
	return &td, nil
}

func loadUniqueDef(ctx context.Context, db *sql.DB, name string) (UniqueDef, error) {
	rows, err := db.QueryContext(ctx, `select name from pragma_index_info(?) order by name`, name)
	if err != nil {
		return UniqueDef{}, err
	}
	defer rows.Close()
	ud := UniqueDef{
		Name: name,
	}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return UniqueDef{}, err
		}
		ud.Columns = append(ud.Columns, name)
	}
	return ud, nil
}

func listUniqueIndexes(ctx context.Context, db *sql.DB, name string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `select name from pragma_index_list(?) where [unique] = 1 order by name`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ret []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		ret = append(ret, name)
	}
	return ret, nil
}
