package kgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// unrefuseFixture is a root with one evidence list holding card, a sidecar naming rows, and
// a LEGACY.txt of legacy; the card is refused (its rows taken out of the lists).
func unrefuseFixture(t *testing.T, card, sidecarRows, legacy string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(cardset.EvidenceDir+"/p.txt", "Aaa\n"+card+"\nZzz\n")
	write("dirac/support/evidence-keys/p.tsv", covHeader+sidecarRows)
	write(coverageLegacy, legacy)
	write("dirac/support/coverage-floor.txt", zeroFloor)
	repro, err := parseRefusedRepro("p:full:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refuseCard(root, card, "lockstep", "differs", repro); err != nil {
		t.Fatal(err)
	}
	return root
}

func legacyOf(t *testing.T, root string) map[string]bool {
	t.Helper()
	m, err := readLegacy(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestUnrefuseWritesLegacyKey: a card whose restored evidence has no sidecar row gets a
// LEGACY.txt entry (sorted), so TestEvidenceCoverageRatchet finds a record for it.
func TestUnrefuseWritesLegacyKey(t *testing.T) {
	const card = "Maze of Ith"
	rowA := "Aaa\taaaaaaaaaaaaaaaa\t3\t2/2\t\t1/1\t\n"
	root := unrefuseFixture(t, card, rowA, "Aaa\nZzz\n")
	if err := unrefuseCard(root, card); err != nil {
		t.Fatal(err)
	}
	if !legacyOf(t, root)[card] {
		t.Fatal("LEGACY.txt has no entry for the unrefused card")
	}
	b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(coverageLegacy)))
	if want := "Aaa\nMaze of Ith\nZzz\n"; string(b) != want {
		t.Errorf("LEGACY.txt = %q, want %q", b, want)
	}
	// The ratchet's own check: every evidenced card has a row or a LEGACY entry.
	fsys := os.DirFS(root)
	rows, _, err := readCoverageRows(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rows[card]; ok {
		t.Fatal("precondition: the card must have no sidecar row")
	}
	falls, err := coverageFalls(fsys, map[string]string{"Aaa": "aaaaaaaaaaaaaaaa"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range falls {
		if f == "no coverage record: "+card {
			t.Errorf("falls %q", falls)
		}
	}
}

// TestUnrefuseKeepsLegacyWhenRowExists: a sidecar row makes a LEGACY entry stale, so none is written.
func TestUnrefuseKeepsLegacyWhenRowExists(t *testing.T) {
	const card = "Maze of Ith"
	row := card + "\tmmmmmmmmmmmmmmmm\t3\t2/2\t\t1/1\t\n"
	root := unrefuseFixture(t, card, row, "Aaa\n")
	if err := unrefuseCard(root, card); err != nil {
		t.Fatal(err)
	}
	if legacyOf(t, root)[card] {
		t.Error("LEGACY.txt gained an entry for a card with a sidecar row")
	}
}
