package cassette

import "fmt"

type (
	InvalidTextContent struct {
		Path     string
		MimeType string
	}

	AssetNotFound struct {
		Path string
	}
)

func (i InvalidTextContent) Error() string {
	return fmt.Sprintf("assets with mimetype %v must be utf-8 encoded", i.MimeType)
}

func (a AssetNotFound) Error() string {
	return fmt.Sprintf("asset %v not found", a.Path)
}
