package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestTeapotSlingerManaExpendCountsConvoke uses the corpus Convoke spell
// Crowd's Favor: under CR 702.50 its tapped creature pays for the red mana,
// so that contribution crosses the real carrier's expend-4 threshold.
func TestTeapotSlingerManaExpendCountsConvoke(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Teapot Slinger"))

	// Build a pre-payment base of three actual pool mana before the carrier is
	// on the battlefield, so the carrier does not fire on this cast.
	first := expendVanilla(t, e, "3")
	e.G.Players[0].Pool[state.MC] = 3
	castExpendVehicle(t, e, first)
	if got := manaExpendedOf(e, 0); got != 3 {
		t.Fatalf("test precondition: initial pool spend = %d, want 3", got)
	}
	e.resolveTop()
	teapot := expendTeapot(t, e)
	if e.G.Obj(teapot).Zone != state.ZBattlefield {
		t.Fatal("test precondition: Teapot Slinger is not on the battlefield")
	}

	// Crowd's Favor is the real corpus card. Its red creature pays its {R}
	// through Convoke, with no mana in the pool: the before/after values must
	// genuinely straddle four (3 != 4).
	creature := e.G.AddObject(card(t, "Name:Convoke Helper\nManaCost:R\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"), 0)
	creature.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), creature.ID))
	favorCard := corpusAlternativeCard(t, "Crowd's Favor")
	favor := e.G.AddObject(favorCard, 0)
	favor.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), favor.ID))
	if e.G.Obj(favor.ID).Zone != state.ZHand || e.G.Players[0].Pool.Total() != 0 {
		t.Fatal("test precondition: Crowd's Favor is not in hand or pool is not empty")
	}

	castMode(t, e, favor.ID, "")
	askedConvoke := false
	for i := 0; i < 8 && e.cast != nil; i++ {
		d := e.Pending()
		if d == nil || len(d.Options) == 0 {
			t.Fatalf("Crowd's Favor stalled before payment: decision=%+v", d)
		}
		choice := 0
		for index, option := range d.Options {
			if option.Kind == "convoke_R" && option.Obj == creature.ID {
				askedConvoke = true
				choice = index
				break
			}
		}
		submitChoices(t, e, choice)
	}
	if !askedConvoke || e.G.Obj(favor.ID).Zone != state.ZStack || e.cast != nil {
		t.Fatalf("test precondition: corpus Convoke payment did not complete (asked=%v, zone=%s)", askedConvoke, e.G.Obj(favor.ID).Zone)
	}
	if !e.G.Obj(creature.ID).Tapped {
		t.Fatalf("test precondition: selected Convoke creature was not tapped (payments=%+v)", e.cast.convoke)
	}
	got := manaExpendedOf(e, 0)
	if got != 4 || got == 3 {
		t.Fatalf("three prior pool mana plus one Convoke mana = %d, want 4 (pool-only value would be 3)", got)
	}
	var wake *events.Event
	for i := range e.L.Events {
		event := &e.L.Events[i]
		if event.Kind == events.CastInfo && events.FlagsFrom(event.Counter)&state.FlagManaExpendCast != 0 {
			wake = event
		}
	}
	if wake == nil || wake.Amount != 1 {
		t.Fatalf("test precondition: expected one Convoke mana in the pay-time wake event, got %+v", wake)
	}
	matched := false
	for _, trigger := range e.G.Obj(teapot).Face().Triggers {
		if e.manaExpendMatches(trigger, teapot, *wake, nil) {
			matched = true
		}
	}
	if !matched {
		t.Fatalf("the real expend-4 crossing did not match Teapot Slinger's trigger: %+v", *wake)
	}
}
