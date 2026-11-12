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

	incomplete := write("incomplete.json", `{"pool":"std-blb","conditions":[]}`+"\n")
	_, ok, err := readPinDoc(incomplete)
	if ok || err == nil {
		t.Fatalf("incomplete document: ok=%v err=%v, want ok=false with an error", ok, err)
	} else if !strings.Contains(err.Error(), "needs a pool, conditions and at least one game") {
		t.Fatalf("incomplete document: unexpected error %v", err)
	}

	broken := write("broken.json", `{"pool":"std-blb","conditions":[]`)
	if _, ok, err := readPinDoc(broken); ok || err == nil {
		t.Fatalf("undecodable document: ok=%v err=%v, want ok=false with an error", ok, err)
	}
}
