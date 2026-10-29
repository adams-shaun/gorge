package rules

import (
	"cmp"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// ObservedAbilities returns id's non-mana activated abilities in the
// object-stable order v2's activate_ability.ability_index names them: the
// printed face's abilities in face order, then every granted ability ordered
// by the granting continuous effect's timestamp, then the granting object's
// id, then the position the grant was collected at. It lists what the object
// HAS, never what one decision OFFERS: no timing, activator, payability or
// target gate applies, and the per-turn GainsAbilitiesLimitPerTurn$ cap is a
// withholding gate rather than an end to the grant, so a spent cap still
// lists the ability. An ability keeps its index while its grant persists and
// a decision that withholds a sibling grant cannot renumber it. The offer
// walks remain the authority for which candidates a decision poses.
//
// Mana abilities are excluded with the offer walk's own split (a loyalty
// ability is never a mana ability, CR 605.1b). A mutated pile keeps its
// historical fail-closed refusal. A phased-out permanent has no abilities to
// observe, exactly as the offer walk treats it (CR 702.25b).
func ObservedAbilities(e *Engine, p state.PlayerID, id state.ObjID) []decision.Option {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || len(o.MergedCards) > 0 {
		return nil
	}
	if o.Zone == state.ZBattlefield && !pay.ExistsOnBattlefield(o) {
		return nil
	}
	f := o.Face()
	var out []decision.Option
	labelName := f.Name
	if o.Zone == state.ZBattlefield && e.faceDownPrintedHides(o) {
		labelName = faceDownGrantLabel
	}
	type grant struct {
		opt    decision.Option
		stamp  uint32
		source state.ObjID
		// pos is the position in this collected slice, not a per-grant
		// ordinal: keyword-granted lines are collected before the SVar/gained
		// grants, so a tie on (stamp, source) between those classes is broken
		// by collection order. The order is deterministic and stays fixed
		// while the grants persist, which is all the index rule needs.
		pos int
	}
	var grants []grant
	if !e.printedAbilitiesGone(o) {
		for i, ab := range f.Abilities {
			if ab == nil || ab.Kind != "AB" || (cards.IsManaAbilitySA(ab) && !e.isLoyaltyAbility(ab)) {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "ability",
				Label: labelName + ": " + ab.ParamStr(cards.PKSpellDescription), Obj: id, Ability: i})
		}
		// Keyword-granted abilities (CR 613.1f) a layer-6 AddKeyword$ grant
		// synthesizes from a derived line no printed face expands. This
		// mirrors the offer walk's granted-keyword block and works in any
		// zone the grant reaches (a hand card given cycling).
		for _, line := range e.grantedKeywordLines(id) {
			ab := cards.GrantedKeywordAbility(line)
			if ab == nil || (cards.IsManaAbilitySA(ab) && !e.isLoyaltyAbility(ab)) {
				continue
			}
			stamp, source := grantedKeywordStamp(e, o, line)
			grants = append(grants, grant{
				opt: decision.Option{Index: len(out), Kind: "ability",
					Label: labelName + ": " + ab.ParamStr(cards.PKSpellDescription),
					Obj:   id, Ability: -1, Keyword: line},
				stamp: stamp, source: source, pos: len(grants),
			})
		}
	}
	if o.Zone != state.ZBattlefield {
		// A non-battlefield object's keyword-granted lines still follow its
		// printed ordinals, in the same object-stable order.
		for _, g := range grants {
			g.opt.Index = len(out)
			out = append(out, g.opt)
		}
		return out
	}
	for _, ga := range e.grantedAbilitiesLimited(p, id, false) {
		ab := ga.SA
		if ab == nil || ab.Kind != "AB" || (cards.IsManaAbilitySA(ab) && !e.isLoyaltyAbility(ab)) {
			continue
		}
		opt := decision.Option{Index: len(out), Kind: "ability",
			Label: labelName + ": " + ab.ParamStr(cards.PKSpellDescription),
			Obj:   id, Ability: -1, Attach: ab.API == "Attach"}
		if ga.Gained {
			opt.GainedSource, opt.GainedIdx = ga.GainedFrom, ga.GainedIdx
		} else {
			opt.SVar, opt.GrantSource = ga.SVar, ga.Source
		}
		grants = append(grants, grant{opt: opt, stamp: ga.Stamp, source: ga.Source, pos: len(grants)})
	}
	// The order is the object's, not the collector's: active() sorts by CR
	// 613 layer and sublayer first, which for one object's grants is a tie,
	// then timestamp and registration order. Re-sort the two grant sources
	// together by the rule the protocol's u32 names.
	slices.SortStableFunc(grants, func(a, b grant) int {
		if c := cmp.Compare(a.stamp, b.stamp); c != 0 {
			return c
		}
		if c := cmp.Compare(a.source, b.source); c != 0 {
			return c
		}
		return cmp.Compare(a.pos, b.pos)
	})
	for _, g := range grants {
		g.opt.Index = len(out)
		out = append(out, g.opt)
	}
	return out
}

// grantedKeywordStamp finds the layer-6 effect that last added line to id's
// keyword list, returning its timestamp and source. The derived keyword walk
// (chars.Compute) applies removal and grant in dependency order but keeps no
// provenance, so this walks the same active list backwards for the last grant
// that could have left line in place. It is a pure read; a line the walk
// already proved present always has such a grant, and the zero fallback only
// keeps a malformed board deterministic.
func grantedKeywordStamp(e *Engine, o *state.Object, line string) (uint32, state.ObjID) {
	if o == nil {
		return 0, 0
	}
	lossStamp, lost := e.abilityLoss(o)
	ces := e.active()
	for i := len(ces) - 1; i >= 0; i-- {
		ce := &ces[i]
		if ce.Layer != state.LAbilities || !hasKeywordLine(ce.AddKeywords, line) {
			continue
		}
		if lost && lossStamp > ce.Timestamp {
			continue
		}
		if !e.matchesSpecFrom(ce.Affects, o.ID, ce.Controller, ce.Source) {
			continue
		}
		return ce.Timestamp, ce.Source
	}
	return 0, 0
}

// hasKeywordLine reports whether lines carries line. The derived keyword list
// appends the continuous effect's own AddKeywords entries verbatim, so this is
// an exact compare: two Cycling lines with different costs are different
// abilities, and no case folding is needed here.
func hasKeywordLine(lines []string, line string) bool {
	for _, k := range lines {
		if k == line {
			return true
		}
	}
	return false
}
