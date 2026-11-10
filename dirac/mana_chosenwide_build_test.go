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

// TestChosenWidePairCensusFirst builds the table the chosen-colour pair needs
// and checks the witness: Sunken Citadel's {C}{C} is not a unit while the
// plain census pays, and Steelswarm Operator's fixed {U}{U} still is.
func TestChosenWidePairCensusFirst(t *testing.T) {
	names := []string{
		"Mountain", "Island", "Sunken Citadel", "Steelswarm Operator",
		"Lightning Bolt", "Mind Stone", "Gilded Assault Cart", "Kher Keep",
	}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "chosenwide", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "chosenwide.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var citadel, opCard, mountain, stone, keep kop.CardID
	found := 0
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Sunken Citadel":
			citadel = kop.CardID(c)
			found |= 1
		case "Steelswarm Operator":
			opCard = kop.CardID(c)
			found |= 2
		case "Mountain":
			mountain = kop.CardID(c)
			found |= 4
		case "Mind Stone":
			stone = kop.CardID(c)
			found |= 8
		case "Kher Keep":
			keep = kop.CardID(c)
			found |= 16
		}
	}
	if found != 31 {
		t.Fatalf("chosenwide table lacks a card (found mask %d)", found)
	}
	dirac.CheckChosenWidePairCensusFirst(t, tbl, citadel, opCard, mountain, stone, keep)
}
