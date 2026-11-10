package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// World War Hulk's chapter I leaves a free-cast grant that gives no zone
// permission (MayPlayDontGrantZonePermissions$ True): it prices a cast that is
// otherwise legal and never opens one, and the first red or green creature
// spell cast ends it.

func nozoneFreeCasts(e *Engine, p state.PlayerID, id state.ObjID) int {
	n := 0
	for _, o := range e.legalActions(p) {
		if o.Kind == "cast" && o.Obj == id && o.AltCostIndex > 0 {
			n++
		}
	}
	return n
}

func nozoneCasts(e *Engine, p state.PlayerID, id state.ObjID) []decision.Option {
	var out []decision.Option
	for _, o := range e.legalActions(p) {
		if o.Kind == "cast" && o.Obj == id {
			out = append(out, o)
		}
	}
	return out
}

func nozoneGrantCount(e *Engine) int {
	n := 0
	for _, ce := range e.active() {
		if ce.MayPlay && ce.MayPlayFree && ce.MayPlayNoZone {
			n++
		}
	}
	return n
}

func TestWorldWarHulkFreeCast(t *testing.T) {
	t.Parallel()
	e, _, _, hand := effectGrantEngine(t, 1, []string{"World War Hulk"},
		[]string{"Goblin Guide", "Craw Wurm", "Looter il-Kor"})
	guide, wurm, looter := hand[0], hand[1], hand[2]
	// The graveyard card: a red creature that sits there before the grant.
	gy := e.G.AddObject(corpusAlternativeCard(t, "Goblin Guide"), 0)
	gy.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), gy.ID))
	for guard := 0; guard < 8 && len(e.G.Stack) > 0; guard++ {
		e.resolveTop()
	}
	e.pending = nil
	e.priorityRound()
	if got := nozoneGrantCount(e); got != 1 {
		t.Fatalf("active no-zone free grants after chapter I = %d, want 1", got)
	}
	// Precondition: the mana-less seat had no free cast before; now the red
	// and the green creature in hand are offered exactly one free cast each.
	if n := nozoneFreeCasts(e, 0, guide); n != 1 {
		t.Fatalf("free casts of the red creature in hand = %d, want 1: %+v", n, e.legalActions(0))
	}
	if n := nozoneFreeCasts(e, 0, wurm); n != 1 {
		t.Fatalf("free casts of the green creature in hand = %d, want 1", n)
	}
	// A blue creature is outside the grant.
	if n := nozoneFreeCasts(e, 0, looter); n != 0 {
		t.Fatalf("free casts of the blue creature = %d, want 0", n)
	}
	// The graveyard creature has no permission of its own: the grant opens
	// no zone, so nothing is offered for it.
	if got := nozoneCasts(e, 0, gy.ID); len(got) != 0 {
		t.Fatalf("graveyard creature offered under a grant that gives no zone permission: %+v", got)
	}
	// Casting the red creature free ends the grant: the green one now costs.
	opt, _ := func() (decision.Option, bool) {
		for _, o := range nozoneCasts(e, 0, guide) {
			if o.AltCostIndex > 0 {
				return o, true
			}
		}
		return decision.Option{}, false
	}()
	e.beginCast(0, opt)
	if got := e.G.Obj(guide).Zone; got != state.ZStack {
		t.Fatalf("red creature after the free cast = %s, want stack", got)
	}
	e.resolveTop()
	if got := nozoneGrantCount(e); got != 0 {
		t.Fatalf("no-zone free grants after the cast = %d, want 0 (ForgetOnCast$)", got)
	}
	e.pending = nil
	e.priorityRound()
	if n := nozoneFreeCasts(e, 0, wurm); n != 0 {
		t.Fatalf("second creature still offered free: %+v", e.legalActions(0))
	}
}
