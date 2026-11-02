package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Monocoloured hybrid mana ({2/W}, CR 107.4e). Only a single-face
// card's printed mana cost reads it (parsePrintedManaCost); a keyword,
// activation, kicker or face cost keeps refusing the symbol. The kernel
// holds the pips in kop.ManaCost.Tb and announces each one's face.

// encodeTwobrid fills out's twobrid pips from m.
func encodeTwobrid(m manaCost, out *kop.ManaCost) error {
	if len(m.Hybrid) != 0 {
		return fmt.Errorf("twobrid beside hybrid")
	}
	if m.X || m.XX {
		return fmt.Errorf("twobrid with {X}")
	}
	pips := m.Twobrid
	if len(m.Phyrexian) != 0 {
		// Phyrexian pips share the twobrid encoding, Hyb[1] marking their
		// second face as two life rather than two generic
		if len(m.Twobrid) != 0 {
			return fmt.Errorf("Phyrexian beside twobrid")
		}
		pips = m.Phyrexian
		out.Hyb[1] = kop.HybPhyrexian
	}
	if len(pips) > 5 {
		return fmt.Errorf("twobrid pips %d", len(pips))
	}
	for i, c := range pips {
		out.Tb |= uint16(strings.IndexByte("WUBRG", c)+1) << (3 * uint(i))
	}
	out.Hyb[0] = kop.HybTwobrid
	if out.Hyb[2] != 0 || out.Hyb[3] != 0 {
		return fmt.Errorf("twobrid marker beside hybrid pips") // the markers own Hyb[0..1] alone
	}
	return nil
}

// twobridKw are the keywords that price or pay a cast another way.
var twobridKw = map[string]bool{
	"Delve": true, "Affinity": true, "Convoke": true, "Improvise": true, "Buyback": true,
	"Entwine": true, "Replicate": true, "Cumulative upkeep": true,
}

// twobridConflict refuses a card whose twobrid cost meets an alternative or
// additional cost, or a second face: the kernel composes none of them with
// the pips.
func twobridConflict(c *lCard) error {
	if len(c.Faces) != 0 || c.Face != 0 || c.FaceMode != 0 {
		return fmt.Errorf("twobrid on a multi-face card")
	}
	if c.Kicker != nil || c.Flashback != nil || c.Warp != nil || c.Plot != nil || c.Sneak != nil ||
		c.MorphCost != nil || c.Offspring != nil || c.Casualty != 0 || c.Convoke || c.Improvise || c.Harmonize != nil || c.Mayhem != nil ||
		c.AltAddCost != nil || c.SpellCost != nil || c.Gift {
		return fmt.Errorf("twobrid beside an alternative or additional cost")
	}
	for _, k := range c.Kw {
		head, _, _ := strings.Cut(k, ":")
		if twobridKw[head] {
			return fmt.Errorf("twobrid beside an alternative or additional cost")
		}
	}
	return nil
}
