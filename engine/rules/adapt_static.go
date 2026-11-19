package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:CanAdapt is Biomancer's Familiar's `Mode$ CanAdapt` static (CR 702.35a's
// condition read "as though it had no +1/+1 counters on it"). The corpus
// delivers it only from an Effect's StaticAbilities$ (`The next time target
// creature adapts this turn`), registered by effects' effectUnsupportedStatic
// with the Effect's remembered creature; the Effect's Mode$ Adapt trigger
// then ends it (trigmatch/adapt.go).
func init() { effects.RegisterNonAPI("stat:CanAdapt") }

// AdaptsAsIfNoCounters reports whether a live Effect-delivered CanAdapt grant
// covers the creature id, in e.active()'s deterministic order. Both halves of
// an Adapt$ ability read it: the offer-time gate (adaptGateOK) and the
// resolve-time condition (effects' putCounterAdaptBlocked).
func (e *Engine) AdaptsAsIfNoCounters(id state.ObjID) bool {
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.Restriction == effects.ModeCanAdapt && e.restrictionApplies(ce, id) {
			return true
		}
	}
	return false
}
