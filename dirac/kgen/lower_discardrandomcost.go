package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Discard<N/Random> as an activated ability's or a spell's cost (CR 701.9b,
// 118.3): the engine's seeded RNG picks each card when the cost objects are
// paid (gorge discardAsk's Random arm), so the kernel draws one pick per card
// and asks nothing (dirac cost_discardrandom.go). lCost keeps the count in
// DiscardN and the spec "Random", so every reader that admits only a plain
// discard (DiscardSpec == "", pureMana) still refuses it.

// discardRandomCost admits a Random discard cost of a countable size.
func discardRandomCost(c *lCost) error {
	if c.DiscardN < 1 || c.DiscardF != nil || c.DiscardSelf || c.Discard2N > 0 {
		return refuse("discard cost %q beside another discard part", "Random")
	}
	return nil
}

// discardRandomManaRefusal refuses a Random discard on a mana ability: gorge
// pays it in the mana window's own walk (pay/manacostwalk.go), which the
// kernel does not model.
func discardRandomManaRefusal(kind int, cost string) error {
	if kind == akMana && strings.Contains(cost, "/Random") && strings.Contains(cost, "Discard<") {
		return refuse("discard-random cost %q on a mana ability", cost)
	}
	return nil
}

// usageDiscardCosts records the whole-hand and random discard cost rows.
func usageDiscardCosts(c kop.Cost, name string, add func(label, name string)) {
	if c.DiscardHand {
		add("cost:DiscardHand", name)
	}
	if c.DiscardRandom {
		add("cost:DiscardRandom", name)
	}
}

// discardRandomPart reports a countable Random discard (kop.Cost.DiscardRandom
// beside CostDiscard).
func discardRandomPart(c lCost) bool {
	return c.DiscardN > 0 && c.DiscardSpec == "Random"
}
