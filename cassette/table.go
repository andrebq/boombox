package cassette

import (
	"context"
	"database/sql"
)

type (
	tableDef struct {
		name    string
		columns []columnDef
		pk      []string
		unique  []uniqueDef
	}

	uniqueDef struct {
		name    string
		columns []string
	}

	columnDef struct {
		name     string
		datatype string
	}
)

func loadTableDef(ctx context.Context, db *sql.DB, name string) (*tableDef, error) {
	td := tableDef{
		name: name,
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
		td.columns = append(td.columns, columnDef{name: row.name, datatype: row.datatype})
		if row.pk {
			td.pk = append(td.pk, row.name)
		}
	}
	if len(td.columns) == 0 {
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
		td.unique = append(td.unique, udef)
	}
	return &td, nil
}

func loadUniqueDef(ctx context.Context, db *sql.DB, name string) (uniqueDef, error) {
	rows, err := db.QueryContext(ctx, `select name from pragma_index_info(?) order by name`, name)
	if err != nil {
		return uniqueDef{}, err
	}
	defer rows.Close()
	ud := uniqueDef{
		name: name,
	}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return uniqueDef{}, err
		}
		ud.columns = append(ud.columns, name)
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
