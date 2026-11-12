package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// changeTargetGateMet evaluates the ChangeTargets gate's two Condition* keys
// (Forge's SpellAbilityCondition.areMet, the Meddle / Quicksilver Dragon
// shape), which conditionMet never reads. The redirect runs only when
//
//   - ConditionTargetValidTargeting$ is absent, or ANY target of the
//     redirected spell -- across its root declaration and every chain link
//     holding announced targets -- matches one of the value's comma-split
//     specs, read relative to the resolving ability's controller; and
//   - ConditionTargetsSingleTarget$ is absent, or the union of the spell's
//     DISTINCT targets over that whole chain is exactly one.
//
// A spell with no target at all meets neither (Forge answers false for both
// when nothing is targeted), so an unmet gate leaves the spell's targets
// alone (CR 115.7a: otherwise nothing changes). declared is effChangeTargets'
// own reader of each link's announced targets; subject is the redirected
// spell's own SA.
func changeTargetGateMet(h Host, c *Ctx, sa, subject *cards.SA, declared func(*cards.SA) []state.Target) bool {
	cp := &ActivationOf(sa).Cond
	valid, single := cp.TargetValidTargeting, cp.TargetsSingleTarget
	if valid == "" && !single {
		return true
	}
	matched := false
	distinct := []state.Target(nil)
	for link := subject; link != nil; link = link.Sub {
		for _, t := range declared(link) {
			if !targetIn(distinct, t) {
				distinct = append(distinct, t)
			}
			if valid != "" && !matched && changeTargetSpecMatches(h, c, valid, t) {
				matched = true
			}
		}
	}
	if valid != "" && !matched {
		return false
	}
	if single && len(distinct) != 1 {
		return false
	}
	return true
}

// changeTargetSpecMatches is one ConditionTargetValidTargeting$ value against
// one declared target: the value's comma-split specs are ANY-matched (Forge's
// GameObject.isValid on the split array), an object target through the object
// filter and a player target through the player spec.
func changeTargetSpecMatches(h Host, c *Ctx, spec string, t state.Target) bool {
	for _, s := range strings.Split(spec, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if t.IsPlayer {
			if MatchesPlayerSpec(h.Game(), s, t.Player, c.Controller) {
				return true
			}
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		if MatchesObjectCtx(h.Game(), s, o, c.SpecContext(c.Controller)) {
			return true
		}
	}
	return false
}
