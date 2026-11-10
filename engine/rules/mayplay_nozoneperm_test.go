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

// nozoneWorldWarHulk seats World War Hulk's chapter I over a hand of a red, a
// green and a blue creature and resolves it, so the no-zone free grant is live.
// gy is a red creature in the graveyard that has no permission of its own.
func nozoneWorldWarHulk(t *testing.T) (e *Engine, guide, wurm, looter, gy state.ObjID) {
	t.Helper()
	e, _, _, hand := effectGrantEngine(t, 1, []string{"World War Hulk"},
		[]string{"Goblin Guide", "Craw Wurm", "Looter il-Kor"})
	guide, wurm, looter = hand[0], hand[1], hand[2]
	g := e.G.AddObject(corpusAlternativeCard(t, "Goblin Guide"), 0)
	g.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), g.ID))
	for guard := 0; guard < 8 && len(e.G.Stack) > 0; guard++ {
		e.resolveTop()
	}
	e.pending = nil
	e.priorityRound()
	if got := nozoneGrantCount(e); got != 1 {
		t.Fatalf("active no-zone free grants after chapter I = %d, want 1", got)
	}
	return e, guide, wurm, looter, g.ID
}

func TestWorldWarHulkFreeCast(t *testing.T) {
	t.Parallel()
	e, guide, wurm, looter, gyID := nozoneWorldWarHulk(t)
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
	if got := nozoneCasts(e, 0, gyID); len(got) != 0 {
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
	// With mana it is offered at its printed cost: the green creature is
	// castable, so the zero free casts above is not an absent card.
	for c := range e.G.Players[0].Pool {
		e.G.Players[0].Pool[c] = 3
	}
	got := nozoneCasts(e, 0, wurm)
	if len(got) != 1 || got[0].AltCostIndex != 0 {
		t.Fatalf("second creature options with mana = %+v, want exactly one printed-cost cast", got)
	}
}

// A plain Effect-delivered permission over an exiled creature (the Atsushi
// shape: MayPlay$ True over exile) makes the cast legal; the live no-zone
// grant then makes THAT cast free, with an empty pool.
func TestWorldWarHulkFreeCastUnderPlainEffectGrant(t *testing.T) {
	t.Parallel()
	e, _, _, _, _ := nozoneWorldWarHulk(t)
	ex := e.G.AddObject(corpusAlternativeCard(t, "Goblin Guide"), 0)
	ex.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), ex.ID))
	e.pending = nil
	e.priorityRound()
	if got := nozoneCasts(e, 0, ex.ID); len(got) != 0 {
		t.Fatalf("precondition: exiled creature offered with no permission of its own: %+v", got)
	}
	e.AddContinuous(state.ContinuousEffect{Source: ex.ID, Controller: 0, Affects: "Card.IsRemembered",
		AffectedZone: "Exile", MayPlay: true, FromEffect: true, UntilEOT: true, Remembered: []state.ObjID{ex.ID}})
	e.pending = nil
	e.priorityRound()
	got := nozoneCasts(e, 0, ex.ID)
	if len(got) != 1 {
		t.Fatalf("exiled creature under a plain grant and the free grant, empty pool: %+v", got)
	}
	e.beginCast(0, got[0])
	if e.G.Obj(ex.ID).Zone != state.ZStack {
		t.Fatalf("exiled creature after the cast = %s, want stack (mana paid: pool %v)",
			e.G.Obj(ex.ID).Zone, e.G.Players[0].Pool)
	}
}

// An alt-only permission (pay life rather than the mana cost) withholds the
// printed offer; the no-zone free grant is then its own free alternative.
func TestWorldWarHulkFreeCastBesideAltOnlyPermission(t *testing.T) {
	t.Parallel()
	e, _, _, _, _ := nozoneWorldWarHulk(t)
	src := e.G.AddObject(card(t, "Name:Lifebinder\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ Card.YouOwn | AffectedZone$ Exile | MayPlay$ True | "+
		"MayPlayAltManaCost$ PayLife<ConvertedManaCost> | EffectZone$ Battlefield | Description$ x\nOracle:x\n"), 0)
	src.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), src.ID))
	ex := e.G.AddObject(corpusAlternativeCard(t, "Goblin Guide"), 0)
	ex.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), ex.ID))
	e.pending = nil
	e.priorityRound()
	got := nozoneCasts(e, 0, ex.ID)
	if len(got) != 2 || got[0].AltCostIndex != 1 || got[1].AltCostIndex != 2 {
		t.Fatalf("exiled creature options = %+v, want the life alternative (1) and the free alternative (2)", got)
	}
	life := e.G.Players[0].Life
	e.beginCast(0, got[1])
	if e.G.Obj(ex.ID).Zone != state.ZStack || e.G.Players[0].Life != life {
		t.Fatalf("free alternative: zone %s, life %d -> %d, want stack and no life paid",
			e.G.Obj(ex.ID).Zone, life, e.G.Players[0].Life)
	}
}
