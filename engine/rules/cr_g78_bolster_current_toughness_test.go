package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBolsterReadsCurrentToughness pins CR 701.39a to the CURRENT toughness:
// "least toughness" is the characteristic after every continuous effect
// (CR 208.3/613), not the printed face plus counters. The old read elected a
// printed 2/2 under Honor of the Pure (actually a 3/3) over a printed 3/3,
// and ignored a -2/-2 pump entirely.
func TestBolsterReadsCurrentToughness(t *testing.T) {
	t.Run("anthem tie is offered as a choice", func(t *testing.T) {
		e, _, lion, giant := bolsterFixture(t)
		// Precondition: the anthem really moves the printed 2/2 to the same
		// current toughness as the printed 3/3, so the two tie at 3.
		if got := e.Toughness(lion); got != 3 {
			t.Fatalf("precondition: Silvercoat Lion under Honor of the Pure toughness = %d, want 3", got)
		}
		if got := e.Toughness(giant); got != 3 {
			t.Fatalf("precondition: Hill Giant toughness = %d, want 3", got)
		}
		d := castDromokasGift(t, e)
		if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_pick" {
			t.Fatalf("a current-toughness tie must pose a counter_pick election, got %+v", d)
		}
		if len(d.Options) != 2 {
			t.Fatalf("election options = %d, want both 3-toughness creatures: %+v", len(d.Options), d.Options)
		}
		seen := map[state.ObjID]bool{}
		for _, o := range d.Options {
			seen[o.Obj] = true
		}
		if !seen[lion] || !seen[giant] {
			t.Fatalf("election must offer the anthem-pumped Lion (%d) and the Giant (%d): %+v",
				lion, giant, d.Options)
		}
		submitChoices(t, e, d.Options[1].Index)
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(giant).Counter("P1P1"); got != 4 {
			t.Errorf("chosen Giant P1P1 = %d, want 4", got)
		}
		if got := e.G.Obj(lion).Counter("P1P1"); got != 0 {
			t.Errorf("unchosen Lion P1P1 = %d, want 0", got)
		}
	})

	t.Run("pump makes the printed 3/3 the least", func(t *testing.T) {
		e, disfigure, lion, giant := bolsterFixture(t)
		addMana(t, e, 0, "B")
		castSpellAtTarget(t, e, disfigure, giant)
		// Precondition: the -2/-2 really changed the current toughness the
		// election must read (1 vs the anthem's 3), while the printed faces
		// still rank the other way round (2 vs 3).
		if got := e.Toughness(giant); got != 1 {
			t.Fatalf("precondition: pumped Hill Giant toughness = %d, want 1", got)
		}
		if got := e.Toughness(lion); got != 3 {
			t.Fatalf("precondition: Silvercoat Lion under Honor of the Pure toughness = %d, want 3", got)
		}
		castDromokasGiftResolved(t, e)
		if got := e.G.Obj(giant).Counter("P1P1"); got != 4 {
			t.Errorf("pumped Hill Giant (current toughness 1) P1P1 = %d, want 4", got)
		}
		if got := e.G.Obj(lion).Counter("P1P1"); got != 0 {
			t.Errorf("Silvercoat Lion (current toughness 3) P1P1 = %d, want 0", got)
		}
	})
}

// bolsterFixture builds a corpus game with Honor of the Pure (a white-only
// anthem), a 2/2 white Silvercoat Lion and a 3/3 red Hill Giant on seat 0's
// battlefield, plus Disfigure and Dromoka's Gift in hand. It returns the
// Disfigure id along with the two creature ids.
func bolsterFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e, _ := fdnCensusEngine(t, "Honor of the Pure", "Silvercoat Lion", "Hill Giant", "Disfigure", "Dromoka's Gift")
	moveByName(t, e, 0, "Honor of the Pure", state.ZBattlefield)
	lion := moveByName(t, e, 0, "Silvercoat Lion", state.ZBattlefield)
	giant := moveByName(t, e, 0, "Hill Giant", state.ZBattlefield)
	disfigure := moveByName(t, e, 0, "Disfigure", state.ZHand)
	// Dromoka's Gift is the pure Bolster 4 carrier; Disfigure is the
	// until-end-of-turn -2/-2 pump. Both must sit in hand for the casts.
	moveByName(t, e, 0, "Dromoka's Gift", state.ZHand)
	e.pending = nil
	e.priorityRound()
	return e, disfigure, lion, giant
}

// castSpellAtTarget casts id and answers its target decision with target,
// then drains the stack.
func castSpellAtTarget(t *testing.T, e *Engine, id, target state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision for %d, got %+v", id, d)
	}
	tidx := -1
	for _, o := range d.Options {
		if o.Obj == target {
			tidx = o.Index
		}
	}
	if tidx < 0 {
		t.Fatalf("target %d not offered: %+v", target, d.Options)
	}
	submitChoices(t, e, tidx)
	passUntilStackEmpty(t, e, 20)
}

// castDromokasGift casts Dromoka's Gift and returns the decision its
// resolution suspends on (the bolster election when the minimum is tied).
func castDromokasGift(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	id := handCardByName(t, e, "Dromoka's Gift")
	addMana(t, e, 0, "GGGGG")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Dromoka's Gift: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	return passUntilNonPriority(t, e, 20)
}

// castDromokasGiftResolved casts Dromoka's Gift and drains it fully (the
// pumped board has a unique least-toughness creature, so no election is
// posed).
func castDromokasGiftResolved(t *testing.T, e *Engine) {
	t.Helper()
	id := handCardByName(t, e, "Dromoka's Gift")
	addMana(t, e, 0, "GGGGG")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Dromoka's Gift: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
}

// handCardByName returns the id of the first card named name in seat 0's
// hand.
func handCardByName(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%q is not in seat 0's hand", name)
	return 0
}
