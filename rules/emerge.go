// emerge.go implements the Emerge alternative cast (CR 702.118a): "You may
// cast this spell by sacrificing a creature and paying the emerge cost reduced
// by that creature's mana value."
//
// Emerge is a CASTING OPTION, read directly off the K: line exactly like the
// Flash/Flashback/Bestow/Mutate/MayFlashCost family (cards/keywords.go's
// expandKeywords doc names it), so it is registered here in its own file as
// those keywords are. It is NOT a plain alternative-cost substitution: the
// cast must sacrifice a chosen creature AND reduce the emerge cost by that
// creature's mana value, so it cannot ride keywordAltCost's
// "pay the printed parameter in place of the mana cost" path. The pieces:
//
//   - emergeCost resolves the printed K:Emerge parameter to a Cost, cutting
//     Forge's colon-suffixed metadata (`K:Emerge:5 B B:Artifact`) and
//     withholding any token ParseCost cannot model or an {X}, the
//     bestowCost/mayflashExtraCost fail-closed convention.
//   - emergeOfferCost prices the offer: the emerge cost composed with the
//     mandatory `Sac<1/Creature>` additional cost, reduced by the LARGEST
//     mana value among the caster's sacrificeable creatures. That is the
//     best case the player can reach, so the offer is present whenever some
//     legal sacrifice makes the cast payable; the actual chosen creature is
//     settled by the ordinary Sac machinery and the reduction is re-applied
//     to the cost the player actually chose (applyEmergeReduction).
//   - the offer (rules/legal.go's hand walk) adds the "emerged" cast mode,
//     and beginCast's "emerged" arm charges the emerge cost plus the Sac part.
//   - sacAsk applies the chosen creature's mana value as a reduction exactly
//     once, after every Sac part is settled, before the mana window reads
//     pc.cost.
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	// The coverage census: kw:Emerge is implemented as a casting option read
	// directly off the K: line (see this file's doc), registered here exactly
	// as bestow.go and mayflash.go register theirs.
	effects.RegisterNonAPI("kw:Emerge")
}

// emergeCost resolves the printed K:Emerge parameter to the base emerge cost.
// Forge's parameter may carry a colon-suffixed metadata tail
// (`K:Emerge:5 B B:Artifact`), the bestow/mutate colon shape, so everything
// from the first colon is dropped before parsing. An absent keyword, a token
// ParseCost reports as Unknown, and any {X} all withhold (ok=false): each
// would otherwise charge a cost that is not the card's, and a withheld emerge
// simply leaves the plain cast offered.
func emergeCost(f *cards.Face) (Cost, bool) {
	if f == nil {
		return Cost{}, false
	}
	s, ok := f.KeywordParam("Emerge")
	if !ok {
		return Cost{}, false
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return Cost{}, false
	}
	c := ParseCost(s)
	if len(c.Unknown) > 0 || c.X > 0 {
		return Cost{}, false
	}
	return c, true
}

// emergeSacrificePart is the mandatory additional cost every emerge cast pays:
// sacrifice one creature (CR 702.118a). It is a CostPart so the ordinary Sac
// machinery (sacAsk, nonManaCastable, sacrificeCostCandidates) enforces it
// without a parallel path.
func emergeSacrificePart() CostPart {
	return CostPart{N: 1, Spec: "Creature"}
}

// emergeOfferCost prices the emerge offer for id: the emerge cost composed
// with the mandatory creature sacrifice, reduced by the largest mana value
// among the caster's sacrificeable creatures. ok=false when the face prints
// no priced emerge cost or no creature can pay the sacrifice; the caller then
// withholds the option rather than offering an incorrect payment.
func (e *Engine) emergeOfferCost(p state.PlayerID, id state.ObjID, f *cards.Face) (Cost, bool) {
	base, ok := emergeCost(f)
	if !ok {
		return Cost{}, false
	}
	part := emergeSacrificePart()
	candidates := e.sacrificeCostCandidates(p, id, part, false)
	if len(candidates) == 0 {
		return Cost{}, false
	}
	best := int32(-1)
	for _, oid := range candidates {
		o := e.G.Obj(oid)
		if o == nil || o.Face() == nil {
			continue
		}
		if mv := o.Face().ManaValue(); mv > best {
			best = mv
		}
	}
	if best < 0 {
		return Cost{}, false
	}
	base = reduceGeneric(base, best)
	base.Sac = append(base.Sac, part)
	return base, true
}

// applyEmergeReduction folds the actual chosen sacrifice's mana value into the
// emerge cast's cost exactly once. Called from sacAsk after every Sac part has
// been settled and before the mana window reads pc.cost, so the reduction is
// part of the cost the payment charges while still composing under the CR
// 601.2f modifiers manaToPay applies afterwards. Idempotent through emergeDone
// so the continueCast re-entries a suspended mana window makes cannot subtract
// twice.
func (e *Engine) applyEmergeReduction(pc *pendingCast) {
	if !pc.emerge || pc.emergeDone {
		return
	}
	n := int32(0)
	for _, oid := range pc.sacs {
		o := e.G.Obj(oid)
		if o == nil || o.Face() == nil {
			continue
		}
		n += o.Face().ManaValue()
	}
	pc.cost = reduceGeneric(pc.cost, n)
	pc.emergeDone = true
}

// reduceGeneric subtracts n mana from c's GENERIC component only, floored at
// zero. A mana-value reduction (CR 702.118a's Emerge, like CR 118.7's
// cost-reduction effects generally) can only reduce the generic amount of a
// cost: the coloured pips are requirements the spell still carries, so
// {5}{U}{U} reduced by mana value 10 is {U}{U}, never zero. Coloured pips,
// hybrid and Phyrexian pips and every non-mana part are left untouched (the
// measured corpus's 15 K:Emerge lines are all plain generic+coloured, and a
// hybrid/Phyrexian face is not reducible generic either).
func reduceGeneric(c Cost, n int32) Cost {
	if n <= 0 {
		return c
	}
	if c.Generic < n {
		c.Generic = 0
		return c
	}
	c.Generic -= n
	return c
}
