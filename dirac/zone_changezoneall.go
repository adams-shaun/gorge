package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCzall3Off is the mutation switch: the ForgetOther
// sweep keeps the old remembered set, and the until-leaves sweep records no
// links (its exiled cards never return).
var DebugCzall3Off bool

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
		for p := uint8(0); p < 2; p++ {
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
		for p := uint8(0); p < 2; p++ {
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
	if in.B.N == int16(kop.ZoneHand) {
		return kop.ZoneHand
	}
	return kop.ZoneExile
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

func (c *czallPend) place(s *State) {
	for p := 0; p < 2; p++ {
		if c.n[p] > 0 {
			s.libTopInOrder(c.list[p][:c.n[p]], c.from[p][:c.n[p]])
		}
	}
}
