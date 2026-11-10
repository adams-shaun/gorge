package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedSpellCastSourceReadsLastKnownPower pins CR 608.2h for Loki
// Laufeyson's delayed "when you next cast an instant or sorcery spell with
// mana value less than or equal to Loki's power": once the source has left the
// battlefield the bound is the power it last had there, not the printed power
// of the card now in the graveyard.
func TestDelayedSpellCastSourceReadsLastKnownPower(t *testing.T) {
	t.Parallel()
	const srcText = "Name:Test Loki\nManaCost:1 R\nTypes:Creature God\nPT:2/2\nSVar:X:Count$CardPower\n" +
		"SVar:CopyIt:DB$ CopySpellAbility | Defined$ TriggeredSpellAbility\nOracle:x\n"
	instant := func(cmc string) string {
		return "Name:Test Bolt " + cmc + "\nManaCost:" + cmc + "\nTypes:Instant\nOracle:x\n"
	}
	for _, tc := range []struct {
		name     string
		pumps    int32 // +1/+1 counters put on the source after the registration
		departs  bool
		cmc      string // mana cost of the cast instant (3 or 2 mana)
		wantFire bool
		power    int32 // battlefield power before any departure
	}{
		{"live power 4 fires a three-mana cast", 2, false, "3", true, 4},
		{"departed after a pump: last known 4 fires a three-mana cast", 2, true, "3", true, 4},
		{"departed without a pump: last known 2 misses a three-mana cast", 0, true, "3", false, 2},
		{"departed without a pump: last known 2 fires a two-mana cast", 0, true, "2", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := layerEngine(t)
			src := onBoard(t, e, 0, srcText)
			e.emit(events.Event{Kind: events.DelayedRegister, Obj: src, Player: 0, Step: e.G.Step,
				Counter: "CopyIt", Text: "SpellCast:Mode$ SpellCast | ValidCard$ Instant.cmcLEX"})
			if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != "SpellCast" {
				t.Fatalf("precondition: registration = %+v", e.G.Delayed)
			}
			if tc.pumps > 0 {
				e.emit(events.Event{Kind: events.CounterChange, Obj: src, Counter: "P1P1", Amount: tc.pumps})
			}
			// Precondition: the battlefield power differs from the printed 2
			// exactly when the case pumped.
			if got := e.Power(src); got != tc.power {
				t.Fatalf("precondition: battlefield power = %d, want %d", got, tc.power)
			}
			if tc.departs {
				e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZGraveyard})
				if z := e.G.Obj(src).Zone; z != state.ZGraveyard {
					t.Fatalf("precondition: source in %v, want the graveyard", z)
				}
				if len(e.G.Delayed) != 1 {
					t.Fatalf("precondition: the registration did not survive the departure: %+v", e.G.Delayed)
				}
			}
			bolt := e.G.AddObject(card(t, instant(tc.cmc)), 0)
			bolt.Zone = state.ZHand
			e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), bolt.ID))
			e.emit(events.Event{Kind: events.PutOnStack, Obj: bolt.ID, Player: 0, From: state.ZHand, To: state.ZStack})
			want := 0
			if tc.wantFire {
				want = 1
			}
			if got := len(e.pendingTriggers); got != want {
				t.Fatalf("pending triggers = %d, want %d (delayed left %d)", got, want, len(e.G.Delayed))
			}
		})
	}
}
