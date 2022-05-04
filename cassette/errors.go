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
)

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
