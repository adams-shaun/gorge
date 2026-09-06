package main

import (
	"embed"
	"io/fs"
)

// webdist holds the Svelte build (make web). It is gitignored except for
// .keep, so a clean clone builds with no Node; webFS sends nil until a
// real build is present and httpapi then serves a 503 for the client.
//
//go:embed all:webdist
var webdist embed.FS

func webFS() fs.FS {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		return nil
	}
	if f, err := sub.Open("index.html"); err != nil {
		return nil
	} else {
		f.Close()
	}
	return sub
}
