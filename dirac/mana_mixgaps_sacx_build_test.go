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

// TestSacxFiltOwnCandidateGap builds the kernel table the mixed-gap board of
// CheckSacxFiltOwnNode needs (a filter gap Three Tree Mascot beside the
// sacrifice-another Bolg's Company, Goblin Tinkerer the play's own {T} source
// and a sacrifice candidate, Raging Goblin the found route's sacrifice) and
// checks the filter search's node accounting there: the sacrifice's
// own-source answer's dead-end node is charged (gorge leftOwnZone,
// CR 602.2b), so the play is not offered one node below the found route's
// last node.
func TestSacxFiltOwnCandidateGap(t *testing.T) {
	names := []string{"Swamp", "Bolg's Company", "Three Tree Mascot", "Goblin Tinkerer", "Raging Goblin"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "sacxfilt", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "sacxfilt.ktab"))
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
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("sacxfilt table lacks %s (refused by kgen?)", n)
		}
	}
	dirac.CheckSacxFiltOwnNode(t, tbl, id)
}
