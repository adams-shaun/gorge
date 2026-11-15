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

// TestActivationHeldOutBesideNoWaterbend builds the kernel table the
// hold-out pair needs and checks the withheld offer
// (dirac.CheckActivationHeldOut): an activation applied and reversed with no
// progress twice in a row is held out of the window on every table, not only
// beside a waterbend card or an X-bound ability (CR 733.2).
func TestActivationHeldOutBesideNoWaterbend(t *testing.T) {
	names := []string{"Lithoform Engine", "Mountain", "Foundry Street Denizen"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "actholdout", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "actholdout.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	var le, den, mountain kop.CardID
	found := 0
	for c := range tbl.Cards {
		switch tbl.Cards[c].Name {
		case "Lithoform Engine":
			le, found = kop.CardID(c), found|1
		case "Foundry Street Denizen":
			den, found = kop.CardID(c), found|2
		case "Mountain":
			mountain, found = kop.CardID(c), found|4
		}
	}
	if found != 7 {
		t.Fatalf("actholdout table lacks a card (found mask %d)", found)
	}
	dirac.CheckActivationHeldOut(t, tbl, le, den, mountain)
}
