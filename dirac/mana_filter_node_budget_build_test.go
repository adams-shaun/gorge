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

// TestFilterSearchNodeBudgetParity pins the plain filter search's node count
// on the root free-{C} branch of the tail-34 pair 25 offer (seed 255781) at
// or below gorge's: Vault of the Archangel's {2}{W}{B} activation beside
// Unknown Shores, two Swamps, two Mountains and Manabarbs. The four basics are
// Taps-deferred census gaps beside the paid filter, and gorge's activation
// loop skips a land that is a twin of one it already walked (scriptTwin); the
// kernel's other-gap branch returned before its twin cut, walked both
// Mountains and both Swamps in every order and spent 151-160 nodes on a branch
// gorge settles in 70, which left the offer on the potentialScriptNodes
// boundary. The cards are lowered by kgen from the corpus (a dirac test reads
// no raw pool); the branch is walked directly, freeBranchHopeless's prune
// bypassed (dirac.FilterBranchNodes).
func TestFilterSearchNodeBudgetParity(t *testing.T) {
	tbl := filtNodeBudgetTable(t, []string{"Unknown Shores", "Vault of the Archangel", "Swamp", "Mountain", "Manabarbs"})
	got := dirac.FilterBranchNodes(t, tbl)
	if got > dirac.FilterBranchNodesGorge {
		t.Fatalf("the root free-{C} branch cost %d nodes, gorge's cost %d: the search spends nodes gorge's twin cut skips", got, dirac.FilterBranchNodesGorge)
	}
}

// filtNodeBudgetTable lowers names with kgen into one pool and returns its
// whole table (Pool: no pair guard runs).
func filtNodeBudgetTable(t *testing.T, names []string) *kop.Table {
	t.Helper()
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
	if code := kgen.Main([]string{"-cards", cardsDir, "-card-list", list, "-pool", "fnb", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "fnb.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := cardset.AllowUnverified(cardset.NewDataSet(pd)).Pool()
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}
