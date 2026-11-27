package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWitnessReadsOtherSacrificeVictims pins the witness over a planned
// other-sacrifice step (PaymentActivation.Sacrifices): Phyrexian Tower taps,
// sacrifices a victim, then adds {B}{B}; Ashnod's Altar neither taps nor
// moves itself, so its victim's zone change starts the activation. Without
// the plan's victims both read as producing nothing.
func TestWitnessReadsOtherSacrificeVictims(t *testing.T) {
	const payer state.PlayerID = 1
	sac := events.Event{Kind: events.MoveZone, Obj: 20, Player: payer, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"}
	mana := func(sym string, n int32) events.Event {
		return events.Event{Kind: events.ManaAdd, Player: payer, Amount: n, Counter: sym}
	}
	tower := []events.Event{{Kind: events.Tap, Obj: 5}, sac, mana("B", 1), mana("B", 1)}
	altar := []events.Event{sac, mana("C", 2)}
	for _, tc := range []struct {
		name    string
		source  state.ObjID
		evs     []events.Event
		victims []state.ObjID
		want    decision.ManaAmount
		ok      bool
	}{
		{"tower with its victim", 5, tower, []state.ObjID{20}, decision.ManaAmount{0, 0, 2, 0, 0, 0}, true},
		{"tower without the victim named", 5, tower, nil, decision.ManaAmount{}, true},
		{"altar with its victim", 6, altar, []state.ObjID{20}, decision.ManaAmount{0, 0, 0, 0, 0, 2}, true},
		{"altar without the victim named", 6, altar, nil, decision.ManaAmount{}, false},
	} {
		got, ok := activationProduction(tc.evs, tc.source, payer, tc.victims...)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: activationProduction = %v, %v; want %v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
