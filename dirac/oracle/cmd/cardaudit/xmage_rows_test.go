package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestXMageManifestAddsRows: a name only the xmage manifest names still gets a
// row, so its x_claim is not invisible.
func TestXMageManifestAddsRows(t *testing.T) {
	file := filepath.Join(t.TempDir(), "xmage.txt")
	if err := os.WriteFile(file, []byte("Only X Mage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := twoCardCorpus(t)
	fsys := fstest.MapFS{
		"dirac/support/evidence/a.txt": {Data: []byte("Elsewhere\n")},
	}
	rows := audit(fsys, reg, fixtureLIR, opts{XMageManifest: file})
	for i := range rows {
		if rows[i].Card != "Only X Mage" {
			continue
		}
		if rows[i].XClaim != "supported" {
			t.Errorf("Only X Mage x_claim = %q, want supported", rows[i].XClaim)
		}
		if rows[i].DClaim != "not-lowered" {
			t.Errorf("Only X Mage d_claim = %q, want not-lowered", rows[i].DClaim)
		}
		return
	}
	t.Fatalf("xmage-only name has no row: %v", rows)
}
