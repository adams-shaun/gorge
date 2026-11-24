package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// copy_exc_changeling_order.repro.json is a recorded game of the two copy
// exceptions CR 707.9b applies in order. Dominion Infiltration makes a token
// copy of an opponent's Grizzly Bears, "except it has changeling" (its
// CopyFace carries the exception, so a copy of the token inherits it);
// Diabolic Edicts clear the opponent's creatures, Lignify makes the token a
// non-Frog Treefolk (so Croaking Counterpart may target it), and Croaking
// Counterpart then copies it "except it's a Frog". CR 707.9d: a copy effect
// that provides a specific set of subtypes does not copy the CDA that defines
// that characteristic, so the Frog exception ends the inherited changeling.
// The kernel's ordered reader (chars_gen.go's seed sites) implements that.
// Gorge's basis fold once kept the copied Changeling keyword, so the recorded
// game ended at that divergence (Cards[81].Chars.AllTypes ref=true
// kernel=false; the file's "divergence" field is that historical record).
// gorge now drops the copied CDA (foldCopyExcept), so the replay is clean.
func TestCopyExcChangelingOrderDiverges(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "copy_exc_changeling_order.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "copyexc-changeling" || r.Start == nil {
		t.Fatalf("precondition: pool %q start %v, want the recording's own table and its scripted start", r.Pool, r.Start != nil)
	}
	if r.Divergence == nil {
		t.Fatal("precondition: the recording carries no divergence")
	}
	reg := synthReg(t)
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 8
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	if pol.Skipped != 0 {
		t.Fatalf("replay skipped %d of %d recorded actions: the game path drifted", pol.Skipped, len(r.Actions))
	}
	if res.Div != nil {
		t.Fatalf("replay diverged at %d/%s: %s", res.Div.At, res.Div.What, strings.Join(res.Div.Diff, "\n"))
	}
	if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("replay stopped on the kernel side (%q, unmodelled %v)", res.Stall, res.StallUnmodelled)
	}
	if len(res.Actions) < len(r.Actions) {
		t.Fatalf("replay served %d of %d recorded actions", len(res.Actions), len(r.Actions))
	}
}
