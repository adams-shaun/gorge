package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A static damage prevention an emblem carries (Ajani Steadfast's -7: "If a
// source would deal damage to you or a planeswalker you control, prevent all
// but 1 of that damage", Duration$ Permanent). The emblem is the runtime
// emblem of OpCreateEmblem (State.g.nemb) with none of the UntilEOT,
// UntilNextTurn or host flags, so no cleanup drops it (cleanupGen drops only
// the flagged kinds); State does not grow. spCollect walks every live emblem
// after the battlefield and matches an emblem ability as the permanent's own
// (spMatchEmb: "you" is the emblem's seat, there is no host).
//
// A candidate's host slot is embSlot(seat), a value no battlefield slot
// takes, and its ability index is the flat index of the ability over the
// seat's emblems in creation order (spCandAb). An emblem candidate applies
// only on its own: beside a second candidate the kernel stalls
// StallReplCands, so no order ask ever names an emblem (spPark, spOptions).

// DebugNoEmblemPrev (the emblem prevention lockstep mutation): the emblem's
// prevention is never collected as a candidate.
var DebugNoEmblemPrev bool

// embSlotBase is the first emblem pseudo-slot: at or above MaxPerm, below NoSlot.
const embSlotBase = MaxPerm + 8

// embSlot is the pseudo host slot of the emblems of seat.
func embSlot(seat uint8) Slot { return Slot(embSlotBase + int(seat)) }

// tableEmbHasStaticPrev: some emblem of t carries a static damage prevention.
func tableEmbHasStaticPrev(t *kop.Table) bool {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if isStaticPrev(&t.Emblems[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// prepEmbPrev opens every damage site when some emblem carries a static
// damage prevention (prep.embPrev), the way a printed static prevention's
// host does (no card carries the bit: the emblem is created at resolution).
func prepEmbPrev(p *prep, g *genPrep) {
	if p.embPrev {
		g.anyRepl |= rbPlusDmg
	}
}

// spMatchEmb reports whether the static prevention a of an emblem of seat is
// a candidate of event e: spMatch with "you" the emblem's seat and no host.
func (s *State) spMatchEmb(k *prep, seat uint8, a *kop.Ability, e *spEv) bool {
	return s.spMatchAs(k, seat, NoSlot, NoObj, a, e)
}

// eachEmbPrev calls f for every static prevention of every live emblem, with
// the emblem's seat and the ability's flat index over that seat's emblems; f
// returns false to stop.
func (s *State) eachEmbPrev(k *prep, f func(seat uint8, flat int, a *kop.Ability) bool) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		flat := 0
		for i := 0; i < int(s.g.nemb[p]); i++ {
			e := &k.t.Emblems[s.embIdx(p, i)-1]
			for j := range e.Abilities {
				if a := &e.Abilities[j]; isStaticPrev(a) && !f(p, flat, a) {
					return
				}
				flat++
			}
		}
	}
}

// spCollectEmb appends the matching emblem preventions to the n candidates of
// e that spCollect found on the battlefield, and returns the new count; -1
// after a loud stall (more than spMax, or an emblem candidate beside another).
func (s *State) spCollectEmb(k *prep, e *spEv, out *[spMax]spCand, n int) int {
	if !k.embPrev || DebugNoEmblemPrev {
		return n
	}
	emb, over := false, false
	s.eachEmbPrev(k, func(seat uint8, flat int, a *kop.Ability) bool {
		if !s.spMatchEmb(k, seat, a, e) {
			return true
		}
		if n == spMax {
			over = true
			return false
		}
		out[n] = spCand{embSlot(seat), uint8(flat)}
		n++
		emb = true
		return true
	})
	if over || emb && n > 1 {
		s.capFull(StallReplCands) // loud: never truncate, never order an emblem
		return -1
	}
	return n
}

// embPrevHit reports whether a live emblem's static prevention is a candidate
// of e (prevNextCompetes' superset guard).
func (s *State) embPrevHit(k *prep, e *spEv) bool {
	if DebugNoEmblemPrev {
		return false
	}
	hit := false
	s.eachEmbPrev(k, func(seat uint8, _ int, a *kop.Ability) bool {
		hit = s.spMatchEmb(k, seat, a, e)
		return !hit
	})
	return hit
}

// spCandAb is the static prevention candidate c names: the host's ability, or
// the emblem ability at the flat index of an emblem pseudo-slot.
func (s *State) spCandAb(k *prep, c spCand) *kop.Ability {
	if c.b < MaxPerm {
		return &k.t.Cards[s.cardAt(c.b)].Abilities[c.ab]
	}
	seat := uint8(int(c.b) - embSlotBase)
	flat := 0
	for i := 0; i < int(s.g.nemb[seat]); i++ {
		e := &k.t.Emblems[s.embIdx(seat, i)-1]
		if int(c.ab) < flat+len(e.Abilities) {
			return &e.Abilities[int(c.ab)-flat]
		}
		flat += len(e.Abilities)
	}
	panic("dirac: emblem prevention candidate names no emblem ability")
}

// emblemIdem reports whether a second copy of emblem e is no new emblem: the
// end-of-turn emblem of CantAttack statics (static_emblemcombat.go), or the
// permanent prevention emblem (no lifetime flag, every ability a static
// prevention: its Effect is Stackable$ False, so gorge registers a second -7's
// emblem not at all and the one emblem competes alone).
func emblemIdem(e *kop.Emblem) bool {
	if e.UntilNextTurn || e.UntilEndOfNextTurn || e.HostBound() || len(e.Abilities) == 0 {
		return false
	}
	for j := range e.Abilities {
		a := &e.Abilities[j]
		if e.UntilEOT && (a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantAttack) || !e.UntilEOT && !isStaticPrev(a) {
			return false
		}
	}
	return true
}

// prepEmbIdem is prep.embIdem: the table has an emblem idemEmblemHeld reads.
func prepEmbIdem(p *prep) bool { return p.embComb || p.embPrev }
