package web

import "embed"

//go:embed index.html
var files embed.FS

func IndexHTML() []byte {
	b, err := files.ReadFile("index.html")
	if err != nil {
		return nil
	}
	return b
}
