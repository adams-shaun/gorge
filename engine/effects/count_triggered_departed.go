package effects

import "github.com/adams-shaun/gorge/engine/state"

// triggeredAttackerDeparted reports the last-known power and toughness of the
// creature an Attacks trigger fired for when it has since left the battlefield
// (CR 608.2h): Emergent Woodwurm bounced with its attack trigger on the stack
// still looks at X cards, X being the power it had there. The card now in its
// new zone carries its printed face and none of the counters that gave it that
// power, so the counters are rebuilt from the log's departure record, the
// capture SacrificedLKI already uses. ok is false for any other ref, for an
// object still on the battlefield (its live layer output stands) and when the
// log holds no departure.
func triggeredAttackerDeparted(h Host, ref string, o *state.Object) (power, toughness int32, ok bool) {
	if ref != "TriggeredAttacker" && ref != "TriggeredAttackerLKICopy" {
		return 0, 0, false
	}
	if o == nil || o.Zone == state.ZBattlefield || o.Face() == nil {
		return 0, 0, false
	}
	cs, found := departedCounters(h, o.ID)
	if !found {
		return 0, 0, false
	}
	departed := state.Object{Counters: cs}
	dp, dt := departed.CounterPTTotals()
	return int32(o.Face().Power()) + dp, int32(o.Face().Toughness()) + dt, true
}
