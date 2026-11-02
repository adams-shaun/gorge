package gorge

import (
	"regexp"
	"testing"
	"testing/fstest"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func mustHash(t *testing.T, fsys fstest.MapFS) string {
	t.Helper()
	h, err := hashSources(fsys)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestKeyIgnoresTestFiles(t *testing.T) {
	base := fstest.MapFS{"dirac/cmd/kgen/a.go": file("package main"), "engine/state/s.go": file("package state")}
	want := mustHash(t, base)

	withTest := fstest.MapFS{"dirac/cmd/kgen/a.go": file("package main"), "engine/state/s.go": file("package state"), "dirac/cmd/kgen/a_test.go": file("package main // edited")}
	if got := mustHash(t, withTest); got != want {
		t.Errorf("adding a _test.go file changed the hash: %s vs %s", got, want)
	}

	// A control: the same edit to a non-test file must re-key.
	withSource := fstest.MapFS{"dirac/cmd/kgen/a.go": file("package main // edited"), "engine/state/s.go": file("package state")}
	if got := mustHash(t, withSource); got == want {
		t.Error("editing a source file did not change the hash")
	}
	renamed := fstest.MapFS{"dirac/cmd/kgen/b.go": file("package main"), "engine/state/s.go": file("package state")}
	if got := mustHash(t, renamed); got == want {
		t.Error("renaming a source file did not change the hash")
	}
}

// The framing keeps a path and its bytes apart: moving a byte across the boundary
// between two files must change the hash.
func TestSourceHashFramesEachFile(t *testing.T) {
	a := mustHash(t, fstest.MapFS{"a": file("xy"), "b": file("z")})
	b := mustHash(t, fstest.MapFS{"a": file("x"), "b": file("yz")})
	if a == b {
		t.Error("two different trees hashed the same")
	}
}

func TestEmbeddedSources(t *testing.T) {
	h := TableSourceHash()
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h) {
		t.Fatalf("TableSourceHash = %q", h)
	}
	if h != TableSourceHash() {
		t.Error("TableSourceHash is not stable")
	}
	for _, p := range []string{"dirac/kgen/main.go", "dirac/kop/table.go", "dirac/cardset/ktab.go", "dirac/cardset/pools/fdn/cards.txt", "dirac/cardset/pools/fdn/ids.tsv", "dirac/cardset/standard.txt", "dirac/cardset/setlists/DSK.txt", "dirac/support/evidence/fdn.txt", "engine/state/game.go"} {
		if _, err := tableSources.ReadFile(p); err != nil {
			t.Errorf("%s is not embedded: %v", p, err)
		}
	}
}
