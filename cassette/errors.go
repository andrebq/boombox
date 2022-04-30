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
