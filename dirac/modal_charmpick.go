package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ChoiceRestriction$ on a charm ("choose one that hasn't been chosen this
// turn / this game"). gorge keeps one per-source pick log on the source
// OBJECT (state.Object.ModeChoices): a pick is recorded when the mode is
// announced (effects.RecordCharmChoices at a trigger's placement, an
// activation's announcement), a later offer of the same charm on the same
// source drops the modes picked under its scope (effects.CharmEligibleModes),
// the turn change drops the ThisTurn picks, and LEAVING THE BATTLEFIELD
// empties the log (CR 400.7: the moving object is a new object). The log
// follows the object everywhere else: a trigger of a source that left the
// battlefield still resolves (CR 603.6c) and both records and reads through
// the object's log, so a queue-mate resolving after it drops the mode it
// picked.
//
// The kernel keeps the same log as two halves:
//
//   - State.g.chPick, one entry per battlefield SLOT, holds the picks made
//     while the source was on the battlefield (an on-battlefield source is
//     found by its slot, and the slot's entry is dropped when it leaves --
//     leave(), the same CR 400.7 clear -- and when a new permanent enters,
//     enterGen).
//   - coldCharmPick, one record per source OBJECT (object + 1, 0 empty),
//     holds every other pick: a source off the battlefield (the queued LTB
//     triggers above), a spell on the stack announcing its modes. Object ids
//     are unique for the game (engine.go allocates them monotonically), so
//     the record is the object's alone, and it survives the object's zone
//     moves the way gorge's log does -- only leave() empties it.
//
// A read consults both halves (the union is gorge's one log): by slot while
// the object is on the battlefield, always by object for the carried picks.
// kgen refuses a second restricted charm on a card, so one scope per source.
// Both logs hold at most 6 entries; a seventh concurrent source stalls the
// game (StallCharmPick). The cold records are not swept: no state proves a
// carried log unreadable (a re-entered source reads its carried picks, gorge
// keeps them too).
//
// Everything is behind genPrep.anyCharmPick (recording, offering) or
// genState.nchPick != 0 / a cold record (the resets).

// DebugCharmPickNoTurnReset keeps the ThisTurn picks across the turn change
// (mutation test only).
var DebugCharmPickNoTurnReset bool

// DebugCharmPickNoGameScope records a ThisGame pick as a ThisTurn one, so it
// clears at the turn change (mutation test only).
var DebugCharmPickNoGameScope bool

const (
	cpGame = 0x80 // chPick/cp m bit: the ThisGame scope
	// maxCpCold is how many off-battlefield (or on-the-stack) pick records a
	// table with a restricted charm keeps in the cold buffer, the State log's
	// 6 entries beside it.
	maxCpCold = 6
)

func init() { stallNames[StallCharmPick] = "charmpick" }

// cpRec is one pick log record for a source off the battlefield: its object
// + 1 (0 empty) and the picked mode indices (bits 0-3) with the scope bit.
type cpRec struct {
	o Obj
	m uint8
}

// coldCharmPick is the cold half of the pick log, for a table with a
// restricted charm.
var coldCharmPick = newCold[cpRec]("charmpick", func(g *genPrep) int {
	if g.anyCharmPick {
		return maxCpCold
	}
	return 0
})

// prepCharmPick sets anyCharmPick: some ability carries ChoiceRestriction$.
func prepCharmPick(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			g.anyCharmPick = g.anyCharmPick || t.Cards[i].Abilities[j].ModeRestrict != 0
		}
	}
}

// charmPickAllBarred reports whether every mode of a was picked before.
func (s *State) charmPickAllBarred(a *kop.Ability, o Obj) bool {
	for m := 0; m < int(a.NModes); m++ {
		if !s.charmPickBarred(a, o, m) {
			return false
		}
	}
	return a.ModeRestrict != 0
}

// cp is the cold log's records.
func (s *State) cp() []cpRec { return coldCharmPick.of(s) }

// cpAt is the index of object o's record (o + 1), or -1.
func (s *State) cpAt(o Obj) int {
	for i, c := range s.cp() {
		if c.o == Obj(o)+1 {
			return i
		}
	}
	return -1
}

// cpBarred reports whether mode m of a was picked on the cold log.
func (s *State) cpBarred(a *kop.Ability, o Obj, m int) bool {
	i := s.cpAt(o)
	if i < 0 {
		return false
	}
	e := s.cp()[i].m
	return (e&cpGame != 0) == (a.ModeRestrict == 2) && e&(1<<m) != 0
}

// charmPickBarred reports whether mode m of a, a restricted charm on source
// object o, was picked before under a's scope: the State log while o is on
// the battlefield (the picks of its current stint), the cold log always (the
// picks it carried out of an earlier stint or made off the battlefield).
func (s *State) charmPickBarred(a *kop.Ability, o Obj, m int) bool {
	if a.ModeRestrict == 0 || o == NoObj {
		return false
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		if i := s.charmPickAt(ob.slot); i >= 0 {
			e := s.g.chPick[i].m
			if (e&cpGame != 0) == (a.ModeRestrict == 2) && e&(1<<m) != 0 {
				return true
			}
		}
	}
	return s.cpBarred(a, o, m)
}

