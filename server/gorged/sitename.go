package gorged

import (
	"bytes"
	"html"
	"io/fs"
)

// defaultSiteName is the -site-name default and the text of the built
// index.html's title element (web/index.html).
const defaultSiteName = "gorge"

// withTitle wraps inner so the index.html it serves names the deployment:
// the first <title>gorge</title> element reads name, HTML-escaped. The
// default name (or none) returns inner itself, so the built page is served
// byte for byte; a nil inner (no web build embedded) stays nil. Every other
// file passes through untouched.
func withTitle(inner fs.FS, name string) fs.FS {
	if inner == nil || name == "" || name == defaultSiteName {
		return inner
	}
	old := []byte("<title>" + defaultSiteName + "</title>")
	repl := []byte("<title>" + html.EscapeString(name) + "</title>")
	return metaFS{inner: inner, inject: func(raw []byte) []byte { return bytes.Replace(raw, old, repl, 1) }}
}
