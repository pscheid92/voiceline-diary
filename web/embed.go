package web

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var embedded embed.FS

var FS = mustSub(embedded, "frontend/dist")

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
