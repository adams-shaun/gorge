package combat

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// AttackRequirementClause is one active `S:Mode$ AttackRequirement` static
// (War's Toll: "If a creature an opponent controls attacks, all creatures that
// player controls attack if able"), resolved against the active player's
// battlefield: a declaration that includes any of Triggers must include every
// able one of Members. It is a condition on the WHOLE declaration, not a
// property of one creature, so it cannot ride Option.Required.
type AttackRequirementClause struct {
	Source   state.ObjID
	Triggers []state.ObjID // ValidAttacker$ matches
	Members  []state.ObjID // ValidCard$ matches
}

// AttackRequirementClauses lists every gated, readable AttackRequirement static
// as a clause over the active player's creatures, in Statics order and then
// battlefield order (deterministic, no map walk). ValidCard$ and ValidAttacker$
// default to every creature (an absent parameter matches everything). A clause
// with an empty side is inert and dropped. "Able" is not judged here: the
// caller narrows both sides to the creatures the decision actually offers.
func AttackRequirementClauses(b Board) []AttackRequirementClause {
	var out []AttackRequirementClause
	for _, sv := range b.Statics("AttackRequirement") {
		if !effects.AttackRequirementParamsReadable(sv.Params) || !b.StaticGateHolds(sv) {
			continue
		}
		members, triggers := sv.ParamStr(cards.PKValidCard), sv.ParamStr(cards.PKValidAttacker)
		if members == "" {
			members = "Card"
		}
		if triggers == "" {
			triggers = "Card"
		}
		c := AttackRequirementClause{Source: sv.Source}
		for _, id := range b.Game().Zone(state.ZBattlefield, b.Game().Active) {
			if b.MatchesStaticSpec(members, id, sv) {
				c.Members = append(c.Members, id)
			}
			if b.MatchesStaticSpec(triggers, id, sv) {
				c.Triggers = append(c.Triggers, id)
			}
		}
		if len(c.Members) > 0 && len(c.Triggers) > 0 {
			out = append(out, c)
		}
	}
	return out
}
