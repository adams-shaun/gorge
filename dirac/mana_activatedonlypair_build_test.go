package dirac_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestActivatedOnlyRestrictedPair builds the kernel table the activated-only
// pair needs and checks the restricted views (CheckActivatedOnlyPair): the
// kernel's spend-restriction record already names an activation restriction
// (kop.Ability.ManaRestrictAb with its restrVarOf variant), and a second
// restricted ability of the same source is planned for the payment class its
// half admits.
func TestActivatedOnlyRestrictedPair(t *testing.T) {
	names := []string{
		"Mountain", "Island", "Sunken Citadel", "Steelswarm Operator",
		"Lightning Bolt", "Mind Stone", "Gilded Assault Cart",
	}
	cardsDir, err := cardset.CorpusDir()
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "activatedonlypair", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "activatedonlypair.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var citadel, operator, mountain, stone, bolt, cart kop.CardID
	found := 0
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Sunken Citadel":
			citadel = kop.CardID(c)
			found |= 1
		case "Steelswarm Operator":
			operator = kop.CardID(c)
			found |= 2
		case "Mountain":
			mountain = kop.CardID(c)
			found |= 4
		case "Mind Stone":
			stone = kop.CardID(c)
			found |= 8
		case "Lightning Bolt":
			bolt = kop.CardID(c)
			found |= 16
		case "Gilded Assault Cart":
			cart = kop.CardID(c)
			found |= 32
		}
	}
	if found != 63 {
		t.Fatalf("activatedonlypair table lacks a card (found mask %d)", found)
	}
	dirac.CheckActivatedOnlyPair(t, tbl, citadel, operator, mountain, stone, bolt, cart)
}
