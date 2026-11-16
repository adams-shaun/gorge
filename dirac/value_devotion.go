package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// devotion is CR 700.5 devotion (gorge devotionCount): the mana symbols in
// the printed mana costs of the permanents player p controls whose colours
// meet mask; a hybrid symbol of two colours counts once. A face-down
// permanent has no mana cost (CR 708.2) and an embalm copy loses it
// (CR 707.9b): neither contributes (the ExDistinctCMC scan's same read).
//
//go:noinline
func (s *State) devotion(k *prep, mask kop.ColorMask, p uint8) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if s.faceDown(k, m.first()) || s.cpxNoCost(k, m.first()) {
			continue
		}
		c := &k.t.Cards[s.objs[s.perms[m.first()].obj].card].Cost
		for i := 0; i < 5; i++ {
			if mask&(1<<i) != 0 {
				n += int(c.Pips[i])
			}
		}
		for _, h := range c.Hyb {
			if h&mask != 0 {
				n++
			}
		}
		if c.Tb != 0 {
			n += tbDevotion(c, mask)
		}
	}
	return n
}
