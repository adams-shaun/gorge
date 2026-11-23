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

// TestLifegapFilterMixedCastOffer builds the kernel table the life-gap mix
// board of CheckLifegapFilterMixedCastOffer needs (Shadowblood Ridge the
// "{1},{T}: Add {B}{R}" filter, Treasonous Ogre the "Pay 3 life: Add {R}"
// card the check turns into a life-only census gap with a {T}: Add {R}
// beside it, Skyshroud Vampire the {3}{B}{B} cast, Forest and Swamp the plain
// mana) and checks the mix route there: a filter beside a life gap is a mix
// board (mana_mixgaps.go), so the Ridge funds the fifth mana of the cast its
// fee the floated Forest paid while it tapped (CR 601.2g-h).
func TestLifegapFilterMixedCastOffer(t *testing.T) {
	names := []string{"Forest", "Swamp", "Shadowblood Ridge", "Treasonous Ogre", "Skyshroud Vampire"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "lifegapfiltmix", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "lifegapfiltmix.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = kop.CardID(c)
	}
	dirac.CheckLifegapFilterMixedCastOffer(t, tbl, id)
}
