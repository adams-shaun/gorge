package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// hexPrinted is the colours of the printed "hexproof from <colour>" keywords
// (CR 702.11d) of the permanent at sl: 0 when it has lost its abilities
// (gorge hexproofBlocksTarget reads the derived keywords). Cold: targetableGen
// calls it behind genPrep.anyHexCol.
//
//go:noinline
func (s *State) hexPrinted(k *prep, sl Slot) kop.ColorMask {
	c := k.t.Cards[s.cardAt(sl)].HexColors
	if c == 0 || s.genLost(k, sl) {
		return 0
	}
	return c
}
