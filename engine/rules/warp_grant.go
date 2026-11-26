package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// altRowReader is the narrow read the alternative-cost row readers need (the
// Engine implements it): the object table, the derived-keyword precheck and
// parameter read, and the printed face's compiled facts.
type altRowReader interface {
	Game() *state.Game
	mayHaveDerivedKeywordH(state.ObjID, kwHead) bool
	derivedWith(state.ObjID, state.Zone) Derived
	walkFaceFactsOf(*cards.Face) *walkFaceFacts
}

// grantedWarpCosts are the Warp costs the derived keywords give a card in the
// hand (Tannuk, Steadfast Second: "Artifact cards and red creature cards in
// your hand have warp {2}{R}", CR 702.185a), in keyword order. Every Warp
// instance is read, not the first: a card that prints Warp and is granted
// another carries both. The read is at the card's current zone; a card outside
// the hand has none (a grant never opens warp from another zone).
func grantedWarpCosts(e altRowReader, id state.ObjID) []Cost {
	o := e.Game().Obj(id)
	if o == nil || o.Zone != state.ZHand || !e.mayHaveDerivedKeywordH(id, kwhWarp) {
		return nil
	}
	var out []Cost
	for _, k := range e.derivedWith(id, 0).Keywords {
		head := cardsKeywordHead(k)
		if head != kwhWarp.S || len(k) <= len(head)+1 {
			continue
		}
		if c := ParseCost(k[len(head)+1:]); len(c.Unknown) == 0 {
			out = append(out, c)
		}
	}
	return out
}

// grantedMadnessCosts are the Madness costs the DERIVED keyword list gives
// card id, read in its live zone: a printed K:Madness line is read by the
// row's compiled face cost, so this walk runs only for a face that prints
// none and prices the grant a continuous static delivered (Falkenrath
// Gorger's AddKeyword$ Madness:ManaCost, "Each Vampire creature card you own
// that isn't on the battlefield has madness. The madness cost is equal to
// its mana cost"). The grant's AffectedZone$ scope covers every zone the
// mechanic travels, so the live-zone read is the zone-pinned one: the
// discard replacement offers from the hand, the ask prices from exile, the
// cast is priced on the stack (CR 702.35a-b).
func grantedMadnessCosts(e altRowReader, id state.ObjID, f *cards.Face) []Cost {
	if !e.mayHaveDerivedKeywordH(id, kwhMadness) {
		return nil
	}
	var out []Cost
	for _, k := range e.derivedWith(id, 0).Keywords {
		if !strings.EqualFold(cardsKeywordHead(k), kwhMadness.S) {
			continue
		}
		raw := ""
		if i := strings.IndexByte(k, ':'); i >= 0 {
			raw = strings.TrimSpace(k[i+1:])
		}
		if c, ok := madnessLineCost(raw, f); ok {
			out = append(out, c)
		}
	}
	return out
}

// madnessCostLabel is the raw cost string a madness ask's option label
// prints for a card whose Madness is granted rather than printed: the first
// derived Madness line's parameter, with its ManaCost/CardManaCost
// placeholders expanded to the face's mana cost -- the same expansion
// madnessLineCost prices, so the label and the charged cost cannot drift.
func madnessCostLabel(e altRowReader, id state.ObjID, f *cards.Face) string {
	if !e.mayHaveDerivedKeywordH(id, kwhMadness) {
		return ""
	}
	for _, k := range e.derivedWith(id, 0).Keywords {
		if !strings.EqualFold(cardsKeywordHead(k), kwhMadness.S) {
			continue
		}
		if i := strings.IndexByte(k, ':'); i >= 0 {
			return madnessCostParam(k[i+1:], f)
		}
		return ""
	}
	return ""
}

