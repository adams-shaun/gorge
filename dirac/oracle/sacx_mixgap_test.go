package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestSacxGapBesideFilterMutation hides the sacrifice-another source in the
// kernel only (dirac.DebugNoSacXMana): Giant's Boulder's destroy is then
// gorge's alone, and the recorded game diverges at its priority decision.
func TestSacxGapBesideFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoSacXMana = true
	defer func() { dirac.DebugNoSacXMana = false }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "sacx_gap_beside_filter_destroy_offer.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := globSet(t, r.Pool, r)
	res := sacxReplay(t, reg, set, r)
	if res.Div == nil {
		t.Fatal("no seed diverged without the sacrifice-another source: the game never needs Bolg's Company's mana")
	}
}

// sacxReplay is replayReproValue without its divergence fatal, for the
// mutation test above.
func sacxReplay(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro) Result {
	t.Helper()
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
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}
