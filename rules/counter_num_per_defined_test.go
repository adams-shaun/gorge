package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// CounterNumPerDefined$ (task param-putcounter-counternumperdefined): the
// count a PutCounter places is evaluated PER AFFECTED OBJECT, not once for
// the resolving source. Both corpus pins are real cards:
//
//   - Canopy Gargantuan's upkeep trigger (`CounterNumPerDefined$ X`,
//     `SVar:X:Count$CardToughness`) puts +1/+1 counters on each other
//     creature you control equal to THAT creature's toughness. Before the
//     fix the key was unread and the shared CounterNum$ path defaulted to
//     1, so every creature took exactly one.
//   - Jared Carthalion's [-3] (`CounterNumPerDefined$ X`,
//     `SVar:X:Count$CardNumColors`) puts counters equal to the number of
//     colours each chosen creature is -- the per-defined read on an
//     ACTIVATED ability, exercising the new Count$CardNumColors head.
//
// The third corpus carrier (Sovereign Okinec Ahau) stays un-pinned: its
// `Defined$ Valid Creature.YouCtrl+powerGTbasePower` spec cannot resolve the
// `powerGTbasePower` predicate's non-literal RHS in the filter grammar
// today, so its Defined set is empty regardless of this primitive -- that
// filter-side gap is filed separately, not silently papered over here.

func TestCanopyGargantuanPutsToughnessCountersOnEachOtherCreature(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Canopy Gargantuan"))
	canopy := e.G.Zone(state.ZHand, 0)[0]
	// Seat-0 companions with DIFFERENT toughnesses (the precondition the
	// per-defined read depends on: one value cannot tell a per-object count
	// from a flat one), and a seat-1 creature the trigger must not touch.
	tall := onBoardCard(t, e, 0, card(t, "Name:Tall\nTypes:Creature Human\nPT:2/3\nOracle:x\n"))
	small := onBoardCard(t, e, 0, card(t, "Name:Small\nTypes:Creature Human\nPT:1/1\nOracle:x\n"))
	enemy := onBoardCard(t, e, 1, card(t, "Name:Enemy\nTypes:Creature Human\nPT:2/2\nOracle:x\n"))
	placeFromHand(t, e, canopy)
	if e.G.Obj(canopy).Zone != state.ZBattlefield {
		t.Fatalf("Canopy in %s, want battlefield", e.G.Obj(canopy).Zone)
	}
	if got := e.Toughness(tall); got != 3 {
		t.Fatalf("precondition: Tall toughness %d, want 3", got)
	}
	if got := e.Toughness(small); got != 1 {
		t.Fatalf("precondition: Small toughness %d, want 1 (the two must differ)", got)
	}
	if got := e.Toughness(canopy); got != 7 {
		t.Fatalf("precondition: Canopy toughness %d, want 7", got)
	}
	e.pending = nil
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("queued triggers = %d, want the Canopy upkeep trigger", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if n := e.G.Obj(tall).Counter("P1P1"); n != 3 {
		t.Fatalf("Tall got %d +1/+1 counters, want its toughness 3", n)
	}
	if n := e.G.Obj(small).Counter("P1P1"); n != 1 {
		t.Fatalf("Small got %d +1/+1 counters, want its toughness 1", n)
	}
	if n := e.G.Obj(canopy).Counter("P1P1"); n != 0 {
		t.Fatalf("Canopy itself got %d counters, want 0 (Defined$ ...+Other)", n)
	}
	if n := e.G.Obj(enemy).Counter("P1P1"); n != 0 {
		t.Fatalf("the opponent's creature got %d counters, want 0 (YouCtrl)", n)
	}
}

func TestJaredCarthalionMinus3PutsColorCountPerCreature(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Jared Carthalion"))
	jared := e.G.Zone(state.ZHand, 0)[0]
	placeFromHand(t, e, jared)
	if got := e.G.Obj(jared).Counter("LOYALTY"); got != 5 {
		t.Fatalf("precondition: Jared entered with %d loyalty, want the printed 5", got)
	}
	mono := onBoardCard(t, e, 0, card(t, "Name:Mono\nTypes:Creature Human\nManaCost:2 G\nPT:2/2\nOracle:x\n"))
	multi := onBoardCard(t, e, 0, card(t, "Name:Multi\nTypes:Creature Human\nManaCost:W U\nPT:2/2\nOracle:x\n"))
	if effects.ColorsOf(e.G.Obj(mono)) != "G" || effects.ColorsOf(e.G.Obj(multi)) != "WU" {
		t.Fatalf("precondition: colours mono=%q multi=%q", effects.ColorsOf(e.G.Obj(mono)), effects.ColorsOf(e.G.Obj(multi)))
	}
	// The [-3] is Jared's second A: line.
	var opt decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == jared && o.Ability == 1 {
			opt, found = o, true
			break
		}
	}
	if !found {
		t.Fatalf("Jared's [-3] ability not offered: %+v", e.legalActions(0))
	}
	e.beginActivation(0, opt)
	answerTargetAsk(t, e, []state.ObjID{mono, multi})
	passUntilStackEmpty(t, e, 60)
	if n := e.G.Obj(mono).Counter("P1P1"); n != 1 {
		t.Fatalf("the monocoloured creature got %d +1/+1 counters, want 1 (its colour count)", n)
	}
	if n := e.G.Obj(multi).Counter("P1P1"); n != 2 {
		t.Fatalf("the two-coloured creature got %d +1/+1 counters, want 2", n)
	}
	if got := e.G.Obj(jared).Counter("LOYALTY"); got != 2 {
		t.Fatalf("Jared's loyalty after the [-3] = %d, want 2 (5-3)", got)
	}
}
