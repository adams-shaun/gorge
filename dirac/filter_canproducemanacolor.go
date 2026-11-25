package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwCanProduceManaColor: FwCanProduceManaColor, the candidate battlefield
// permanent's printed mana abilities can produce a mana type in the colour
// mask l.Arg (gorge's canProduceManaColor <Colour>, engine/effects/filter.go;
// Forge's AbilityManaPart.canProduce, where "Any" never means colourless).
// The producible set is cardProducible, the read Mana Web's
// FwCanProduceSameManaTypeWith already makes.
//
//kmanifest:fword FwCanProduceManaColor
func (s *State) fwCanProduceManaColor(c *fctx, l *kop.FLit) bool {
	if !c.perm() {
		return false
	}
	return s.cardProducible(c.k, s.cardAt(c.sl))&kop.ColorMask(l.Arg) != 0
}
