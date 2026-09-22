package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestPlayerCountHasPropertyStateBacked(t *testing.T) {
	h, c := fixtureHost(t)
	h.g.HasMonarch = true
	h.g.Monarch = 1
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyisMonarch"); !ok || got != 1 {
		t.Fatalf("monarch = (%d, %v)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyisMonarch"); !ok || got != 1 {
		t.Fatalf("opponent monarch = (%d, %v)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRegisteredOpponents$HasPropertyisMonarch"); !ok || got != 1 {
		t.Fatalf("registered monarch = (%d, %v)", got, ok)
	}

	grave := h.g.AddObject(mkCard(t, "Name:Spell\nTypes:Instant\nOracle:x\n"), 1)
	hand := h.g.AddObject(mkCard(t, "Name:Creature\nTypes:Creature\nOracle:x\n"), 1)
	h.g.SetZone(state.ZGraveyard, 1, []state.ObjID{grave.ID})
	h.g.SetZone(state.ZHand, 1, []state.ObjID{hand.ID})
	grave.Zone, hand.Zone = state.ZGraveyard, state.ZHand
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInGraveyard_Instant_GE1"); !ok || got != 1 {
		t.Fatalf("graveyard count = (%d, %v)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GE1/Times.2"); !ok || got != 2 {
		t.Fatalf("hand count = (%d, %v)", got, ok)
	}

	h.dmgTaken = map[state.PlayerID]int32{1: 2, 0: 4}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRegisteredOpponents$HasPropertywasDealtDamageThisTurn"); !ok || got != 1 {
		t.Fatalf("damage count = (%d, %v)", got, ok)
	}
	h.combatHits = []CombatDamageHit{{Player: 1, Amount: 1}}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertywasDealtCombatDamageThisTurn"); !ok || got != 1 {
		t.Fatalf("combat count = (%d, %v)", got, ok)
	}

	for _, expr := range []string{"Count$PlayerCountOpponents$HasPropertyNoSuchProperty", "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Card_BAD1"} {
		if got, ok := EvalCountOK(h, c, expr); ok {
			t.Errorf("%s = (%d, true), want unresolvable", expr, got)
		}
	}
}
