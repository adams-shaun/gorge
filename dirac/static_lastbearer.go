package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the last-known bearer of a departed Aura or
// Equipment (gorge state.Object.LastBearer; CR 608.2h).
//
// gorge's definedSpecEquipped (engine/effects/context.go:1306-1328) reads
// the source's AttachedTo while it is on the battlefield and its
// LastBearer once it is off. The kernel's live link already covers the
// first case; this file remembers the bearer for the second. The record is
// per departed permanent, in its perm's chosen byte, so three Auras on one
// host all keep their own (the old two-entry attLKI pad could not).

// pfLastBear marks a perm record whose chosen byte holds the object this
// departed permanent was last attached to. The bit is outside every flag
// the projection and the rules read, so a stale record cannot leak.
const pfLastBear uint16 = 1 << 14

// EquipShufDebug is the lockstep test's observation
// window: Watch names the carrier CardIDs, Hits counts each one's resolved
// shuffles, and Gone counts the resolutions that had to use LastBearer (the
// bearer had already left the battlefield). Cold: only a resolution reads
// it.
type EquipShufDebug struct {
	On        bool
	Watch     [8]kop.CardID
	Hits      [8]int
	Gone      int
	Token     int
	TokenGone int
}

// DebugEquipShuf is the test hook above.
var DebugEquipShuf EquipShufDebug

// DebugEquipShufNoShuffle is the mutation knob: toLibraryShuffle skips the
// shuffle of a RefAttached referent, and the lockstep must diverge.
var DebugEquipShufNoShuffle bool

// noteLastBear parks the host of a leaving Aura or Equipment while it is still on
// the battlefield. perm.bord is the non-creature attachment's unused combat-order
// byte (a bestowed Aura becomes a creature and is left alone), and the
// projection does not expose it; removePerm turns it into the departed
// record's chosen byte, and attaching again clears the mark. Cold: only a host's
// departure reaches it.
//
//go:noinline
func (s *State) noteLastBear(q Slot, host Obj) {
	k := s.k()
	if k == nil {
		return
	}
	if t := s.typesAt(k, q); t&kop.TAura == 0 && t&kop.TCreature != 0 {
		return
	}
	if k.gen != nil && k.gen.anyBestow && k.t.Cards[s.cardAt(q)].HasBestow {
		return
	}
	p := &s.perms[q]
	p.bord = objWord(host)
	p.flags |= pfLastBear
}

// recordLastBearLKI keeps a two-record fallback for the case the departed
// permanent's slot is reused before its ability resolves. ver == 0 marks a
// LastBearer record (a battlefield host's ver is never 0), so lkiAttached's
// ordinary AttachedTo lookup skips it and the reuse loop treats it as free.
//
//go:noinline
func (s *State) recordLastBearLKI(o, host Obj) {
	// A departure whose bearer had already left (the live link was broken
	// first) supersedes any earlier departure of the same attachment: drop
	// the older versioned AttachedTo record, or attLKIHost would resurrect a
	// bearer the attachment left long before (gorge Object.LastBearer names
	// only the MOST RECENT departure, CR 608.2h).
	//
	// Clear to the pad's empty sentinel, not the zero record: zero names
	// object 0, a real card, so lastBearerObj(0) would read the phantom and
	// the obj == NoObj free-slot test would never reclaim it.
	//
	// The clear is keyed on the card object, not its incarnation: the pad
	// holds only two records, so a trigger still pending from an earlier
	// incarnation of this object can lose the record dropped here and read
	// the newest bearer where gorge names the older one. That is the pad's
	// bounded capacity, widened by this clear; it is not an unbounded
	// divergence, and the slot scan still covers an unreused slot.
	if s.objs[host].zone != kop.ZoneBattlefield {
		for i := range s.g.attLKI {
			if r := &s.g.attLKI[i]; r.obj == o && r.ver != 0 {
				r.obj, r.host, r.ver = NoObj, NoObj, 0
			}
		}
	}
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.obj == o && r.ver == 0 {
			r.host = host
			return
		}
	}
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.ver == 0 && r.obj == NoObj {
			r.obj, r.host = o, host
			return
		}
	}
	// never displace a live AttachedTo record (ver != 0): the departed
	// perm's chosen byte is the primary record and the slot scan finds it.
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.ver == 0 {
			r.obj, r.host = o, host
			return
		}
	}
}

// dropAttLKI forgets every departure record for o. An already-unattached
// departure names no bearer (gorge Object.LastBearer = 0), so a host an
// earlier departure recorded must not survive (CR 608.2h). Reset to the
// pad's empty sentinel: a zero record names object 0, a real card (Init).
//
//go:noinline
func (s *State) dropAttLKI(o Obj) {
	for i := range s.g.attLKI {
		if r := &s.g.attLKI[i]; r.obj == o {
			r.obj, r.host, r.ver = NoObj, NoObj, 0
		}
	}
}

// lastBearerObj is the object off-battlefield permanent o was most
// recently attached to, NoObj when it never was (gorge LastBearer). The
// departed perm record is authoritative; attLKI covers a reused slot.
//
//go:noinline
func (s *State) lastBearerObj(o Obj) Obj {
	for sl := Slot(0); sl < MaxPerm; sl++ {
		p := &s.perms[sl]
		if p.obj == o && p.ts != 0 && !s.occ.has(sl) && p.flags&pfLastBear != 0 {
			return Obj(p.chosen)
		}
	}
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.obj == o && r.obj != NoObj && r.ver == 0 {
			return r.host
		}
	}
	return NoObj
}

// noteEquipShuf counts a resolved shuffle for the lockstep's minimums. Cold:
// only a resolution reaches it with the test switch on.
//
//go:noinline
func (s *State) noteEquipShuf(o Obj, lastBear bool) {
	if lastBear {
		DebugEquipShuf.Gone++
	}
	if s.objs[o].of&ofToken != 0 {
		DebugEquipShuf.Token++
		if lastBear {
			DebugEquipShuf.TokenGone++
		}
	}
	c := s.res.e.card
	for i := range DebugEquipShuf.Watch {
		if DebugEquipShuf.Watch[i] == c {
			DebugEquipShuf.Hits[i]++
			return
		}
	}
}
