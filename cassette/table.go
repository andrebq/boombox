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
	return &td, nil
}
