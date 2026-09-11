package main

import (
	"bytes"
	"io"
	"io/fs"
	"time"
)

// cardMetaTag is the <meta name="gorge-cards"> tag gorged injects into the
// index.html it serves. Its PRESENCE — not its content — is what
// web/src/lib/oracle.ts reads as "an embedder chose a catalog": this tag
// names the SAME-ORIGIN catalog, gorged's own /cards/named (art.go). The
// content is "/" because it reads as a URL in the HTML, but normalize("/")
// in oracle.ts is "" and "" is exactly what its no-catalog guard tests, so
// the client keys on the tag existing at all. That is deliberate — see the
// detect() comment there.
const cardMetaTag = `<meta name="gorge-cards" content="/">`

// withCardMeta wraps an fs.FS so Open("index.html") serves a rewritten copy
// of the real build's index.html carrying cardMetaTag, while every other
// file passes through untouched and the on-disk build is never modified.
// gorged wraps the embedded web build with this before handing it to
// httpapi as Options.Web, so the SPA fallback path (host/httpapi/static.go)
// serves the tag with the page that needs it.
func withCardMeta(inner fs.FS) fs.FS {
	return metaFS{inner: inner}
}

type metaFS struct{ inner fs.FS }

func (m metaFS) Open(name string) (fs.File, error) {
	f, err := m.inner.Open(name)
	if err != nil || name != "index.html" {
		return f, err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return newTextFile(injectCardMeta(raw)), nil
}

// injectCardMeta inserts the tag right after <head> — where the client's
// meta-tag readers (oracle.ts, basepath.ts) find it, and where a served
// page's own <head> already begins. A build whose <head> is not literal
// still gets the tag: a <meta> at the very top of the document is parsed
// into the head by every browser's HTML parser. Idempotent, so a future
// build that ships the tag itself is served byte-for-byte as built.
func injectCardMeta(raw []byte) []byte {
	if bytes.Contains(raw, []byte(`name="gorge-cards"`)) {
		return raw
	}
	meta := []byte(cardMetaTag)
	if i := bytes.Index(raw, []byte("<head>")); i >= 0 {
		head := i + len("<head>")
		out := make([]byte, 0, len(raw)+len(meta)+1)
		out = append(out, raw[:head]...)
		out = append(out, '\n')
		out = append(out, meta...)
		return append(out, raw[head:]...)
	}
	out := make([]byte, 0, len(meta)+1+len(raw))
	out = append(out, meta...)
	out = append(out, '\n')
	return append(out, raw...)
}

// textFile is an in-memory index.html as an fs.File: bytes.Reader supplies
// Read and (for http.FileServer's serveContent) Seek; Stat reports a plain
// read-only file so the static handler treats it like any asset.
type textFile struct {
	*bytes.Reader
	size int64
}

var _ fs.File = (*textFile)(nil)

func newTextFile(b []byte) *textFile {
	return &textFile{Reader: bytes.NewReader(b), size: int64(len(b))}
}

func (f *textFile) Close() error               { return nil }
func (f *textFile) Stat() (fs.FileInfo, error) { return textFileInfo{size: f.size}, nil }

type textFileInfo struct{ size int64 }

func (fi textFileInfo) Name() string       { return "index.html" }
func (fi textFileInfo) Size() int64        { return fi.size }
func (fi textFileInfo) Mode() fs.FileMode  { return 0o444 }
func (fi textFileInfo) ModTime() time.Time { return time.Time{} }
func (fi textFileInfo) IsDir() bool        { return false }
func (fi textFileInfo) Sys() any           { return nil }
