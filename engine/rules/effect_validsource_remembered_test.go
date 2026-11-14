package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect-created bodyless prevention whose ValidSource$ is
// Card.IsRemembered (Maze of Ith, Cephalid Illusionist, Kor Haven: "prevent
// all combat damage that would be dealt by target creature this turn") names
// the creature the Effect captured with RememberObjects$ Targeted. The Effect
// never writes that creature onto its host's own remembered list, so a source
// filter that reads only the host's list matches nothing and the damage
// lands. The fixture is authored (never a corpus .txt): an artifact whose
// ability grants both halves of the Cephalid shape to a target creature.

const preventWardScript = "Name:Prevent Ward\nTypes:Artifact\n" +
	"A:AB$ Effect | Cost$ U | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | " +
	"ReplacementEffects$ RPFrom,RPTo | RememberObjects$ Targeted | ExileOnMoved$ Battlefield | " +
	"SpellDescription$ Prevent all combat damage dealt to and by target creature you control this turn.\n" +
	"SVar:RPFrom:Event$ DamageDone | Prevent$ True | IsCombat$ True | ValidSource$ Card.IsRemembered | Description$ from\n" +
	"SVar:RPTo:Event$ DamageDone | Prevent$ True | IsCombat$ True | ValidTarget$ Card.IsRemembered | Description$ to | Secondary$ True\n" +
	"Oracle:x\n"

// TestEffectValidSourceRememberedPreventsCapturedSource activates the ward
// on one of two creatures and deals damage from each: only the named
// creature's combat damage is prevented; its noncombat damage and the other
// creature's combat damage land.
func TestEffectValidSourceRememberedPreventsCapturedSource(t *testing.T) {
	t.Parallel()
	ward := card(t, preventWardScript)
	shielded := card(t, "Name:Shielded Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	plain := card(t, "Name:Plain Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 83, ward, shielded, plain)
	wardID := moveSeededCard(t, e, 0, ward, state.ZBattlefield)
	shieldedID := moveSeededCard(t, e, 0, shielded, state.ZBattlefield)
	plainID := moveSeededCard(t, e, 0, plain, state.ZBattlefield)
	activatePreventWard(t, e, wardID, shieldedID)

	// Preconditions: the Effect's from-half is registered and captured the
	// shielded creature, and the host's OWN remembered list does not hold it
	// (the list the source filter read alone).
	captured := false
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "DamageDone" && ce.ReplacementParams[cards.PKValidSource.String()] == "Card.IsRemembered" &&
			len(ce.Remembered) == 1 && ce.Remembered[0] == shieldedID {
			captured = true
		}
	}
	if !captured {
		t.Fatal("precondition: no active ValidSource$ Card.IsRemembered prevention capturing the shielded creature")
	}
	for _, r := range e.G.Obj(wardID).Remembered {
		if r.Obj == shieldedID {
			t.Fatal("precondition: the ward's own remembered list holds the shielded creature; the test would not exercise the capture")
		}
	}

	deal := func(src state.ObjID, combat bool) int32 {
		before := e.G.Players[1].Life
		e.damaging, e.combatDamaging = src, combat
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
		e.damaging, e.combatDamaging = 0, false
		return before - e.G.Players[1].Life
	}
	if got := deal(shieldedID, true); got != 0 {
		t.Fatalf("combat damage dealt by the remembered creature: player lost %d, want 0 (prevented)", got)
	}
	if got := deal(plainID, true); got != 2 {
		t.Fatalf("combat damage dealt by another creature: player lost %d, want 2", got)
	}
	if got := deal(shieldedID, false); got != 2 {
		t.Fatalf("noncombat damage dealt by the remembered creature: player lost %d, want 2 (IsCombat$ True)", got)
	}
	replayCheck(t, e, cfg)
}

// activatePreventWard funds {U}, activates the ward targeting target and
// drains the stack.
func activatePreventWard(t *testing.T, e *Engine, ward, target state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "U")
	submitChoices(t, e, abilityOption(t, e, ward, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision for the ward: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == target {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("target %d not offerable: %+v", target, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
}
