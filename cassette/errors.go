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

		msg string
	}

	AssetNotFound struct {
		Path string
	}
)

func (i InvalidTextContent) Error() string {
	return fmt.Sprintf("assets with mimetype %v must be utf-8 encoded", i.MimeType)
}

func (i InvalidCodebase) Error() string {
	if i.msg != "" {
		return i.msg
	}
	return fmt.Sprintf("assets with mimetype %v cannot be used as code", i.MimeType)
}

func (a AssetNotFound) Error() string {
	return fmt.Sprintf("asset %v not found", a.Path)
}
