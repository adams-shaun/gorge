package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mobilize 1 and 2 (CR 702.181a). gorge expands the keyword
// into an Attacks trigger that mints the Warrior tokens remembered and a
// RememberChain$ False delayed trigger whose body sacrifices the surviving
// group (Defined$ DelayTriggerRememberedLKI) as one batch at the next end
// step. A delayed program (dprog) carries the whole group: objs[0..n) with
// the count in n's low bits; firing passes the second object to the program
// in the pending trigger's trigAmt (packed as obj+1, 0 = none).

const dpCountMask = 3

// delayGroup is OpDelayedProgram with kop.DpGroup: register the resolution's
// remembered set as one program. More than two (a token doubler) stalls; none
// registers a program that fires and does nothing (as gorge's empty set).
//
//go:noinline
func (s *State) delayGroup(e *stackEntry, in *kop.Instr) {
	n := int(s.g.nrem)
	if n > 2 {
		s.capFull(StallDProg)
		return
	}
	if n == 0 {
		s.delayEmptyGen(e, uint8(in.C))
		if in.A.N != 0 {
			s.g.dp[s.g.ndp-1].n |= dpWhen(in)
			s.g.dp[s.g.ndp-1].ext = dpExt(in)
		}
		return
	}
	d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: uint8(n) | dpWhen(in), ext: dpExt(in)}
	for i := 0; i < n; i++ {
		o := s.g.rem[i]
		d.objs[i], d.vers[i] = o, s.objs[o].ver
	}
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}

// dpSecond is the packed trigAmt of a firing program d: its second object
// (when it carries two and that object is unchanged), else 0.
//
//go:noinline
func (s *State) dpSecond(d *dprog) int16 {
	o := d.objs[1]
	if o == NoObj || s.objs[o].ver != d.vers[1] {
		return 0
	}
	return int16(o) + 1
}

// sacDelayed is ExtSacDelayed: the group's members still on the battlefield
// are sacrificed together.
//
//go:noinline
func (s *State) sacDelayed(k *prep) {
	e := &s.res.e
	var hit slotMask
	for i := 0; i < 2; i++ {
		var o Obj
		if i == 0 {
			o = e.trigObj
		} else if e.trigAmt > 0 {
			o = Obj(e.trigAmt - 1)
		} else {
			break
		}
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield || s.sacBlocked(k, s.objs[o].slot) {
			continue
		}
		sl := s.objs[o].slot
		if s.perms[sl].ctrl != e.ctrl {
			s.capFull(StallSacOther) // gorge checks no controller; CR 701.21a would
			return
		}
		hit = hit.with(sl)
	}
	if !hit.any() {
		return
	}
	s.fireSacAll(k, hit)
	s.leaveAll(k, hit, kop.ZoneGraveyard)
}
