package rules

import "github.com/adams-shaun/gorge/engine/state"

// ReleaseImprintedEffects is the ClearImprinted$ half of the ImprintOnHost$
// idiom (effects.ImprintedEffectReleaser). Forge's Cleanup that clears the
// imprinted list makes the host card forget the effect token an
// ImprintOnHost$ True Effect imprinted on it: the effect itself lives on
// (a permanent duration), but a later ChangeZone of the imprinted cards from
// the command zone to exile on the same host no longer reaches it. Superior Foes of Spider-Man's Static$ True leaves-the-battlefield
// cleanup is the load-bearing carrier: "you may play that card until you
// exile another card with THIS creature" -- once the creature leaves the
// battlefield it is a new object when it returns (CR 400.7), so a card the
// returned creature exiles must not end the old card's permission. This
// build has no effect-token object (EndImprintedEffects), so the release
// clears the ImprintOnHost marker on the source's live registrations;
// EndImprintedEffects then skips them. Engine-runtime only, rebuilt by
// re-execution on replay exactly like EndImprintedEffects. Delayed-trigger
// registrations keep their marker: no ClearImprinted$ carrier releases a
// delayed one before its own ender runs (Semester's End exiles first).
func (e *Engine) ReleaseImprintedEffects(source state.ObjID) {
	if source == 0 {
		return
	}
	changed := false
	for i := range e.continuous {
		if e.continuous[i].Source == source && e.continuous[i].ImprintOnHost {
			e.continuous[i].ImprintOnHost = false
			changed = true
		}
	}
	if changed {
		e.continuousChanged()
	}
}
