package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A delayed program over a many-object remembered set.
// Random Encounter's ChangeZoneAll ForgetOtherRemembered$ leaves the milled
// creatures that entered (0..4, hand-sized by the mill) as Remembered, and a
// RememberObjects$ Remembered delayed trigger returns "those creatures" to
// hand at the next end step. OpDelayedProgram with kop.DpMany registers the
// whole set (dprog.objs[0..4), a vers byte each); fire carries the objects
// into the pending trigger and the stack entry without new fields:
//
//	object 1  trigObj
//	object 2  trigAmt = obj+1 (dpSecond: the Mobilize carrier)
//	object 3  x       = obj+1 (0 = none)
//	object 4  trigMV  = obj+1 (0 = none)
//
// x and trigMV are overloaded here: a delayed program's entry never reads
// them (x is a spell's paid X or a trigger's bound X, trigMV a SpellCast
// trigger's pre-cast mana value), and the only program that fires with
// dpMany set runs bounceMany. Fire drops any object whose ver changed since
// registration (CR 400.7: a new object), so an entry names only live ones.

// dpMany marks a dprog record the many-object form (kop.DpMany). Bit 4 of
// dprog.n: dpTrack 1<<2, dpDies 1<<3, dpSpell 1<<5, the count 0-1, timing 6-7.
const dpMany uint8 = 1 << 4

// DebugDpManyOff is the mutation switch: the many-object return moves only
// the entry's first object.
var DebugDpManyOff bool

// delayMany is OpDelayedProgram with kop.DpMany: register the resolution's
// remembered set (at most four) as one program. More than four stalls; none
// registers a program that fires and does nothing (CR 603.7, as gorge's
// empty set).
//
//go:noinline
func (s *State) delayMany(e *stackEntry, in *kop.Instr) {
	n := int(s.g.nrem)
	if n > len(dprog{}.objs) {
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
	d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: 1 | dpMany | dpWhen(in), ext: dpExt(in)}
	for i := range d.objs {
		d.objs[i] = NoObj
		if i < n {
			o := s.g.rem[i]
			d.objs[i], d.vers[i] = o, s.objs[o].ver
		}
	}
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}

// dpManyObj is d's object i when it is unchanged since registration, else
// NoObj.
func (s *State) dpManyObj(d *dprog, i int) Obj {
	o := d.objs[i]
	if o == NoObj || s.objs[o].ver != d.vers[i] {
		return NoObj
	}
	return o
}

// dpManyCarry fills pt's carriers for objects 2..4 of the firing many-object
// program d (object 1 is trigObj, set by the fire site).
//
//go:noinline
func (s *State) dpManyCarry(d *dprog, pt *pendingTrig) {
	pt.trigAmt, pt.x, pt.trigMV = 0, 0, 0
	pt.skw |= skwMany
	if o := s.dpManyObj(d, 1); o != NoObj {
		pt.trigAmt = int16(o) + 1
	}
	if o := s.dpManyObj(d, 2); o != NoObj {
		pt.x = objWord(o) + 1
	}
	if o := s.dpManyObj(d, 3); o != NoObj {
		pt.trigMV = objWord(o) + 1
	}
}

// dpManyVers stamps on the stacking entry e the versions of the objects 2..4
// its carriers name (stackEntry.tver, which a delayed entry has no targets to
// use): no pending trigger waits through an object's zone change, so they are
// the versions the program remembered.
func (s *State) dpManyVers(e *stackEntry) {
	if e.trigAmt > 0 {
		e.tver[0] = s.objs[e.trigAmt-1].ver
	}
	if e.x > 0 {
		e.tver[1] = s.objs[e.x-1].ver
	}
	if e.trigMV > 0 {
		e.tver[2] = s.objs[e.trigMV-1].ver
	}
}

// dpManyCarrier is object i (0..3) the entry names with the ver the program
// remembered it at, in registration order: dpManyCarry's layout (object 1 in
// trigObj/remVer, 2..4 in the trigAmt, x and trigMV carriers with tver).
// NoObj when the slot is empty. The caller filters on what it needs (still on
// the battlefield, still in exile).
func (e *stackEntry) dpManyCarrier(i int) (Obj, uint8) {
	switch i {
	case 0:
		return e.trigObj, uint8(e.remVer)
	case 1:
		if e.trigAmt > 0 {
			return Obj(e.trigAmt - 1), e.tver[0]
		}
	case 2:
		if e.x > 0 {
			return Obj(e.x - 1), e.tver[1]
		}
	default:
		if e.trigMV > 0 {
			return Obj(e.trigMV - 1), e.tver[2]
		}
	}
	return NoObj, 0
}

// bounceMany is BncDelayedMany: each object the resolving entry names, in
// registration order, that is still on the battlefield and still the object
// the program remembered returns to its owner's hand (the body's Origin$
// Battlefield). An object another trigger exiled and returned in between,
// Y'shtola Rhul's, is a new one with nothing to return (CR 400.7, 603.7c).
//
//go:noinline
func (s *State) bounceMany(k *prep) {
	e := &s.res.e
	for i := 0; i < 4; i++ {
		o, ver := e.dpManyCarrier(i)
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield || s.objs[o].ver != ver {
			continue
		}
		s.leave(k, s.objs[o].slot, kop.ZoneHand, false)
		if DebugDpManyOff {
			return
		}
	}
}
