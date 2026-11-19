package rules

import "github.com/adams-shaun/gorge/engine/effects"

// stat:CanAdapt is Biomancer's Familiar's `Mode$ CanAdapt` static (CR 702.35a's
// condition read "as though it had no +1/+1 counters on it"). The corpus
// delivers it only from an Effect's StaticAbilities$ (`The next time target
// creature adapts this turn`), registered by effects' effectUnsupportedStatic
// with the Effect's remembered creature and read through
// RestrictionApplies(effects.ModeCanAdapt, id) by both halves of an Adapt$
// ability: the offer-time gate (adaptGateOK) and the resolve-time condition
// (effects' putCounterAdaptBlocked). The Effect's Mode$ Adapt trigger then ends
// the grant (trigmatch/adapt.go).
func init() { effects.RegisterNonAPI("stat:CanAdapt") }
