package gorged

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSiteNameSetsTheTitle(t *testing.T) {
	const page = "<!doctype html><html><head><meta charset=\"utf-8\"><title>gorge</title></head><body><h1>gorge</h1></body></html>"
	inner := fstest.MapFS{
		"index.html": {Data: []byte(page)},
		"a.js":       {Data: []byte("console.log('gorge')")},
	}
	if !strings.Contains(page, "<title>gorge</title>") {
		t.Fatal("fixture lacks the title element the rewrite targets")
	}
	web := withTitle(inner, "Pod & Co")
	got, err := fs.ReadFile(web, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(page, "<title>gorge</title>", "<title>Pod &amp; Co</title>", 1); string(got) != want {
		t.Fatalf("index.html =\n%s\nwant\n%s", got, want)
	}
	js, err := fs.ReadFile(web, "a.js")
	if err != nil {
		t.Fatal(err)
	}
	if string(js) != "console.log('gorge')" {
		t.Fatalf("a.js = %q, want it unchanged", js)
	}
	if m, ok := withTitle(inner, "gorge").(fstest.MapFS); !ok || m == nil {
		t.Fatal("withTitle(inner, \"gorge\") must return inner itself")
	}
	if m, ok := withTitle(inner, "").(fstest.MapFS); !ok || m == nil {
		t.Fatal("withTitle(inner, \"\") must return inner itself")
	}
	if withTitle(nil, "Pod & Co") != nil {
		t.Fatal("withTitle(nil, ...) must be nil")
	}
}
