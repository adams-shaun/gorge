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

// TestExileRiderTwoKalitas builds the table of CheckExileRiderTwoKalitas.
func TestExileRiderTwoKalitas(t *testing.T) {
	names := []string{"Swamp", "Kalitas, Traitor of Ghet", "Grizzly Bears"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "tworiders", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "tworiders.ktab"))
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
	dirac.CheckExileRiderTwoKalitas(t, tbl, id)
}
