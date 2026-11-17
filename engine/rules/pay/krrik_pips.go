package pay

import "github.com/adams-shaun/gorge/engine/state"

// AnnounceGrantedBlackPips is the cost with every plain {B} pip turned into
// an announceable {B/P} pip. A PayLifeInsteadOf:B grant (K'rrik, Son of
// Yawgmoth) lets the payer pay 2 life rather than the mana for each {B} in a
// cost, which CR 601.2h makes the payer's choice per pip -- the same shape as
// a Phyrexian pip, so ManaAsk's one-decision-per-pip announcement carries it
// with option 0 the mana payment. Without the conversion the grant only
// settles inside the automatic resolver, which pays life when mana is short
// and never offers it otherwise.
func AnnounceGrantedBlackPips(c Cost) Cost {
	n := c.Colored[state.ManaIndex('B')]
	if n <= 0 {
		return c
	}
	pips := make([]byte, 0, len(c.Phyrexian)+int(n))
	pips = append(pips, c.Phyrexian...)
	for i := int32(0); i < n; i++ {
		pips = append(pips, 'B')
	}
	c.Phyrexian = pips
	c.Colored[state.ManaIndex('B')] = 0
	return c
}
