package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// altPayLifeExileHand: the lowered cost of a spell's own AlternativeCost is
// PayLife<N> and ExileFromHand<N/Spec> and nothing else (Force of Will: "You
// may pay 1 life and exile a blue card from your hand rather than pay this
// spell's mana cost", CR 118.9, 601.2b). The two parts share kop.Cost.N, so
// the life equals the exiled count; build_ab's AlternativeCost arm gates the
// card (an instant or sorcery with one spell ability, cardBuilder.altSacOK).
func altPayLifeExileHand(c *lCost) bool {
	if c.ExileG == nil || !c.ExileHand || c.ExileGN != 1 || c.PayLife != c.ExileGN || c.ExileGBf || c.exileGCard {
		return false
	}
	rest := *c
	rest.ExileG, rest.ExileGN, rest.ExileHand, rest.PayLife = nil, 0, false, 0
	return rest.pureMana()
}

// altExileHandOnly: the alternative cost is ExileFromHand<1/Spec> alone, no
// life beside it (the Force cycle's "you may exile a blue card from your
// hand rather than pay this spell's mana cost", CR 118.9; Misdirection the
// same shape). The built Cost is the bare hand-exile pick, CostExileGrave
// with ExileHand, no PayLife flag.
func altExileHandOnly(c *lCost) bool {
	if c.ExileG == nil || !c.ExileHand || c.ExileGN != 1 || c.PayLife != 0 || c.ExileGBf || c.exileGCard {
		return false
	}
	rest := *c
	rest.ExileG, rest.ExileGN, rest.ExileHand = nil, 0, false
	return rest.pureMana()
}

// altCostCost lowers a spell's own AlternativeCost static's Cost$ with
// lc.altCostHand set, so an ExileFromHand<1/Spec> part keeps its filter
// (altExileHandCost). PayLife<N> beside ExileFromHand<N/Spec> (Force of
// Will) and the exile alone (the Force cycle, Misdirection) are the
// kernel's hand-exile alternative casts; any other exile shape is refused.
func (lc *lowerCtx) altCostCost(s string) (lCost, error) {
	lc.altCostHand = true
	c, err := lc.cost(s)
	lc.altCostHand = false
	if err != nil {
		return c, err
	}
	if c.ExileG != nil && !altPayLifeExileHand(&c) && !altExileHandOnly(&c) {
		return c, refuse("alternative cost with an exile part other than PayLife<N> or ExileFromHand<1/Spec> alone")
	}
	return c, nil
}

// altExileHandCost lowers an alternative cost's ExileFromHand<n/Spec[/desc]>
// part (Force of Will's "exile a blue card from your hand"; CR 118.9,
// 601.2b): one card matching the spec, picked as the cast pays.
func (lc *lowerCtx) altExileHandCost(c *lCost, parts []string, n int, t string) error {
	if len(parts) < 2 || len(parts) > 3 || n != 1 || c.ExileG != nil || c.ExileHandN != 0 {
		return refuse("exile cost %q", t)
	}
	f, err := lc.filter(parts[1])
	if err != nil {
		return err
	}
	c.ExileG, c.ExileGN, c.ExileHand = f, n, true
	return nil
}

// altLifeShared: while an alternative cost lowers, its PayLife<N> beside
// ExileFromHand<N/Spec> shares the exile's count (kop.Cost.N), so it is not
// a counted part of its own.
func (lc *lowerCtx) altLifeShared(c *lCost) bool {
	return lc.altCostHand && c.ExileHand && c.PayLife == c.ExileGN
}

// altHandExileShape: the built alternative cost is the hand-exile pick
// (kernel dirac.altHandExileCost) -- PayLife<N> beside the exile (Force of
// Will, Flags CostPayLife|CostExileGrave sharing Cost.N) or the exile alone
// (the Force cycle, Misdirection; bare CostExileGrave).
func altHandExileShape(c *kop.Cost) bool {
	shared := c.Flags == kop.CostPayLife|kop.CostExileGrave
	bare := c.Flags == kop.CostExileGrave
	return (shared || bare) && c.ExileHand && c.N > 0 && c.Mana == (kop.ManaCost{})
}

// altHandExileOK: the hand-exile shape on a card whose one spell ability can
// carry it (altSacOK: an instant, sorcery, or a creature with no spell
// ability of its own).
func (cb *cardBuilder) altHandExileOK(c *kop.Cost) bool {
	return altHandExileShape(c) && cb.altSacOK()
}
