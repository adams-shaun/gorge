package main

import (
	"embed"
	"io/fs"
)

// webdist holds the Svelte build (make web). It is gitignored except for
// .keep, so a clean clone builds with no Node; webFS sends nil until a
// real build is present and httpapi then serves a 503 for the client.
//
// The served index.html carries an injected <meta name="gorge-cards">
// (cardsmeta.go): gorged acts as its own embedder-chosen card catalog, and
// the client's oracle.ts reads the tag to find it. The build on disk is
// never modified — the rewrite happens at open time.
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
	return withCardMeta(sub)
}
