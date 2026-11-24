package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A scripted start (docs/design/scripted-scenario-starts.md section 3): the
// game begins from an ordinary shuffle and deal, then the start's placements
// are applied before the first turn. Start is an Init-time argument only; it
// is never stored in State, so it adds no State bytes.
//
// Every index is a position in the seat's deck slice as passed to InitNStart,
// so card i of seat p is object off(p)+i with off(p) the decks laid out in
// seat order, the numbering both engines share.

// Start is the scripted position, one StartSeat per seat in seat order (fewer
// than the game's seat count leaves the rest dealt as usual).
type Start struct {
	Seats []StartSeat
}

// StartSeat is one seat's placements. Hand, Battlefield, Graveyard, Exile and
// Library list deck indices; the zones are disjoint. Library is the library
// top, top first; the rest of the library follows the closed rule in
// libOrder. Tapped and BackFace name battlefield cards; Counters add to
// battlefield cards. Slices only: no map, so every walk is ordered.
type StartSeat struct {
	Hand, Battlefield, Graveyard, Exile, Library []int
	Tapped, BackFace                             []int
	Counters                                     []StartCounter
	Attach                                       []StartAttach
	Life                                         *int32
}

// StartCounter adds N counters of Kind to battlefield card Card.
type StartCounter struct {
	Card int
	Kind kop.CounterKind
	N    int32
}

// StartAttach enters battlefield Aura Card of this seat attached to the
// battlefield permanent HostCard of seat HostSeat (CR 303.4f). The Aura is
// not in Battlefield: it enters after every plain battlefield card, so its
// host is already there.
type StartAttach struct {
	Card     int
	HostSeat int
	HostCard int
}

// startObjs resolves the seat-order object numbering of decks and validates
// st against it (indices in range, every placed card named once). A bad
// start panics: the loader refuses these before a game is built.
func startObjs(decks [MaxSeats][]CardID, ns int, st *Start) (off [MaxSeats]int) {
	if len(st.Seats) > ns {
		panic("kernel: start names more seats than the game has")
	}
	n := 0
	for p := 0; p < ns; p++ {
		off[p] = n
		n += len(decks[p])
	}
	for p := range st.Seats {
		sd := &st.Seats[p]
		seen := make([]bool, len(decks[p]))
		claim := func(i int) {
			if i < 0 || i >= len(seen) {
				panic("kernel: start card index out of range")
			}
			if seen[i] {
				panic("kernel: start names a card twice")
			}
			seen[i] = true
		}
		for _, l := range [][]int{sd.Hand, sd.Battlefield, sd.Graveyard, sd.Exile, sd.Library} {
			for _, i := range l {
				claim(i)
			}
		}
		onBF := make([]bool, len(seen))
		for _, i := range sd.Battlefield {
			onBF[i] = true
		}
		for _, a := range sd.Attach {
			claim(a.Card)
			onBF[a.Card] = true
		}
		for _, l := range [][]int{sd.Tapped, sd.BackFace} {
			for _, i := range l {
				if i < 0 || i >= len(onBF) || !onBF[i] {
					panic("kernel: start tapped/back-face card is not on the battlefield")
				}
			}
		}
		for _, c := range sd.Counters {
			if c.Card < 0 || c.Card >= len(onBF) || !onBF[c.Card] || c.N < 0 {
				panic("kernel: start counter card is not on the battlefield")
			}
		}
	}
	for p := range st.Seats {
		for _, a := range st.Seats[p].Attach {
			if a.HostSeat < 0 || a.HostSeat >= len(st.Seats) || a.HostCard < 0 || a.HostCard >= len(decks[a.HostSeat]) {
				panic("kernel: start attach host out of range")
			}
			h := &st.Seats[a.HostSeat]
			if !startHas(h.Battlefield, a.HostCard) && !startHas(attachCards(h.Attach), a.HostCard) {
				panic("kernel: start attach host is not on the battlefield")
			}
		}
	}
	return off
}

// startRefuseNonPermanents panics on a Battlefield card whose entering face
// (the back face when BackFace names it) is an instant or sorcery: never a
// permanent (CR 304.4, 307.4), as gorge's StartSpec.Validate refuses it.
func startRefuseNonPermanents(t *kop.Table, decks [MaxSeats][]CardID, st *Start) {
	for p := range st.Seats {
		sd := &st.Seats[p]
		for _, i := range sd.Battlefield {
			c := decks[p][i]
			if f := t.Cards[c].Faces; startHas(sd.BackFace, i) && len(f) > 1 {
				c = f[1]
			}
			if t.Cards[c].Types&(kop.TInstant|kop.TSorcery) != 0 {
				panic("kernel: start battlefield card is not a permanent")
			}
		}
	}
}

func startHas(l []int, i int) bool {
	for _, x := range l {
		if x == i {
			return true
		}
	}
	return false
}

