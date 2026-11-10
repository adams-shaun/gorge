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

// TestEachColourFeeGap builds the kernel table Tarnation Vista's fee-paid
// each-colour gap needs and checks its search (CheckEachColourFee): the {1} fee
// is paid from the floating pool, never by a land, and a Bloom Tender's free
// gap is untouched.
func TestEachColourFeeGap(t *testing.T) {
	names := []string{"Mountain", "Tarnation Vista", "Grizzly Bears", "Savannah Lions", "Watchwolf", "Bloom Tender"}
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "eachfee", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "eachfee.ktab"))
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
			t.Fatalf("eachfee table lacks %s", n)
		}
	}
	dirac.CheckEachColourFee(t, tbl, id)
}