// charmPickRecord logs mode m, just announced for the entry being built, on
// its source object (gorge RecordCharmChoices; a source that left the
// battlefield has its own record, never the battlefield slot's).
//
//go:noinline
func (s *State) charmPickRecord(k *prep, m uint8) {
	if a := s.buildAbility(k); a != nil {
		s.charmPickLog(a, s.bld().e.obj, m)
	}
}

// charmPickLog logs mode m of a, a restricted charm on source object o: the
// State log keyed by o's battlefield slot while o is on the battlefield, the
// cold log keyed by o otherwise.
//
//go:noinline
func (s *State) charmPickLog(a *kop.Ability, o Obj, m uint8) {
	if a.ModeRestrict == 0 || o == NoObj {
		return
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		s.charmPickSlotLog(a, ob.slot, m)
		return
	}
	s.cpLog(a, o, m)
}

// charmPickSlotLog logs mode m of a on the permanent at battlefield slot sl.
func (s *State) charmPickSlotLog(a *kop.Ability, sl Slot, m uint8) {
	i := s.charmPickAt(sl)
	if i < 0 {
		if int(s.g.nchPick) == len(s.g.chPick) {
			s.charmPickSweep()
		}
		if int(s.g.nchPick) == len(s.g.chPick) {
			s.capFull(StallCharmPick)
			return
		}
		i = int(s.g.nchPick)
		s.g.nchPick++
		s.g.chPick[i].sl = sl
		s.g.chPick[i].m = 0
		if a.ModeRestrict == 2 && !DebugCharmPickNoGameScope {
			s.g.chPick[i].m = cpGame
		}
	}
	s.g.chPick[i].m |= 1 << m
}

// cpLog logs mode m of a on source object o in the cold log.
func (s *State) cpLog(a *kop.Ability, o Obj, m uint8) {
	cs := s.cp()
	i := s.cpAt(o)
	if i < 0 {
		for i = 0; i < len(cs); i++ {
			if cs[i].o == 0 {
				break
			}
		}
		if i == len(cs) {
			s.capFull(StallCharmPick)
			return
		}
		cs[i].m = 0
		if a.ModeRestrict == 2 && !DebugCharmPickNoGameScope {
			cs[i].m = cpGame
		}
	}
	cs[i].o = Obj(o) + 1
	cs[i].m |= 1 << m
}

// charmPickAt is the index of slot sl's entry, or -1.
func (s *State) charmPickAt(sl Slot) int {
	for i := 0; i < int(s.g.nchPick); i++ {
		if s.g.chPick[i].sl == sl {
			return i
		}
	}
	return -1
}

// charmPickDel drops State entry i, keeping the order of the rest.
func (s *State) charmPickDel(i int) {
	n := int(s.g.nchPick) - 1
	copy(s.g.chPick[i:n], s.g.chPick[i+1:n+1])
	s.g.chPick[n] = struct {
		sl Slot
		m  uint8
	}{}
	s.g.nchPick = uint8(n)
}

// charmPickSweep drops the entries of slots no permanent occupies (their
// source left; the entry would be dropped at the slot's next enter anyway).
func (s *State) charmPickSweep() {
	for i := int(s.g.nchPick) - 1; i >= 0; i-- {
		if !s.occ.has(s.g.chPick[i].sl) {
			s.charmPickDel(i)
		}
	}
}

// charmPickLeft drops the leaving permanent's log: the State entry of its
// battlefield slot and its cold record. gorge empties the object's log when
// it leaves the battlefield (CR 400.7); a pick a later trigger of the same
// source makes is recorded after the leave and starts a fresh log.
//
//go:noinline
func (s *State) charmPickLeft(sl Slot, o Obj) {
	s.charmPickSlotLeft(sl)
	if o != NoObj {
		if i := s.cpAt(o); i >= 0 {
			s.cp()[i] = cpRec{}
		}
	}
}

// charmPickSlotLeft drops battlefield slot sl's State entry: a permanent
// entering it starts with no pick log (CR 400.7; leave() drops the cold
// record of the permanent that left, and an entering permanent keeps the
// picks its object carried the way gorge's entering object does).
//
//go:noinline
func (s *State) charmPickSlotLeft(sl Slot) {
	if i := s.charmPickAt(sl); i >= 0 {
		s.charmPickDel(i)
	}
}

// charmPickTurn drops the ThisTurn picks at the turn change.
//
//go:noinline
func (s *State) charmPickTurn() {
	if DebugCharmPickNoTurnReset {
		return
	}
	for i := int(s.g.nchPick) - 1; i >= 0; i-- {
		if s.g.chPick[i].m&cpGame == 0 {
			s.charmPickDel(i)
		}
	}
	for i := range s.cp() {
		if c := &s.cp()[i]; c.o != 0 && c.m&cpGame == 0 {
			*c = cpRec{}
		}
	}
}
