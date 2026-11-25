package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// LeaveBattlefield$ Exile on a move to the battlefield (Isareth the
// Awakener, From the Catacombs): "if that creature would leave the
// battlefield, exile it instead of putting it anywhere else". gorge
// registerLeaveExile (engine/effects/leavebattlefield.go) adds, once the
// object has entered, a Moved replacement scoped to that object's own
// battlefield departure that rewrites the destination to exile, ended by the
// departure itself (ExileOnMoved), so a later incarnation is a plain
// permanent (CR 400.7).
//
// The kernel keeps the promise as a delayed record (when dlyLeaveExile) over
// the object and the version it entered with, the unearth promise's shape
// (kw_unearth.go unearthZone) without its end-step exile: the end step keeps
// it (splitLeaveDelayed), the leave paths read it (zones.go fireLeave and
// removePerm), and a record whose object has moved on is dead and is dropped
// at the next registration or read. No State byte: the record lives in the
// delayed array.

// dlyLeaveExile is delayed.when for an exile-instead promise.
const dlyLeaveExile = 8

// opLeaveExile is the promise record's act (kernel-internal, above every
// kop.Op; never dispatched: the record never fires).
const opLeaveExile = 231

// DebugNoLeaveExile (TestSetsObjects5LeaveExileLockstep's mutation): the
// promise is ignored.
var DebugNoLeaveExile bool

// DebugLeaveExileHook (oracle tests only; nil in play): reg is called with
// the source card of each registered promise, hit with each object a promise
// sends to exile instead (called once per leave path that reads it).
var DebugLeaveExileHook *struct {
	Reg func(c CardID)
	Hit func(o Obj)
}

// prepLeaveExile turns the leave hook on for a table with an ExtLeaveExile.
func prepLeaveExile(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtLeaveExile {
					g.anyLeaveExile = true
					return
				}
			}
		}
	}
}

// leaveExileReg is ExtLeaveExile: the promise over the object in.Obj names,
// when the move before it put that object onto the battlefield. A move that
// did not happen (a missing target, the card gone from the graveyard)
// registers nothing.
//
//go:noinline
func (s *State) leaveExileReg(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	s.leaveExilePrune()
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{act: opLeaveExile, ctrl: s.res.e.ctrl, obj: o, ver: objWord(s.objs[o].ver), when: dlyLeaveExile, creator: NoObj}
	s.ndly++
	if h := DebugLeaveExileHook; h != nil {
		h.Reg(s.res.e.card)
	}
}

// leaveExilePrune drops the promise records whose object is no longer the
// incarnation they were made for.
func (s *State) leaveExilePrune() {
	w := uint8(0)
	for i := uint8(0); i < s.ndly; i++ {
		d := s.dly[i]
		if d.when == dlyLeaveExile && (s.objs[d.obj].zone != kop.ZoneBattlefield || objWord(s.objs[d.obj].ver) != d.ver) {
			continue
		}
		s.dly[w] = d
		w++
	}
	s.ndly = w
}

// leaveExileZone is the zone a permanent leaving the battlefield for z goes
// to: exile while its promise is live.
func (s *State) leaveExileZone(sl Slot, z kop.Zone) kop.Zone {
	if DebugNoLeaveExile {
		return z
	}
	o := s.perms[sl].obj
	v := objWord(s.objs[o].ver)
	for i := 0; i < int(s.ndly); i++ {
		if d := &s.dly[i]; d.when == dlyLeaveExile && d.obj == o && d.ver == v {
			if h := DebugLeaveExileHook; h != nil {
				h.Hit(o)
			}
			return kop.ZoneExile
		}
	}
	return z
}
