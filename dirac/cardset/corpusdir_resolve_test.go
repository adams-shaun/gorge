package cardset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpusDirResolvesCardReadsOutOfTheModule pins the split go test's input
// tracking depends on: card-script and table reads made through CorpusDir go
// through the resolved corpus path, outside the module, so the test cache does
// not hash them, while the cards.lock sentinel stays a stat through the
// in-module .cards path — the one tracked open a corpus swap changes.
func TestCorpusDirResolvesCardReadsOutOfTheModule(t *testing.T) {
	isolate(t)
	real := makeCorpus(t, t.TempDir())
	if err := os.Symlink(real, ".cards"); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// The sentinel path stays the in-module link, so its stat of cards.lock is
	// an open go test tracks; a corpus swap changes what it reads.
	found, err := corpusFound()
	if err != nil {
		t.Fatal(err)
	}
	if found != filepath.Join(wd, ".cards") {
		t.Fatalf("sentinel path = %s, want the in-module link %s", found, filepath.Join(wd, ".cards"))
	}

	// Card-script and table reads go through the resolved corpus, outside the
	// module, so go test does not hash them.
	d, err := CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	if d != real {
		t.Fatalf("CorpusDir = %s, want the resolved corpus %s", d, real)
	}
	if resolved, err := filepath.EvalSymlinks(d); err != nil || resolved != d {
		t.Fatalf("CorpusDir = %s is still a symlink path (%s, %v)", d, resolved, err)
	}
	b, err := os.ReadFile(filepath.Join(d, "cards.lock"))
	if err != nil || !strings.Contains(string(b), testDigest) {
		t.Fatalf("read through the resolved corpus: %q, %v", b, err)
	}

	// A corpus swap: retarget the link, and the tracked sentinel — the
	// unchanged in-module path — reads the new corpus's lock, so go test
	// invalidates the cached results.
	other := makeCorpus(t, t.TempDir())
	otherDigest := strings.Repeat("f", 64)
	writeFile(t, filepath.Join(other, "cards.lock"), []byte(`{"commit":"other","digest":"`+otherDigest+`"}`))
	if err := os.Remove(".cards"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, ".cards"); err != nil {
		t.Fatal(err)
	}
	found, err = corpusFound()
	if err != nil {
		t.Fatal(err)
	}
	if found != filepath.Join(wd, ".cards") {
		t.Fatalf("sentinel path after the swap = %s, want the unchanged link", found)
	}
	if b, err := os.ReadFile(filepath.Join(found, "cards.lock")); err != nil || !strings.Contains(string(b), otherDigest) {
		t.Fatalf("tracked sentinel lock = %q, %v; want the swapped corpus's digest", b, err)
	}
	if d, err := CorpusDir(); err != nil || d != other {
		t.Fatalf("CorpusDir after the swap = %s, %v; want %s", d, err, other)
	}
}
