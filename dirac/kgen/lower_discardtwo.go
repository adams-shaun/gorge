package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// altDiscardTwo lowers the second Discard<N/Spec> part of a spell's own
// AlternativeCost (Foil: "discard an Island card and another card",
// Discard<1/Island> Discard<1/Card.Other>; CR 118.3, 601.2h). gorge's
// discardAsk asks the parts in order, each over pay.DiscardCandidates with
// the earlier parts' picks reserved and the cast card excluded, so the
// second part's Other clause names nothing the exclusion does not already
// drop: Card.Other is any card. The first part is a counted discard (any
// card or a filter); a third part, a self or whole-hand part, and the
// unless-cost selection specs stay refused.
func (lc *lowerCtx) altDiscardTwo(c *lCost, parts []string, n int, t string) error {
	if c.Discard2N > 0 || c.DiscardSelf || c.DiscardHand || c.DiscardSpec != "" || c.DiscardN < 1 ||
		len(parts) < 2 || len(parts) > 3 || n < 1 {
		return refuse("discard cost %q: more than one discard part", t)
	}
	spec := strings.TrimSuffix(parts[1], ".Other")
	switch spec {
	case "Card":
	case "CARDNAME", "NICKNAME", "Random", "Hand", "LastDrawn":
		return refuse("discard cost %q: more than one discard part", t)
	default:
		f, err := lc.filter(strings.ReplaceAll(spec, ";", ","))
		if err != nil {
			return err
		}
		c.Discard2F = f
	}
	c.Discard2N = n
	return nil
}

// filteredDiscardHome: a filtered Discard<N/Spec> part lowers in a spell's
// own AlternativeCost (Abolish's Discard<1/Plains>, Foil's first part) or in
// an instant's or sorcery's additional cost (Magmatic Insight's
// Discard<1/Land>), where the cast card itself is never a candidate (gorge
// pay.DiscardCandidates' casting flag; the kernel's costCands skips the
// entry's own card). The second part of a spell cost stays alternative-only.
func (lc *lowerCtx) filteredDiscardHome(c lCost) bool {
	if lc.altCostHand {
		return true
	}
	return lc.lifeXSpell && c.Discard2N == 0 && lc.out.Types&(tInstant|tSorcery) != 0
}

// altDiscardShape: the built alternative cost is one discard part, or two,
// and nothing else (Abolish, Foil; kernel dirac.altDiscardCost): the
// CostDiscard part, filtered or not, then Cost.Discard2N cards when set.
func altDiscardShape(c *kop.Cost) bool {
	return c.Flags&^kop.CostDiscardFiltered == kop.CostDiscard && c.N > 0 &&
		!c.DiscardHand && c.Mana == (kop.ManaCost{})
}
