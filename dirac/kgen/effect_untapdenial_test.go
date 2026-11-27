package kgen

import (
	"strings"
	"testing"
)

// TestEffectUntapDenialRefusesBothBodies: an Effect that carries both the
// Exhaustion static and the Blinding Beam replacement is neither shape
// exactly. The static branch would take the grant form and leave the
// replacement unread, so the lowering refuses it by name instead of silently
// dropping one body.
func TestEffectUntapDenialRefusesBothBodies(t *testing.T) {
	const probe = `Name:Both Bodies Probe
ManaCost:2 U
Types:Sorcery
A:SP$ Effect | ValidTgts$ Opponent | StaticAbilities$ DontUntap | ReplacementEffects$ CantUntap | Triggers$ RemoveEffect | RememberObjects$ Targeted | Duration$ Permanent | SpellDescription$ probe
SVar:DontUntap:Mode$ Continuous | Affected$ Creature.RememberedPlayerCtrl,Land.RememberedPlayerCtrl | AffectedZone$ Battlefield | AddHiddenKeyword$ This card doesn't untap during your next untap step.
SVar:CantUntap:Event$ Untap | ValidCard$ Creature | ValidStepTurnToController$ Player.IsRemembered | Layer$ CantHappen
SVar:RemoveEffect:Mode$ Phase | Phase$ Untap | ValidPlayer$ Player.IsRemembered | Static$ True | TriggerZones$ Command | Execute$ ExileEffect
SVar:ExileEffect:DB$ ChangeZone | Destination$ Exile | Defined$ Self | Origin$ Command
Oracle:probe
`
	card := lowerScriptCard(t, "Both Bodies Probe", probe)
	var got string
	for _, rf := range card.Refusals {
		if strings.Contains(rf.Reason, "Effect untap denial") {
			got = rf.Reason
		}
	}
	if !strings.Contains(got, "both StaticAbilities$ and ReplacementEffects$") {
		t.Fatalf("refusals %v, want the both-bodies by-name refusal", card.Refusals)
	}
	for _, a := range card.Abilities {
		for _, in := range a.Prog {
			if in.Op == "SkipUntap" {
				t.Errorf("both-bodies Effect lowered to %s, want it refused", in)
			}
		}
	}
}
