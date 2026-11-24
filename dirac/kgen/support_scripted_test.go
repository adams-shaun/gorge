package kgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A scripted evidence list (design section 5, T8) supports its cards like any
// other list: dirac/support/evidence/sc-<pool>.txt is read by the same glob and
// rule as fdn.txt. Its sidecar's last two columns, the scenario sha and the
// organic plays, are trailing data: the coverage reader still reads the first
// seven columns the organic sidecars share.
func TestSupportScriptedEvidence(t *testing.T) {
	const name = "Llanowar Elves"
	const header = "card\tlir16\tplays\tkeys\tunused\tbranches\tmissing_branches\tscenario\torganic_plays\n"
	const row = "Llanowar Elves\t\t0\t2/2\t\t0/0\t\tscenario-sha\t5\n"
	root := t.TempDir()
	for file, body := range map[string]string{
		"dirac/support/evidence/sc-x.txt":      name + "\n",
		"dirac/support/evidence-keys/sc-x.tsv": header + row,
	} {
		p := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := readSupport(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if !s.evidencedFor(name) {
		t.Fatalf("%s: an sc- list does not evidence it", name)
	}
	rows, order, err := readCoverageRows(os.DirFS(root))
	if err != nil {
		t.Fatalf("an sc- sidecar with its two extra columns does not parse: %v", err)
	}
	r, ok := rows[name]
	if !ok || r.lir16 != "" || r.keysUsed != 2 || r.keysTotal != 2 || r.brHit != 0 || r.brTotal != 0 {
		t.Errorf("%s: parsed %+v, ok %v", name, r, ok)
	}
	if len(order) != 1 || order[0] != name {
		t.Errorf("rows %v, want [%s]", order, name)
	}
	list := filepath.Join(root, "cards.txt")
	if err := os.WriteFile(list, []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, _, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	if st, why := statusOf(all[0], s); st != cardset.StatusSupported {
		t.Errorf("an sc- list alone: %v, %q", st, why)
	}
}
