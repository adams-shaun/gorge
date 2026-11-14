package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prepUnearth turns the leave hook on for a table with an OpUnearth.
func prepUnearth(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpUnearth {
					g.anyUnearth = true
					return
				}
			}
		}
	}
}

// unearth applies CR 702.84a's riders to the unearthed source the ability
// just returned (gorge rules/unearth.go unearthEnter): haste until end of
// turn, and the exile at the beginning of the next end step, a delayed
// record tracking this incarnation whose act is OpUnearth. Nothing when the
// return did not happen (the card left the graveyard first).
//
//go:noinline
func (s *State) unearth(k *prep) {
	o := s.res.e.obj
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	s.touchSl(ob.slot)
	s.grantEOT(ob.slot, kop.KwHaste)
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{act: uint8(kop.OpUnearth), ctrl: s.res.e.ctrl, obj: o, ver: objWord(ob.ver), creator: NoObj}
	s.ndly++
}

// unearthZone is the zone a permanent leaving the battlefield for z goes
// to: exile while its unearth promise is live (CR 702.84a "or if it would
// leave the battlefield"; gorge unearthReplacementApplies reads the live
// __kwUnearthExile registration of this incarnation, gone once the end
// step trigger fired).
func (s *State) unearthZone(sl Slot, z kop.Zone) kop.Zone {
	if DebugNoUnearthLeave {
		return z
	}
	o := s.perms[sl].obj
	v := s.objs[o].ver
	for i := 0; i < int(s.ndly); i++ {
		if d := &s.dly[i]; d.act == uint8(kop.OpUnearth) && d.obj == o && d.ver == objWord(v) {
			return kop.ZoneExile
		}
	}
	return z
}

// DebugNoUnearthLeave (TestUnearthLockstepLeaveMutation): the exile-instead
// promise is ignored.
var DebugNoUnearthLeave bool
