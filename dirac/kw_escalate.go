package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Escalate (CR 702.120a): a modal spell's additional cost, paid once for each
// mode chosen beyond the first (CardDef.Escalate). It rides Spree's machinery
// (HasModeCost with every CardDef.ModeCost zero; kw_spree.go): the cast is
// offered on its printed cost, the chosen modes' total adds (modes-1) copies
// of the escalate cost (buildCost0 through spreeCost), and a mode is offered
// at the mode step only while the set picked so far plus it can be paid, which
// is gorge's castModeAsk clamp of the ask's Max to one plus the escalations
// the floating pool can afford (castable reads the pool alone at the ask: the
// CR 601.2g window that taps the lands comes after the modes; the cost depends
// on the count alone, so the clamp and the one-by-one offer agree). The first
// mode is never withheld for its price.

// escalateCost adds the escalate cost for each mode of the set beyond the
// first to c.
func escalateCost(cd *kop.CardDef, mode uint8, c kop.ManaCost) kop.ManaCost {
	for n := bits.OnesCount8(mode & 0x0f); n > 1; n-- {
		c = addCost(c, cd.Escalate)
	}
	return c
}

// escalateModeAffordable: mode m can join the modes picked so far, the pool
// paying the printed cost plus the escalate cost for each mode beyond the first.
//
//go:noinline
func (s *State) escalateModeAffordable(k *prep, m int) bool {
	e := &s.bld().e
	if e.mode&0x0f == 0 {
		return true // the first mode costs nothing extra (gorge: Min 1)
	}
	saved := e.mode
	e.mode = saved | 1<<m
	c := s.buildCost(k)
	e.mode = saved
	return s.windowPoolPays(e.ctrl, &c, s.bld().bfl&bflAnyColor != 0)
}
