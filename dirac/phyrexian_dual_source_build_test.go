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

// TestPhyrexianDualSourcePricedHalf builds the dual-source board Hall of
// Tagsin's priced {C} beside its any-colour census gap needs, and checks the
// Phyrexian activation gate keeps the priced half (dirac/phyrexian_dual_source_test.go).
func TestPhyrexianDualSourcePricedHalf(t *testing.T) {
	names := []string{"Khenra Spellspear", "Hall of Tagsin", "Mountain"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "phydual", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "phydual.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	deck := []cardset.Entry{
		{Name: "Mountain", Count: 20},
		{Name: "Khenra Spellspear", Count: 1},
		{Name: "Hall of Tagsin", Count: 1},
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = kop.CardID(c)
	}
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("phydual table lacks %s", n)
		}
	}
	dirac.CheckPhyrexianDualSourcePricedHalf(t, tbl, id)
}