func attachCards(a []StartAttach) []int {
	c := make([]int, len(a))
	for i := range a {
		c[i] = a[i].Card
	}
	return c
}

// applyStart is the start's op sequence, run by InitNStart between the deal
// and the first turn:
//
//	toLibrary: each seat's dealt hand goes back to its library end, hand order
//	place:     the listed cards move to their zones (enter/place), a back face
//	           turned before it enters, tapped as it enters
//	counters:  the listed counters are added to their permanents
//	settle:    every placed permanent loses its summoning sickness (CR 302.6)
//	libOrder:  each seat's library is rebuilt (libOrder)
//	life:      each seat's life total is written
//
// then npend = 0: no trigger queued by the setup survives it (D4). It writes
// existing fields only.
func (s *State) applyStart(decks [MaxSeats][]CardID, st *Start) {
	if s.mullMax != 0 {
		panic("kernel: a scripted start needs Mulligans 0")
	}
	ns := int(s.nSeats())
	off := startObjs(decks, ns, st)
	k := s.k()
	startRefuseNonPermanents(k.t, decks, st)
	for p := range st.Seats {
		s.startToLibrary(uint8(p))
	}
	for p := range st.Seats {
		sd := &st.Seats[p]
		pp := uint8(p)
		for _, i := range sd.Hand {
			o := Obj(off[p] + i)
			s.detach(o)
			s.place(o, kop.ZoneHand, false)
		}
		for _, i := range sd.Battlefield {
			s.startEnter(k, Obj(off[p]+i), pp, sd, i)
		}
		for _, i := range sd.Graveyard {
			o := Obj(off[p] + i)
			s.detach(o)
			s.place(o, kop.ZoneGraveyard, false)
		}
		for _, i := range sd.Exile {
			o := Obj(off[p] + i)
			s.detach(o)
			s.place(o, kop.ZoneExile, false)
		}
	}
	for p := range st.Seats {
		sd := &st.Seats[p]
		for _, a := range sd.Attach {
			host := s.objs[Obj(off[a.HostSeat]+a.HostCard)].slot
			s.enterOnto(host)
			s.startEnter(k, Obj(off[p]+a.Card), uint8(p), sd, a.Card)
			s.g.entHost = 0
		}
	}
	for p := range st.Seats {
		sd := &st.Seats[p]
		for _, c := range sd.Counters {
			sl := s.objs[Obj(off[p]+c.Card)].slot
			s.touchSl(sl)
			if c.N > 0 && !s.perms[sl].addCtrN(s, int(c.Kind), int(c.N)) {
				s.ctrStall()
			}
		}
		s.startLibOrder(uint8(p), off[p], sd.Library)
		if sd.Life != nil {
			s.pl[p].life = int16(*sd.Life)
		}
	}
	s.npend = 0
}

// startToLibrary returns seat p's dealt hand to the end of its library, in
// hand order (the mulligan return's mover, nextMullDeclarer).
func (s *State) startToLibrary(p uint8) {
	pl := &s.pl[p]
	for pl.handN > 0 {
		o := pl.hand[0]
		s.detach(o)
		s.objs[o].of &^= ofInHand
		s.objs[o].ver++
		s.objs[o].zone = kop.ZoneLibrary
		pl.libBottom(o)
	}
}

// startEnter puts o (deck card i of seat p, in the library) onto p's
// battlefield: the back face is turned first, as gorge's FlipFace rides ahead
// of the MoveZone, then it is tapped as it enters, and settled.
func (s *State) startEnter(k *prep, o Obj, p uint8, sd *StartSeat, i int) {
	s.detach(o)
	if startHas(sd.BackFace, i) {
		s.turnFace(k, o, 1)
	}
	sl := s.enter(k, o, p, startHas(sd.Tapped, i), kop.ZoneLibrary)
	s.perms[sl].flags &^= pfSick // settle: as if there since before the game
}

// startLibOrder rebuilds seat p's library: the listed cards top first, then
// the cards left after the deal and the placements in their current order
// (the post-shuffle order minus every placed object, then the returned
// unlisted opening-hand cards in hand order, which startToLibrary appended).
func (s *State) startLibOrder(p uint8, off int, top []int) {
	pl := &s.pl[p]
	var buf [MaxDeck + 8]Obj
	n := 0
	for _, i := range top {
		buf[n] = Obj(off + i)
		n++
	}
	for _, o := range pl.lib[pl.libLo:pl.libHi] {
		if !startHas(top, int(o)-off) {
			buf[n] = o
			n++
		}
	}
	if n != pl.libLen() {
		panic("kernel: start library card is not in the library")
	}
	for _, i := range top {
		s.touch(Obj(off + i))
	}
	copy(pl.lib[:], buf[:n])
	pl.libLo, pl.libHi = 0, uint8(n)
}
