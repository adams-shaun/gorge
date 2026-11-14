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

// altDiscardTwoShape: the built alternative cost is two discard parts and
// nothing else (Foil; kernel dirac.altDiscardTwoCost): the CostDiscard part,
// filtered or not, then Cost.Discard2N cards.
func altDiscardTwoShape(c *kop.Cost) bool {
	return c.Flags&^kop.CostDiscardFiltered == kop.CostDiscard && c.N > 0 && c.Discard2N > 0 &&
		!c.DiscardHand && c.Mana == (kop.ManaCost{})
}
