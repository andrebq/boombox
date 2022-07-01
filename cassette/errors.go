package cassette

import "fmt"

type (
	InvalidTextContent struct {
		Path     string
		MimeType string
	}

	InvalidCodebase struct {
		Path     string
		MimeType string

		cause error
	}

	AssetNotFound struct {
		Path string
	}

	CannotQuery struct{}

	QueryError struct {
		Query  string
		Params []interface{}
		cause  error
	}

	WriteOverflow struct {
		Total int
		Max   int
		Next  int
	}

	ReadonlyCassette  struct{}
	DatasetNotAllowed struct{}

	InvalidTableName struct {
		Name string
	}

	InvalidColumnName struct {
		Name string
	}

	RestrictedTable struct {
		Name string
	}

	TableAlreadyExistsError struct {
		Name string
	}

	MissingExtendedPrivileges struct{}

	RouteNotFound struct {
		Route string
	}
)

func (t TableAlreadyExistsError) Error() string {
	return fmt.Sprintf("cassete: table %v already exists in this cassette and cannot be re-created", t.Name)
}

func (r RestrictedTable) Error() string {
	return fmt.Sprintf("casset: table [%v] cannot be used as dataset, that name is restricted", r.Name)
}

func (r RouteNotFound) Error() string {
	return fmt.Sprintf("cassette: route [%v] not found", r.Route)
}

func (i InvalidTextContent) Error() string {
	return fmt.Sprintf("assets with mimetype %v must be utf-8 encoded", i.MimeType)
}

func (i InvalidCodebase) Error() string {
	if i.cause != nil {
		return i.cause.Error()
	}
	return fmt.Sprintf("assets with mimetype %v cannot be used as code", i.MimeType)
}

func (i InvalidCodebase) Unwrap() error {
	return i.cause
}

func (a AssetNotFound) Error() string {
	return fmt.Sprintf("asset %v not found", a.Path)
}

func (c CannotQuery) Error() string { return "cassette is writable, therefore cannot be queried" }

func (q QueryError) Error() string {
	return q.cause.Error()
}

func (q QueryError) Unwrap() error {
	return q.cause
}

func (o WriteOverflow) Error() string {
	return fmt.Sprintf("output buffer overflow, max: %v, total: %v, next: %v", o.Max, o.Total, o.Next)
}

func (r ReadonlyCassette) Error() string {
	return "cassette is opened in read-only mode"
}

func (r InvalidTableName) Error() string {
	return fmt.Sprintf("table %v does not conform to the required identifier names (%v)", r.Name, reValidIdentifiers.String())
}

func (r InvalidColumnName) Error() string {
	return fmt.Sprintf("column %v does not conform to the required identifier names (%v)", r.Name, reValidIdentifiers.String())
}
func (d DatasetNotAllowed) Error() string {
	return "cassette is not configured as a data cassette"
}

func (m MissingExtendedPrivileges) Error() string {
	return "cassette is not configured for extended privileges, operation is not allowed"
}
