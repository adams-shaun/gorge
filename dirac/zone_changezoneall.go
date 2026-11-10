package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCzall3Off is the mutation switch: the ForgetOther
// sweep keeps the old remembered set, and the until-leaves sweep records no
// links (its exiled cards never return).
var DebugCzall3Off bool

// czallBattleHalf is the battlefield scan of an OpChangeZoneAllEx sweep:
// one simultaneous departure (CR 603.10a), seat 0's battlefield list then
// seat 1's (gorge effChangeZoneAll: AliveFrom(0) inside one departure batch).
// The plain battlefield sweep runs it alone; the Battlefield,Graveyard form
// (kop.CzBattleGrave) runs it before its graveyard half. Cold and out of
// line: the hot path reaches it once per sweep, not per permanent.
//
//go:noinline
func (s *State) czallBattleHalf(k *prep, in *kop.Instr, only uint8, src Slot, z kop.Zone, mv int16) {
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if (only == 255 || s.perms[sl].ctrl == only) && s.matchPermGen(k, in.F, sl, s.res.e.ctrl, src, layerFull) {
			hit = hit.with(sl)
		}
	}
	if mv&kop.MvRemember != 0 {
		s.rememberLeaveAll(k, hit, z) // zone_rememberparams.go
		return
	}
	s.leaveAll(k, hit, z)
}

// czall3 runs the two OpChangeZoneAllEx forms execChangeZoneAll hands off
// behind its one guard (cold):
//   - MvForgetOther (ForgetOtherRemembered$, gorge effChangeZoneAll): the
//     matched set is copied out first (the IsRemembered filter reads the
//     memory the clear drops), the remembered set is cleared (both halves),
//     then the copy moves in scan order, MvRemember re-remembering each. Kept
//     to the non-battlefield, non-library, entry-rider-free sweeps kgen lowers.
//   - CzUntilLeaves (Duration$ UntilHostLeavesPlay): a battlefield exile sweep
//     that does nothing once the source has left (gorge exileHostGoneFor),
//     then links each exiled non-token permanent to the source, in sweep
//     order (gorge recordExileReturnFor); leaveGen returns them in link order.
//
//go:noinline
func (s *State) czall3(k *prep, in *kop.Instr, mv int16, z kop.Zone) {
	e := &s.res.e
	fl := k.t.Filters[in.F]
	if mv&kop.MvForgetOther == 0 {
		src := s.objRef(k, kop.RefSelf)
		if src == NoSlot {
			return
		}
		var hit slotMask
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPermGen(k, in.F, sl, e.ctrl, src, layerFull) {
				hit = hit.with(sl)
			}
		}
		var order [MaxPerm]Slot
		var objs [MaxPerm]Obj
		n := 0
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), order[:], n)
		}
		for i := 0; i < n; i++ {
			objs[i] = s.perms[order[i]].obj
		}
		s.leaveAll(k, hit, z)
		if DebugCzall3Off {
			return
		}
		for i := 0; i < n; i++ {
			o := objs[i]
			if s.objs[o].zone != kop.ZoneExile || s.objs[o].of&ofToken != 0 {
				continue // a token ceases to exist: nothing to return
			}
			if s.g.nlinks >= uint8(len(s.g.links)) {
				s.capFull(StallLinks)
				return
			}
			l := &s.g.links[s.g.nlinks]
			l.src, l.sver, l.obj = e.obj, s.objs[e.obj].ver, o
			s.g.nlinks++
		}
		return
	}
	var snap [MaxDeck * 2]Obj
	n := 0
	nz, zbase := 1, fl.Zone
	if mv&kop.MvGraveExile != 0 {
		nz, zbase = 2, kop.ZoneGraveyard
	}
	for zi := 0; zi < nz; zi++ {
		zsrc := zbase
		if zi == 1 {
			zsrc = czallSecondZone(in)
		}
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			pl := &s.pl[p]
			var list [MaxDeck]Obj
			nl := 0
			switch zsrc {
			case kop.ZoneGraveyard:
				nl = copy(list[:], pl.grave[:pl.graveN])
			case kop.ZoneExile:
				nl = copy(list[:], pl.exile[:pl.exileN])
			case kop.ZoneHand:
				nl = copy(list[:], pl.hand[:pl.handN])
			}
			for i := 0; i < nl && n < len(snap); i++ {
				if s.matchCardGen(k, in.F, s.objs[list[i]].card, p, e.ctrl, list[i]) {
					snap[n] = list[i]
					n++
				}
			}
		}
	}
	if !DebugCzall3Off {
		s.g.nrem, s.g.remCnt = 0, 0 // both halves (gorge forgetOther)
	}
	var eb entBatch
	for i := 0; i < n; i++ {
		o := snap[i]
		if s.tokenLeftBattlefield(o) || s.instantStays(k, o, z) {
			continue // CR 111.8, 304.4, 307.4: gorge's emitMove guard
		}
		if z == kop.ZoneBattlefield && s.occ.full() {
			s.fireEntries(k, &eb)
			s.stall(StallBoard)
			return
		}
		p := s.objs[o].owner()
		from := s.detach(o)
		if mv&kop.MvRemember != 0 {
			s.remember(o)
		}
		if z != kop.ZoneBattlefield {
			s.placeFromK(k, o, z, false, from, false)
			continue
		}
		if k.gen != nil && k.gen.anyMayLimit {
			s.countMayPlayLand(k, o, from)
		}
		ctrl := p
		if mv&kop.MvUnderYourControl != 0 {
			ctrl = e.ctrl
		}
		sl := s.enterHeld(k, &eb, o, ctrl, mv&kop.MvTapped != 0, from)
		if from == kop.ZoneGraveyard {
			s.fireLeavesGraveyardSelf(k, sl, o)
		}
	}
	s.fireEntries(k, &eb)
}

