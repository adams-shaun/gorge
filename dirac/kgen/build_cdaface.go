package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// cdaFace fills d.CDAFace from a CDA-folded card's own characteristics
// (lCard.CDAFace); its subtypes ride cb.out.printed into PrintedSubtypes
// (pool indexes, renumbered per matchup). A card with no fold is untouched.
func (cb *cardBuilder) cdaFace(d *kop.CardDef, c *lCard) error {
	f := c.CDAFace
	if f == nil {
		return nil
	}
	subs, err := cb.subIndexes(f.Subtypes)
	if err != nil {
		return err
	}
	cb.out.printed = subs
	d.CDAFace = kop.CDAFace{Folded: true, Types: typeMask(f.Types) | subtypeTypeBits(f.Subtypes),
		Colors: colorMask(f.Colors), AllCreatureTypes: f.Changeling}
	if c.Token {
		d.CDAFace.Types |= kop.TToken
	}
	return nil
}
