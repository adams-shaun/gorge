package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCostModesLockstep: an opponent's battlefield RaiseCost whose
// charge waits for the spell's target (Esior, Wardwing Familiar's "spells
// your opponents cast that target one or more commanders you control cost
// {3} more", ValidTarget$) makes every targeted spell's price depend on its
// announcement (CR 601.2c before 601.2f). gorge's cast planner declines that
// witness ("shape:target_dependent_cost", paymentPlanTargetDependentFor), so
// on the autopay surface its priority list has no plain cast of a targeted
// spell the floating pool does not pay. The kernel priced it anyway and
// offered Flusterstorm (Counter ... unless its controller pays {1}; Storm)
// with an untapped Island: the recorded game, p0 holding Flusterstorm as
// p1's Tragic Slip resolves, Esior on p1's battlefield.
//
// The repro is replayed in the MBC draft split that holds Flusterstorm
// (mbcSplitOf), so a re-split of the pools keeps the test.
func TestUnlessCostModesLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/target_dependent_cost_board_raise.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil {
		t.Fatal("the repro records no divergence point")
	}
	reg := synthReg(t)
	pool, set := mbcSplitOf(t, "Flusterstorm")
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatalf("%s: %v", pool, err)
	}
	cfg := Config{GorgeAutopaySurface: true}
	k, err := SetSide(set, tbl, cfg)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	// The precondition, read on gorge's board at the recorded divergence:
	// Esior on p1's battlefield, Flusterstorm in p0's hand, a spell on the
	// stack for it to target, and p0 holding priority.
	reached := false
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq == r.Divergence.At && c.G != nil {
			g := c.G.E.G
			reached = zoneHoldsNamed(g, state.ZBattlefield, 1, "Esior, Wardwing Familiar") &&
				zoneHoldsNamed(g, state.ZHand, 0, "Flusterstorm") && len(g.Stack) > 0 && c.D.Player == 0
		}
		return script.Pick(c)
	})
	res := Lockstep(m, r.Seed, k, pol, RunConfig{Config: cfg, MaxDecisions: len(r.Actions) + 200})
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if d := res.Div; d != nil {
		t.Fatalf("divergence at %d: %s %s %s\n gorge %v\n dirac %v", d.At, d.What, d.Field, d.Detail, d.RefOptions, d.KerOptions)
	}
	if !reached {
		t.Fatalf("decision %d is not p0's priority with Flusterstorm in hand, a spell on the stack and Esior on p1's battlefield", r.Divergence.At)
	}
}

// zoneHoldsNamed reports whether seat's zone z holds a card named n.
func zoneHoldsNamed(g *state.Game, z state.Zone, seat state.PlayerID, n string) bool {
	for _, id := range g.Zone(z, seat) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == n {
			return true
		}
	}
	return false
}