// overrideRemembered is OpOverride over a remembered set of several objects
// (Random Encounter's "they gain haste": Pump Defined$ Remembered, gorge
// applies the grant to every remembered object, in set order). The set is
// copied out first: applying an override can fire nothing that remembers.
//
//go:noinline
func (s *State) overrideRemembered(k *prep, in *kop.Instr) {
	e := &s.res.e
	var objs [maxRem]Obj
	n := copy(objs[:], s.g.rem[:s.g.nrem])
	for i := 0; i < n; i++ {
		o := objs[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		sl := s.objs[o].slot
		switch {
		case k.gen.anyWhileHost && k.t.Overrides[in.C].WhileHost:
			s.applyWhileHost(k, sl, int(in.C), e)
		case in.B.N == kop.OvChosenColor:
			s.ovChosenColor(k, sl, int(in.C), e.card, s.ovDurFl(in.A.N))
		default:
			s.applyOverrideFl(k, sl, int(in.C), e.card, s.ovDurFl(in.A.N))
		}
	}
}

// czallSecondZone decodes OpChangeZoneAllEx's second source zone for a
// MvGraveExile sweep: B.N == 0 is the legacy Graveyard,Exile; B.N ==
// ZoneHand is Graveyard,Hand (Weftwalking). B.N's CzUntilLeaves bit is
// never set on the dual shape.
func czallSecondZone(in *kop.Instr) kop.Zone {
	if in.B.N&kop.CzHandGrave != 0 {
		return kop.ZoneGraveyard // Hand,Graveyard: the hand is scanned first (Echo of Eons)
	}
	switch in.B.N &^ kop.CzShuffleEach {
	case int16(kop.ZoneHand):
		return kop.ZoneHand
	case kop.CzLibrarySideboard:
		return kop.ZoneSideboard // Library,Sideboard (Turtles Forever)
	}
	return kop.ZoneExile
}

// czallFirstZone is the first source zone of a MvGraveExile sweep: the
// graveyard, or the library for Library,Sideboard.
func czallFirstZone(in *kop.Instr) kop.Zone {
	if in.B.N&kop.CzHandGrave != 0 {
		return kop.ZoneHand
	}
	if in.B.N == kop.CzLibrarySideboard {
		return kop.ZoneLibrary
	}
	return kop.ZoneGraveyard
}

// czallPend collects the library-bound cards of a two-zone sweep per owner
// across both zones, so they are placed once in move order (gorge
// libraryOrderPlacement: zone-major moves, then the rest), not
// per zone, which would leave the second zone's cards on top.
type czallPend struct {
	list [2][2 * MaxDeck]Obj
	from [2][2 * MaxDeck]kop.Zone
	n    [2]int
}

func (c *czallPend) add(p uint8, os []Obj, from []kop.Zone) {
	for i := range os {
		if c.n[p] < len(c.list[p]) {
			c.list[p][c.n[p]], c.from[p][c.n[p]] = os[i], from[i]
			c.n[p]++
		}
	}
}

func (c *czallPend) place(s *State, carry *czallCarry) {
	for p := 0; p < 2; p++ {
		if c.n[p] > 0 {
			s.czallPlace(c.list[p][:c.n[p]], c.from[p][:c.n[p]], carry)
		}
	}
}

// czallCarry is the per-owner prefix a library-bound ChangeZoneAll has
// already placed when a CR 903.9b commander move parks the sweep. The
// answer's re-run places the cards after the park on top of that prefix;
// rotating each new group under it restores gorge's move order (the first
// moved on top) without touching the earlier cards again (a second MoveZone
// would make them new objects, CR 400.7). k is carried in the parked ask's
// spare bytes across the answer, so the sweep needs no State field.
type czallCarry struct {
	k [2]int
	// first is the seat whose card moved first plus one (0: none yet), read by
	// a CzShuffleEach sweep to shuffle the receiving libraries in gorge's order.
	// k is at most MaxDeck (100), so bit 7 of each parked byte carries it.
	first uint8
}

// load reads the carried prefix of an answered commander park at the
// instruction's re-run.
func (c *czallCarry) load(pre *preAsk) {
	if pre.kind == akCmdZone && (pre.state == preCmdZoneMove || pre.state == preCmdZoneCmd) {
		c.k[0], c.k[1] = int(pre.val&0x7f), int(pre.val2&0x7f)
		c.first = pre.val>>7 | pre.val2>>7<<1
	}
}

// store writes the carried prefix for the next park.
func (c *czallCarry) store(pre *preAsk) {
	pre.val, pre.val2 = uint8(c.k[0])|c.first&1<<7, uint8(c.k[1])|c.first>>1<<7
}

// noteFirst records o's owner as the first mover of a sweep that shuffles
// the receiving libraries (CzShuffleEach); the first card wins.
func (c *czallCarry) noteFirst(s *State, o Obj) {
	if c.first == 0 {
		c.first = s.objs[o].owner()&1 + 1
	}
}

// shuffleEach is the library-shuffle tail of a CzShuffleEach sweep (gorge
// changeZoneAllShuffleLibraries, scoped): the owner of the first card moved
// shuffles first, then the other seat; with nothing moved, seat 0 then seat 1.
func (c *czallCarry) shuffleEach(s *State) {
	f := uint8(0)
	if c.first == 2 {
		f = 1
	}
	s.shuffleLib(f)
	s.shuffleLib(NextSeat(f))
}

// czallPlace puts os on top of their owners' libraries in move order. When
// the sweep was parked earlier, the owner's new group is rotated under the
// prefix placed before the park, so the moved cards end up in gorge's order
// (libraryOrderPlacement: the first moved on top).
func (s *State) czallPlace(os []Obj, from []kop.Zone, c *czallCarry) {
	if len(os) == 0 {
		return
	}
	s.libTopInOrder(os, from)
	var m [2]int
	for _, o := range os {
		if s.objs[o].zone == kop.ZoneLibrary {
			m[SeatIx(s.objs[o].owner())]++
		}
	}
	for p := 0; p < 2; p++ {
		if m[p] == 0 {
			continue
		}
		if c.k[p] > 0 {
			s.libRotateTop(uint8(p), c.k[p]+m[p], m[p])
		}
		c.k[p] += m[p]
	}
}

// libRotateTop rotates the top total cards of p's library left by m: the
// group just placed moves under the prefix placed before it.
func (s *State) libRotateTop(p uint8, total, m int) {
	pl := &s.pl[p]
	lo := int(pl.libLo)
	var tmp [MaxDeck]Obj
	copy(tmp[:], pl.lib[lo+m:lo+total])
	copy(tmp[total-m:], pl.lib[lo:lo+m])
	copy(pl.lib[lo:lo+total], tmp[:total])
}
