package effects

import "github.com/adams-shaun/gorge/engine/state"

// refDepartedPT is the last-known power and toughness of a referent that has
// left the battlefield, for the count path's per-object P/T reads: a target
// this chain captured (targetPTLKI) or, for the creature an Attacks trigger
// fired for, the value rebuilt from its departure record (CR 608.2h).
// Emergent Woodwurm bounced with its attack trigger on the stack still looks at
// X cards, X being the power it had there; the card now in its new zone carries
// its printed face and none of the counters that gave it that power, so the
// counters come from the log, the capture SacrificedLKI already uses. ok is
// false for an object still on the battlefield (its live layer output stands),
// for any other ref and when the log holds no departure.
func refDepartedPT(h Host, c *Ctx, ref string, o *state.Object) (TargetPT, bool) {
	if pt, ok := targetPTLKI(c, o); ok {
		return pt, true
	}
	if ref != "TriggeredAttacker" && ref != "TriggeredAttackerLKICopy" {
		return TargetPT{}, false
	}
	// A characteristic-defining P/T functions in every zone (CR 208.2a, 604.3):
	// Face.Power holds only the star's fixed term, so refPower's layer read
	// stays authoritative (Ajani Goldmane's Avatar token under Mage Slayer).
	if o == nil || o.Zone == state.ZBattlefield || o.Face() == nil || o.Face().CharacteristicDefining() {
		return TargetPT{}, false
	}
	cs, found := departedCounters(h, o.ID)
	if !found {
		return TargetPT{}, false
	}
	departed := state.Object{Counters: cs}
	dp, dt := departed.CounterPTTotals()
	return TargetPT{Power: int32(o.Face().Power()) + dp, Toughness: int32(o.Face().Toughness()) + dt}, true
}
