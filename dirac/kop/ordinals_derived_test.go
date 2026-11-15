package kop

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestOrdinalsHoldNoManifestRow: a constant a dirac/manifest entry owns has no
// row in testdata/ordinals.tsv, because TestKopOrdinalsUnchanged derives it
// from the entry. A row there would make every new entry's neighbours
// edit the same file again, and could drift from the entry's value.
func TestOrdinalsHoldNoManifestRow(t *testing.T) {
	tr, err := manifest.Load(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ordinalsFile)
	if err != nil {
		t.Fatal(err)
	}
	owner := map[string]*manifest.Entry{}
	for _, e := range tr.Entries {
		if e.Fam().Pkg == "kop" {
			owner[e.Name+"\t"+e.Family] = e
		}
	}
	for _, row := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		f := strings.Split(row, "\t")
		if len(f) != 3 {
			t.Fatalf("%s: bad row %q", ordinalsFile, row)
		}
		if e := owner[f[0]+"\t"+f[1]]; e != nil {
			t.Errorf("%s is derived from %s; delete its ordinals.tsv row", f[0], e.Path)
		}
	}
	if len(owner) == 0 {
		t.Fatal("the manifest has no kop entry: the check saw nothing")
	}
}
