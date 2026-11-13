package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
)

// A shock land entering under Warp Speed's "lands you control enter untapped"
// is a CR 616.1 competition over the tapped bit: its body is a tap behind the
// pay-life gate. gorge poses the order, then the election whichever body goes
// first; declining taps the land only when the shock body is applied last. The
// kernel poses the same two asks, counts the land play once, and a declined
// election taps nothing when the untap came last.
func TestShockLandUnderEntersUntappedOrder(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "shock_land_order_beside_enters_untapped.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Synth(t, "std-trk")
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
	orders := 0
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = true
	rc.MaxDecisions = len(r.Actions) + 200
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		if d := ref.E.Pending(); d != nil && d.Kind == decision.KReplacement {
			if o := ref.E.G.Obj(d.Source); o != nil && o.Face() != nil && o.Face().IsLand() {
				orders++
			}
		}
		return nil
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if orders == 0 {
		t.Fatal("no land's entry-order ask was posed: the recorded game no longer reaches the competition")
	}
}

// Spock, Voice of Reason draws a card for each nontoken permanent its
// controller has that entered this turn. gorge read the nonToken property as
// unknown and drew none, the kernel drew for each.
func TestNonTokenEnteredThisTurnCount(t *testing.T) {
	t.Parallel()
	pinnedGamesRun(t, "std-trc", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, activated("Spock, Voice of Reason"))
}
