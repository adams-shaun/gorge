package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// ObservedAbilities returns id's non-mana activated abilities in the order the
// seat observation publishes them: the printed face's abilities in face order,
// then the derived keyword-granted lines in grantedKeywordLines order, then the
// AddAbilities$/GainsAbilitiesOf$ grants in grantedAbilities order. It is the
// one order both the observation and mapping read; do not sort or range a map.
//
// It lists what the object HAS, never what one decision offers: no offer gate
// (sorcery timing, activator, payability, targets, per-turn caps) applies, so
// the list is stable across decisions and a v2 client can name a granted body
// by its position. The offer walk remains the authority for candidates.
func (e *Engine) ObservedAbilities(p state.PlayerID, id state.ObjID) []decision.Option {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || len(o.MergedCards) > 0 {
		// A mutated pile's activation keeps its existing fail-closed refusal
		// (mapping.NonManaAbilityIndex's ability/mutated_pile path).
		return nil
	}
	if o.Zone == state.ZBattlefield && !pay.ExistsOnBattlefield(o) {
		// CR 702.25b: a phased-out permanent is treated as though it does not
		// exist, exactly as the offer walk treats it.
		return nil
	}
	f := o.Face()
	var out []decision.Option
	if !e.printedAbilitiesGone(o) {
		// The offer walk's exact non-mana split (legal_walk_battlefield.go):
		// an AB the engine would offer as an "ability" option is a non-mana
		// activated ability; a loyalty ability is never a mana ability.
		for i, ab := range f.Abilities {
			if ab == nil || ab.Kind != "AB" || (cards.IsManaAbilitySA(ab) && !e.isLoyaltyAbility(ab)) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "ability",
				Label: f.Name + ": " + ab.ParamStr(cards.PKSpellDescription), Obj: id, Ability: i})
		}
		// Keyword-granted abilities (CR 613.1f) a layer-6 AddKeyword$ grant
		// synthesizes from a derived line no printed face expands. This
		// mirrors the offer walk's granted-keyword block.
		for _, line := range e.grantedKeywordLines(id) {
			ab := cards.GrantedKeywordAbility(line)
			if ab == nil {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "ability",
				Label: f.Name + ": " + ab.ParamStr(cards.PKSpellDescription), Obj: id, Ability: -1, Keyword: line})
		}
	}
	if o.Zone != state.ZBattlefield {
		return out
	}
	// Granted and gained activated abilities (CR 613.1f), in the collector's
	// order: the same list the offer walk reads.
	labelName := f.Name
	if e.faceDownPrintedHides(o) {
		labelName = faceDownGrantLabel
	}
	for _, ga := range e.grantedAbilities(p, id) {
		ab := ga.SA
		if ab == nil || ab.Kind != "AB" || (cards.IsManaAbilitySA(ab) && !e.isLoyaltyAbility(ab)) {
			continue
		}
		opt := decision.Option{Index: len(out), Kind: "ability",
			Label: labelName + ": " + ab.ParamStr(cards.PKSpellDescription), Obj: id, Ability: -1,
			Attach: ab.API == "Attach"}
		if ga.Gained {
			opt.GainedSource, opt.GainedIdx = ga.GainedFrom, ga.GainedIdx
		} else {
			opt.SVar, opt.GrantSource = ga.SVar, ga.Source
		}
		out = append(out, opt)
	}
	return out
}
