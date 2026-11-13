package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadPinDocRequiresACompleteDocument pins readPinDoc's contract at the
// boundary the manifest gates on: a file that starts with "{" claims to be a
// pin document, so an incomplete or undecodable one is an error, not a silent
// skip that takes the pin out of TestPinManifest. A bare array is still a pin
// some Go test drives and stays a skip, not an error.
func TestReadPinDocRequiresACompleteDocument(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	array := write("array.json", "[]\n")
	if _, ok, err := readPinDoc(array); err != nil || ok {
		t.Fatalf("bare array: ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	complete := write("complete.json", `{"pool":"std-blb","conditions":[{"kind":"cardOut","card":"X"}],"games":[{}]}`)
	doc, ok, err := readPinDoc(complete)
	if err != nil || !ok {
		t.Fatalf("complete document: ok=%v err=%v, want ok=true err=nil", ok, err)
	}
	if doc.Pool != "std-blb" || len(doc.Conditions) != 1 || len(doc.Games) != 1 {
		t.Fatalf("complete document decoded to pool=%q conditions=%d games=%d", doc.Pool, len(doc.Conditions), len(doc.Games))
	}

	for _, tc := range []struct{ name, body string }{
		{"no pool", `{"conditions":[{"kind":"cardOut","card":"X"}],"games":[{}]}`},
		{"no conditions", `{"pool":"std-blb","games":[{}]}`},
		{"empty conditions", `{"pool":"std-blb","conditions":[],"games":[{}]}`},
		{"no games", `{"pool":"std-blb","conditions":[{"kind":"cardOut","card":"X"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := write(strings.ReplaceAll(tc.name, " ", "_")+".json", tc.body+"\n")
			_, ok, err := readPinDoc(path)
			if ok || err == nil {
				t.Fatalf("incomplete document: ok=%v err=%v, want ok=false with an error", ok, err)
			} else if !strings.Contains(err.Error(), "needs a pool, conditions and at least one game") {
				t.Fatalf("incomplete document: unexpected error %v", err)
			}
		})
	}

	broken := write("broken.json", `{"pool":"std-blb","conditions":[]`)
	if _, ok, err := readPinDoc(broken); ok || err == nil {
		t.Fatalf("undecodable document: ok=%v err=%v, want ok=false with an error", ok, err)
	}
}
