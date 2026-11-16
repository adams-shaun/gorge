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

// TestAnyToGraveyardFiresOncePerCard builds the kernel table Vulturous
// Zombie's "put into an opponent's graveyard from anywhere" line needs and
// checks it (CheckAnyToGraveyard): the opponent's discard, mill and creature
// death each queue the trigger once, the controller's own cards never.
func TestAnyToGraveyardFiresOncePerCard(t *testing.T) {
	names := []string{"Swamp", "Vulturous Zombie", "Grizzly Bears"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "anytograve", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "anytograve.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := cardset.AllowUnverified(cardset.NewDataSet(pd)).Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = kop.CardID(c)
	}
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("anytograve table lacks %s (refused by kgen?)", n)
		}
	}
	dirac.CheckAnyToGraveyard(t, tbl, id)
}