// madnessLineCost prices one Madness parameter: the ManaCost placeholder
// (Falkenrath Gorger's AddKeyword$ Madness:ManaCost spelling, and its
// CardManaCost alias) is the face's own mana cost ("the madness cost is
// equal to its mana cost"); a parameterless instance (a grant with no cost
// of its own) prices none; a cost ParseCost cannot price is withheld rather
// than charged wrong (the escapeCost convention).
func madnessLineCost(raw string, f *cards.Face) (Cost, bool) {
	if strings.TrimSpace(raw) == "" {
		return Cost{}, false
	}
	c := ParseCost(madnessCostParam(raw, f))
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// madnessCardCostTokens are the Forge placeholder spellings that stand for
// the card's own mana cost in a Madness cost parameter (Falkenrath Gorger's
// AddKeyword$ Madness:ManaCost, and Norman Osborn's Mayhem:CardManaCost
// alias).
var madnessCardCostTokens = [...]string{"ManaCost", "CardManaCost"}

// madnessCostParam expands one Madness parameter's cost tokens, for parsing
// and for the ask's option label: the ManaCost/CardManaCost placeholders
// become the face's own mana cost.
func madnessCostParam(raw string, f *cards.Face) string {
	var toks []string
	for tok := range strings.FieldsSeq(raw) {
		if madnessCardCostToken(tok) {
			toks = append(toks, strings.Fields(f.ManaCost)...)
			continue
		}
		toks = append(toks, tok)
	}
	return strings.Join(toks, " ")
}

// madnessCardCostToken reports whether tok is one of madnessCardCostTokens.
func madnessCardCostToken(tok string) bool {
	for _, t := range madnessCardCostTokens {
		if strings.EqualFold(tok, t) {
			return true
		}
	}
	return false
}

// altRowCosts are row m's costs for card id on face f, in offer order: the
// printed keyword's cost (the shared compiled read), and for the Warp row each
// hand grant's after it. CR 702.185a: each warp instance is its own
// alternative cost and the caster picks one, so each surviving instance is its
// own row, addressed by the option's AltCostIndex. An instance another one
// dominates (warpCostCovers) is dropped, the earlier of two equal ones kept:
// any payment for it pays the other, and both yield the same warped cast.
// Incomparable instances ({4} and {2}{R}) both stay.
func altRowCosts(e altRowReader, m *altCastMode, id state.ObjID, f *cards.Face) []Cost {
	var all []Cost
	printed, ok := m.faceCost(f, e.walkFaceFactsOf(f))
	if ok {
		all = append(all, printed)
	}
	if m.id == altMadness && !ok {
		// A face that prints no K:Madness line still carries the keyword a
		// continuous-effect grant delivered (Falkenrath Gorger): the derived
		// read is what the compiled face cost is for a printed carrier. Only
		// the unprinted case is walked, so the row stays exactly one
		// instance -- the family addresses alternatives by AltCostIndex for
		// Warp alone, and no corpus carrier prints Madness beside the grant.
		all = append(all, grantedMadnessCosts(e, id, f)...)
	}
	if m.id != altWarp {
		return all
	}
	all = append(all, grantedWarpCosts(e, id)...)
	if len(all) < 2 {
		return all
	}
	out := all[:0:0]
	for i := range all {
		dropped := false
		for j := range all {
			if j != i && warpCostCovers(all[j], all[i]) && (j < i || !warpCostCovers(all[i], all[j])) {
				dropped = true
				break
			}
		}
		if !dropped {
			out = append(out, all[i])
		}
	}
	return out
}

// altRowCost is instance idx of row m's costs (altRowCosts), the ONE read the
// hand offer, the cast's charge and the base-cost reader share, so the three
// cannot drift. An out-of-range idx (a stale option) has no cost.
func altRowCost(e altRowReader, m *altCastMode, id state.ObjID, f *cards.Face, idx int) (Cost, bool) {
	costs := altRowCosts(e, m, id, f)
	if idx < 0 || idx >= len(costs) {
		return Cost{}, false
	}
	return costs[idx], true
}

// warpCostCovers reports whether a dominates b: both are plain mana (coloured
// pips and generic only) and a asks no more of any mana type and no more in
// total, so every payment of b also pays a. Equal costs cover each other; a
// cost with any other part neither dominates nor is dominated otherwise.
func warpCostCovers(a, b Cost) bool {
	if a.Equal(&b) {
		return true
	}
	if !plainManaCost(a) || !plainManaCost(b) || a.Colored.Total()+a.Generic > b.Colored.Total()+b.Generic {
		return false
	}
	for i := range a.Colored {
		if a.Colored[i] > b.Colored[i] {
			return false
		}
	}
	return true
}

// plainManaCost is a cost of coloured pips and generic mana only.
func plainManaCost(c Cost) bool {
	plain := Cost{Colored: c.Colored, Generic: c.Generic}
	return c.Equal(&plain)
}
