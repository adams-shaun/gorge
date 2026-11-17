package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const xCensusQuirionSrc = "Name:Quirion Source\nTypes:Creature Elf Druid\nPT:1/1\n" +
	"A:AB$ Mana | Cost$ T | Produced$ G\n" +
	"A:AB$ Mana | Cost$ T | Produced$ W\nOracle:x\n"

const xCensusRestrictedSrc = "Name:Restricted Source\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ Spell.Creature\nOracle:x\n"

// xCensusBoard places the grower's synthetic "you may pay {X}" trigger beside
// the named sources, ready to tap (no summoning sickness), and returns the
// grower and each source's id in order.
func xCensusBoard(t *testing.T, extras ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Shanna, Purifying Blade")
	grower := onBoard(t, e, 0, xGrowerSrc)
	ids := make([]state.ObjID, 0, len(extras))
	for _, src := range extras {
		ids = append(ids, onBoardReady(t, e, 0, src))
	}
	return e, grower, ids
}

// TestTriggeredCostXSupplyFiltersRestrictedCensusMana pins the per-source
// one-tap supply against restricted mana: the potential walk excludes
// Secluded Courtyard-style mana a trigger cost cannot spend, so the census
// must exclude it too, not add back the per-ability over-count it was meant
// to correct.
func TestTriggeredCostXSupplyFiltersRestrictedCensusMana(t *testing.T) {
	t.Parallel()
	e, grower, _ := xCensusBoard(t, xCensusQuirionSrc, xCensusRestrictedSrc)
	desc := paymentDescriptor{ID: grower, Class: paymentOther, Cost: &Cost{X: 1}}
	if got := e.potentialManaFor(0, &desc).Total(); got != 2 {
		t.Fatalf("precondition: potential bound %d, want 2 (Quirion's two faces; the restricted source is excluded)", got)
	}
	tc := &triggeredEffectCost{source: grower, player: 0, amount: Cost{X: 1}}
	admits := func(id state.ObjID, ma *cards.SA) bool {
		return potentialRestrictionAdmits(e, tc.player, id, ma, &desc)
	}
	pool := pay.AvailableFor(asPayer(e), tc.player, desc).Pool.Total()
	if got := triggeredCostXSupply(pool, pay.PaymentPlanQueryUnits(asPayer(e), tc.player), admits); got != 1 {
		t.Fatalf("X supply %d, want 1 (Quirion's one tap; the restricted source cannot pay)", got)
	}
}

// TestTriggeredCostXOfferCapsAtThePayableSources is the user-visible form:
// the announcement offers only the X values the one-tap supply can pay.
func TestTriggeredCostXOfferCapsAtThePayableSources(t *testing.T) {
	t.Parallel()
	e, _, _ := xCensusBoard(t, xCensusQuirionSrc, xCensusRestrictedSrc)
	entrant := e.G.AddObject(card(t, "Name:Big Entrant\nManaCost:X G\nTypes:Creature Hydra\nPT:0/0\nOracle:x\n"), 0)
	entrant.Zone = state.ZLibrary
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	e.G.SetZone(state.ZLibrary, 0, append(ids, entrant.ID))
	entrant.X = 2
	e.emit(events.Event{Kind: events.MoveZone, Obj: entrant.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
	d := passUntilNonPriority(t, e, 80)
	if d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("expected the choose-X announcement ask, got %+v", d)
	}
	got := make([]int, 0, len(d.Options))
	for _, o := range d.Options {
		got = append(got, o.Amount)
	}
	want := []int{0, 1}
	if len(got) != len(want) {
		t.Fatalf("X options %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("X options %v, want %v", got, want)
		}
	}
}
