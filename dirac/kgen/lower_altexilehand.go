package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// altPayLifeExileHand: the lowered cost of a spell's own AlternativeCost is
// PayLife<N> and ExileFromHand<N/Spec> and nothing else (Force of Will: "You
// may pay 1 life and exile a blue card from your hand rather than pay this
// spell's mana cost", CR 118.9, 601.2b), or the exile alone (Misdirection and
// the Force cycle's bare ExileFromHand<1/Spec>, PayLife 0; Commandeer's bare
// ExileFromHand<2/Card.Blue+Other>, PayLife 0). The two parts
// share kop.Cost.N, so the life equals the exiled count; build_ab's
// AlternativeCost arm gates the
// card (an instant or sorcery with one spell ability, cardBuilder.altSacOK).
func altPayLifeExileHand(c *lCost) bool {
	one := c.ExileGN == 1
	twoBare := c.ExileGN == 2 && c.PayLife == 0 // Commandeer: two cards, no life beside
	if c.ExileG == nil || !c.ExileHand || !one && !twoBare || c.PayLife != c.ExileGN && c.PayLife != 0 || c.ExileGBf || c.exileGCard {
		return false
	}
	rest := *c
	rest.ExileG, rest.ExileGN, rest.ExileHand, rest.PayLife = nil, 0, false, 0
	return rest.pureMana()
}

// altCostCost lowers a spell's own AlternativeCost static's Cost$ with
// lc.altCostHand set, so an ExileFromHand<1/Spec> part keeps its filter
// (altExileHandCost). ExileFromHand<N/Spec>, with PayLife<N> beside it
// or not (Force of Will, Misdirection and the Force cycle), is the kernel's
// hand-exile alternative cast; ExileFromGrave<N/Spec> beside its mana (Squee,
// Dubious Monarch's {3}{R} and four other graveyard cards, CR 118.9) is the
// kernel's grave-exile one (dirac altGraveExileCost); any other exile shape
// is refused.
func (lc *lowerCtx) altCostCost(s string) (lCost, error) {
	lc.altCostHand = true
	c, err := lc.cost(s)
	lc.altCostHand = false
	if err != nil {
		return c, err
	}
	if c.ExileG != nil && !altPayLifeExileHand(&c) && !altGraveExileCost(&c) {
		return c, refuse("alternative cost with an exile part other than ExileFromHand<1/Spec>, alone or beside PayLife<N>, or ExileFromGrave<N/Spec> beside its mana")
	}
	return c, nil
}

// altGraveExileCost: the lowered alternative cost is exiling N graveyard
// cards matching its filter beside its mana and nothing else (Squee's
// ExileFromGrave<4/Card.Other/other cards>; kernel dirac altGraveExileCost):
// the exile count rides c.ExileGN (kop.Cost.N), the mana the rest carries.
func altGraveExileCost(c *lCost) bool {
	if c.ExileG == nil || c.ExileHand || c.ExileGBf || c.exileGCard || c.ExileGN == 0 {
		return false
	}
	rest := *c
	rest.ExileG, rest.ExileGN = nil, 0
	return rest.pureMana()
}

// altExileHandCost lowers an alternative cost's ExileFromHand<n/Spec[/desc]>
// part (Force of Will's "exile a blue card from your hand"; CR 118.9,
// 601.2b): the N cards matching the spec, picked as the cast pays (the
// kernel's hand-exile pick walks Cost.N; Commandeer's <2/...> pays two).
// While lc.announceAlt (the static's Announce$ X, static_cost.go), it is the
// announced X-bound one only: <1/Spec> whose spec bounds the mana value to X
// with exactly one cmcEQX clause (the Shoal cycle; announceHandExile).
func (lc *lowerCtx) altExileHandCost(c *lCost, parts []string, n int, t string) error {
	if len(parts) < 2 || len(parts) > 3 || n != 1 && n != 2 || c.ExileG != nil || c.ExileHandN != 0 {
		return refuse("exile cost %q", t)
	}
	f, err := lc.filter(parts[1])
	if err != nil {
		return err
	}
	if lc.announceAlt {
		if n != 1 || !announceXBound(f) {
			return refuse("announced exile cost %q: only ExileFromHand<1/Spec> whose spec bounds the exiled card's mana value to X (one cmcEQX clause) is announced", t)
		}
		c.AnnounceX = true
	}
	c.ExileG, c.ExileGN, c.ExileHand = f, n, true
	return nil
}

// announceXBound reports whether f is the announce-bearing exile spec: one
// clause bounding the card's mana value to the SVar X (the Shoal cycle's
// Card.Red+Other+cmcEQX).
func announceXBound(f *lFilter) bool {
	if f == nil || len(f.Clauses) != 1 {
		return false
	}
	c := f.Clauses[0]
	return c.CmpStat == cmpCMC && c.CmpOp == opEQ && c.CmpX == "X"
}

// altLifeShared: while an alternative cost lowers, its PayLife<N> beside
// ExileFromHand<N/Spec> shares the exile's count (kop.Cost.N), so it is not
// a counted part of its own.
func (lc *lowerCtx) altLifeShared(c *lCost) bool {
	return lc.altCostHand && c.ExileHand && c.PayLife == c.ExileGN
}

// altHandExileShape: the built alternative cost is the hand-exile pick
// (kernel dirac.altHandExileCost) -- PayLife<N> beside the exile (Force of
// Will, Flags CostPayLife|CostExileGrave sharing Cost.N), the exile alone
// (the Force cycle, Misdirection; bare CostExileGrave), or the announced
// X-bound one (the Shoal cycle; announceHandExile).
func altHandExileShape(c *kop.Cost) bool {
	shared := c.Flags == kop.CostPayLife|kop.CostExileGrave
	bare := c.Flags == kop.CostExileGrave
	return (shared || bare) && c.ExileHand && (c.N > 0 || announceHandExile(c))
}

// announceHandExile: the built alternative cost is the announced X-bound
// hand exile (kernel dirac.announceHandXCost, cost_announcex.go): the exile
// alone with Cost.N 0, its filter's cmcEQX predicate bound to the announced
// X.
func announceHandExile(c *kop.Cost) bool {
	return c.Flags == kop.CostExileGrave && c.ExileHand && c.N == 0
}

// altHandExileOK: the hand-exile shape on a card whose one spell ability can
// carry it (altSacOK: an instant, sorcery, or a creature with no spell
// ability of its own).
func (cb *cardBuilder) altHandExileOK(c *kop.Cost) bool {
	return altHandExileShape(c) && cb.altSacOK(c)
}
