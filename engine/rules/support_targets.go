package rules

import (
	"maps"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// supportTargetSA returns sa unchanged unless it is a SPELL or ACTIVATED
// ability whose PutCounter body carries Support$ N. CR 701.41a: on an
// instant or sorcery, Support N is "put a +1/+1 counter on each of up to N
// target creatures", and CR 601.2c announces those targets while casting --
// not as a resolution choice. Forge's CountersPutEffect Support branch
// installs TargetMin 0 / TargetMax N / ValidTgts at cast for the same
// reason. On an ACTIVATED ability (Joraga Auxiliary's "{4}{G}{W}: Support
// 2") the announcement is CR 602.2b's: the rest of activating an ability is
// the casting process, so the cast flow's target ask poses the same
// synthesized declaration at activation.
//
// The engine's generic targeting machinery already owns the cast-time ask,
// the CR 601.2c recording and the CR 608.2b recheck. This helper gives it
// the target declaration the Support keyword implies: a clone of the spell
// ability with ValidTgts$/TargetMin$/TargetMax$ synthesized (the SA's own
// Choices$ spec when it has one, else the CR 701.41a split -- a true
// instant/sorcery spell's "Creature", a permanent spell's
// "Creature.+Other"). The clone is only ever handed to a targeting read; the
// parsed ability and its cached facts are untouched.
//
// A permanent's Support (an ETB trigger) is unchanged: it keeps the
// resolution counter_pick, whose "other" exclusion CR 701.41a spells out.
func supportTargetSA(sa *cards.SA, f *cards.Face) *cards.SA {
	if sa == nil {
		return sa
	}
	kind := sa.KindCode()
	if kind != cards.SAKindSpell && kind != cards.SAKindActivated {
		return sa
	}
	pc := effects.PutCounterOf(sa)
	if !pc.Support.Present {
		return sa
	}
	spec := pc.Choices // as putCounterSupport reads it
	if spec == "" {
		spec = "Creature.+Other"
		if kind == cards.SAKindSpell && f != nil && (f.IsInstant() || f.IsSorcery()) {
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
