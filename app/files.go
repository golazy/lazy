package app

import (
	"embed"
	"io/fs"
)

//go:embed all:views public
var files embed.FS

func Views() (fs.FS, error) {
	return fs.Sub(files, "views")
}

func Public() (fs.FS, error) {
	return fs.Sub(files, "public")
}
