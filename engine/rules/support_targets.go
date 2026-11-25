package rules

import (
	"maps"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// supportTargetSA returns sa unchanged unless its PutCounter body carries
// Support$ N, in which case it returns a clone declaring the targets the
// keyword implies. CR 701.41a: Support N is "put a +1/+1 counter on each of
// up to N target creatures" (an instant or sorcery's spelling; a permanent's
// ability says "other"). Those targets are announced when the ability is put
// on the stack -- CR 601.2c for a spell, CR 603.3d for a triggered ability
// (including a trigger whose body carries a Cost$, which compiles as an AB$
// SA), CR 602.2b for an activated one -- never chosen as a resolution pick.
// Forge expands the keyword to an ETB trigger whose CountersPutEffect
// Support branch installs TargetMin 0 / TargetMax N / ValidTgts, and XMage's
// SupportAbility adds the same targets to the triggered ability.
//
// The engine's generic targeting machinery already owns the placement ask,
// the announced-target recording and the CR 608.2b recheck. This helper
// gives it the declaration the Support keyword implies: a clone of the
// ability with ValidTgts$/TargetMin$/TargetMax$ synthesized (the SA's own
// Choices$ spec when it has one, else the CR 701.41a split -- an
// instant/sorcery's "Creature", every other ability's "Creature.+Other").
// The clone is only ever handed to a targeting read and to the
// TargetsOffered marker that tells putCounterSupport the ask already
// happened; the parsed ability and its cached facts are untouched.
func supportTargetSA(sa *cards.SA, f *cards.Face) *cards.SA {
	if sa == nil {
		return sa
	}
	pc := effects.PutCounterOf(sa)
	if !pc.Support.Present {
		return sa
	}
	spec := pc.Choices // as putCounterSupport reads it
	if spec == "" {
		spec = "Creature.+Other"
		if f != nil && (f.IsInstant() || f.IsSorcery()) {
			spec = "Creature"
		}
	}
	max := pc.Support.Text
	if max == "" {
		max = "1"
	}
	clone := *sa
	clone.Params = maps.Clone(sa.Params)
	if clone.Params == nil {
		clone.Params = map[string]string{}
	}
	clone.SetParam(cards.PKValidTgts, spec)
	clone.SetParam(cards.PKTargetMin, "0")
	clone.SetParam(cards.PKTargetMax, max)
	return &clone
}
