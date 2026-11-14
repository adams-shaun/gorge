package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBarePreventionsApplyWithoutOrderAsk activates two copies of the
// Cephalid-shaped ward (effect_validsource_remembered_test.go) on one
// creature. Its combat damage, dealt or taken, then matches two bare
// Prevent$ True lines. Either one prevents all of it and logs the same Note,
// so no CR 616.1 order ask is posed and the damage is prevented.
func TestBarePreventionsApplyWithoutOrderAsk(t *testing.T) {
	t.Parallel()
	ward := card(t, preventWardScript)
	shielded := card(t, "Name:Shielded Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 84, ward, ward, shielded)
	w1 := moveSeededCard(t, e, 0, ward, state.ZBattlefield)
	shieldedID := moveSeededCard(t, e, 0, shielded, state.ZBattlefield)
	w2 := moveSeededCard(t, e, 0, ward, state.ZBattlefield)
	if w2 == w1 || e.G.Obj(w2).Zone != state.ZBattlefield {
		t.Fatalf("precondition: want two wards on the battlefield, got %d and %d", w1, w2)
	}
	activatePreventWard(t, e, w1, shieldedID)
	activatePreventWard(t, e, w2, shieldedID)

	// Precondition: two separate Effects each hold both halves for the
	// shielded creature, so each damage event below has two candidates.
	halves := map[string]int{}
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "DamageDone" && len(ce.Remembered) == 1 && ce.Remembered[0] == shieldedID {
			if ce.ReplacementParams[cards.PKValidSource.String()] == "Card.IsRemembered" {
				halves["from"]++
			}
			if ce.ReplacementParams[cards.PKValidTarget.String()] == "Card.IsRemembered" {
				halves["to"]++
			}
		}
	}
	if halves["from"] != 2 || halves["to"] != 2 {
		t.Fatalf("precondition: want two of each half capturing the shielded creature, got %v", halves)
	}

	before := e.G.Players[1].Life
	e.damaging, e.combatDamaging = shieldedID, true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.damaging, e.combatDamaging = 0, false
	if len(e.replChoices) != 0 {
		t.Fatalf("combat damage dealt by the shielded creature parked an order ask between identical preventions: %d candidates", len(e.replChoices[0].cands))
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("combat damage dealt by the shielded creature posed an order ask between identical preventions: %+v", d.Options)
	}
	if lost := before - e.G.Players[1].Life; lost != 0 {
		t.Fatalf("combat damage dealt by the shielded creature: player lost %d, want 0 (prevented)", lost)
	}

	e.damaging, e.combatDamaging = w1, true
	e.emit(events.Event{Kind: events.Damage, Obj: shieldedID, Amount: 2})
	e.damaging, e.combatDamaging = 0, false
	if len(e.replChoices) != 0 {
		t.Fatalf("combat damage dealt to the shielded creature parked an order ask between identical preventions: %d candidates", len(e.replChoices[0].cands))
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("combat damage dealt to the shielded creature posed an order ask between identical preventions: %+v", d.Options)
	}
	if dmg := e.G.Obj(shieldedID).Damage; dmg != 0 {
		t.Fatalf("combat damage dealt to the shielded creature: marked %d, want 0 (prevented)", dmg)
	}
	replayCheck(t, e, cfg)
}
