package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// CounterNumPerDefined$ (task param-putcounter-counternumperdefined): the
// count a PutCounter places is evaluated PER AFFECTED OBJECT, not once for
// the resolving source. The pin is a real card:
//
//   - Canopy Gargantuan's upkeep trigger (`CounterNumPerDefined$ X`,
//     `SVar:X:Count$CardToughness`) puts +1/+1 counters on each other
//     creature you control equal to THAT creature's toughness. Before the
//     fix the key was unread and the shared CounterNum$ path defaulted to
//     1, so every creature took exactly one.
//
// Two other carriers are deliberately NOT pinned here, and their count
// heads stay unread:
//
//   - Jared Carthalion's [-3] needs `Count$CardNumColors`, which must read
//     an object's LIVE derived colours (rules/layers.go's Engine.Colors
//     read) — the face-only ColorMaskOf is wrong under a layer-5 colour
//     effect (Leyline of the Guildpact etc.). Filed as its own ticket.
//   - Sovereign Okinec Ahau needs `Count$CardBasePower` (a derived base
//     characteristic after layer-7b SetPower statics) AND its
//     `Defined$ Valid Creature.YouCtrl+powerGTbasePower` spec cannot resolve
//     the non-literal `basePower` predicate RHS today (separate filter
//     ticket), so its Defined set is empty regardless — not silently
//     papered over here.

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
