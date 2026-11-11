package dirac_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiamondMixRefusedByName: a table holding Lion's Eye Diamond beside a
// mana source of another census-gap kind is refused at its start as
// diamondmix, naming both cards (gorge's one prefix search activates both;
// the kernel models the Diamond alone). The Diamond beside a Mountain and a
// Jayemdae Tome (the Diamond-alone lockstep shape) is not refused.
func TestDiamondMixRefusedByName(t *testing.T) {
	const led = "Lion's Eye Diamond"
	for _, partner := range []string{
		"Bloom Tender",         // each-colour
		"Heap Gate",            // filter
		"Hermitic Herbalist",   // restricted any-colour (Cavern of Souls is not lowered by kgen)
		"Saruli Caretaker",     // tap-another
		"Wall of Roots",        // counter-cost
		"Pyromancer's Goggles", // deferred
		"",                     // control: Jayemdae Tome alone
	} {
		name := partner
		if name == "" {
			name = "alone"
		}
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			names := []string{led, "Mountain", "Jayemdae Tome"}
			if partner != "" {
				names = append(names, partner)
			}
			tbl := diamondMixTable(t, names)
			id := map[string]kop.CardID{}
			for c := range tbl.Cards {
				id[tbl.Cards[c].Name] = kop.CardID(c)
			}
			for _, n := range names {
				if _, ok := id[n]; !ok {
					t.Fatalf("table lacks %s (refused by kgen?)", n)
				}
			}
			if partner == "" {
				dirac.CheckDiamondMix(t, tbl, id[led])
				return
			}
			dirac.CheckDiamondMix(t, tbl, id[led], id[partner])
		})
	}
}

// diamondMixTable lowers names with kgen into one pool and returns its whole
// table (Pool: no pair guard runs).
func diamondMixTable(t *testing.T, names []string) *kop.Table {
	t.Helper()
	cardsDir, err := cardset.CorpusDir()
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "dhmix", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "dhmix.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := cardset.AllowUnverified(cardset.NewDataSet(pd)).Pool()
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}
