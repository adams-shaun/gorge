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

// TestRestrictedLifeUnitFilterCastOffer builds the kernel table the
// restricted-planner-unit filter board of
// CheckRestrictedLifeUnitFilterCastOffer needs (Shadowblood Ridge the
// "{1},{T}: Add {B}{R}" filter, Untaidake, the Cloud Keeper the restricted
// "{T}, Pay 2 life: Add {C}{C}" planner unit, Skyshroud Vampire the
// {3}{B}{B} cast, Forest and Swamp the plain mana) and checks the filter
// route there: a filter beside a restricted planner unit is filter-only, so
// the Ridge funds the fifth mana of the cast its fee the census sources paid
// while it tapped (CR 601.2g-h).
func TestRestrictedLifeUnitFilterCastOffer(t *testing.T) {
	names := []string{"Forest", "Swamp", "Shadowblood Ridge", "Untaidake, the Cloud Keeper", "Skyshroud Vampire"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "restrlifefilt", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "restrlifefilt.ktab"))
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
	dirac.CheckRestrictedLifeUnitFilterCastOffer(t, tbl, id)
}
