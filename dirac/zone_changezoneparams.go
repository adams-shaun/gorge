package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the Mill / ChangeZone choice params gorge reads
// (Mill Optional$, Attacking$, DifferentNames$, ThisDefinedAndTgts$ Self).
// Cold and out of line: each hot site tests one flag bit and calls in here.

// raGenMayMill is OpMill with MvMayMill: the acting player's yes/no before
// milling (gorge effMill's mill_optional, ChGenConfirm; yes then no). The
// player asked is s.g.pickP, the players after it s.g.pickZ (a packed seat
// chain, seatsPack; 255: none) and the amount s.g.pickLeft.
const raGenMayMill = raGenManifestDread + 3

// mayMill starts the per-player mill_optional asks. gorge asks each acting
// player in turn, when the amount is above zero, before the MillProposal
// replacement and even with an empty library; false: the first ask is
// posed.
//
//go:noinline
func (s *State) mayMill(q seatSeq, n int) bool {
	if q.n == 0 {
		return true
	}
	if n > 255 {
		n = 255
	}
	p1 := q.s[0]
	s.g.pickP, s.g.pickZ, s.g.pickLeft = p1, seatsPack(q, 1), uint8(n)
	s.res.ask, s.res.askP = raGenMayMill, p1
	return false
}

// millTop mills player p's top n cards one at a time (OpMill's loop).
func (s *State) millTop(p uint8, n int, c int) {
	pl := &s.pl[p]
	for i := 0; i < n && pl.libLo < pl.libHi; i++ {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFrom(o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
		s.millMark(o, c)
	}
}

// millMark records a milled card o in the sets OpMill's flags name: the
// remembered set (MvRemember, RememberMilled$ of Sphinx's Tutelage) and the
// imprinted set (MvImprint, Imprint$ True of Patient Naturalist). The card
// that was milled is marked wherever a replacement sent it (gorge imprints
// the id it milled). All four mill paths call it right after the move.
func (s *State) millMark(o Obj, c int) {
	if c&kop.MvRemember != 0 {
		s.remember(o)
	}
	if c&kop.MvImprint != 0 {
		s.imprint(o)
	}
}

// millParkX packs OpMill's mark flags into a parked mill's x byte (bit 0
// MvRemember, bit 1 MvImprint) and millParkFlags unpacks it.
func millParkX(c int) (x uint8) {
	if c&kop.MvRemember != 0 {
		x = 1
	}
	if c&kop.MvImprint != 0 {
		x |= 2
	}
	return x
}

func millParkFlags(x uint8) (c int) {
	if x&1 != 0 {
		c |= kop.MvRemember
	}
	if x&2 != 0 {
		c |= kop.MvImprint
	}
	return c
}

// askOptionsCzParams builds this file's resolution asks' options; false:
// not one of them.
//
//go:noinline
func (s *State) askOptionsCzParams(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenMayMill:
		d.Sub = ChGenConfirm // gorge's mill_optional options: yes, no
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenEnterDefender:
		d.Sub = ChGenEnterDefender
		var buf [MaxOptions]uint8
		n := s.enteringDefenderCands(k, s.res.askP, &buf)
		if MaxSeats > 2 && s.res.e.kind == seTrigger {
			if fb := s.enterDefenderFallback(k, s.res.askP); fb != NoTarget {
				n = enterDefenderOrder(&buf, n, fb) // gorge chooseEnteringDefender: fallback first
			}
		}
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
	default:
		return false
	}
	return true
}

// applyAskCzParams applies an answer to this file's resolution asks; false:
// not one of them.
//
//go:noinline
func (s *State) applyAskCzParams(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenMayMill:
		s.res.ask = raNone
		p := s.g.pickP
		if a.Mode() != 0 {
			s.millTop(p, int(s.g.pickLeft), int(s.prog(k)[s.res.pc].C))
		}
		if next := seatsPop(&s.g.pickZ); next != 255 {
			s.g.pickP = next
			s.res.ask, s.res.askP = raGenMayMill, next
			return true
		}
		s.continueRes(k, 0)
	case raGenEnterDefender:
		s.res.ask = raNone
		sl := Slot(s.res.chosen)
		if sl == NoSlot { // a TokenAttacking$ True mint's pick, asked before it (filter_combatwords.go)
			s.g.tokDef, s.g.tokDefSet = a.Source(), true
			s.runProg(k) // re-run the mint instruction with the answer
			return true
		}
		if s.occ.has(sl) {
			s.setEnterAttacker(k, sl, a.Source())
		}
		if s.unparkTail(k) {
			return true
		}
		s.continueRes(k, 0) // the ask was posed after the move: the instruction is done
	default:
		return false
	}
	return true
}

// raGenEnterDefender is an Attacking$ True entry's CR 508.4 defender pick
// (gorge resolveEnteringDefender's entering_defender, chooseEnteringDefender):
// res.chosen is the entered permanent's slot, and the options are the
// defenders in gorge's order (enteringDefenders, the fallback -- the opponent
// player -- first).
const raGenEnterDefender = raGenManifestDread + 75

// enterAttacking is Attacking$ True on a card just put onto the battlefield
// at sl under ctrl: it enters attacking (gorge TokenAttacks fold: IsAttacking,
// Attacking = the defender; nothing is declared, so no Attacks trigger fires,
// CR 508.4) when the step is declare attackers through end of combat and ctrl
// is the active player (gorge enteringDefenderFallback). Outside that window
// it enters and does not attack. With one CR 508.4 defender that is the
// opponent player; with two or more gorge asks which (CR 508.4), so the ask is
// posed and true returned -- the caller must suspend, parking its remaining
// work behind it. Every lowered carrier is an attack trigger whose captured
// defender is the same opponent, so the fallback is the list's first entry.
//
//go:noinline
func (s *State) enterAttacking(k *prep, sl Slot, ctrl uint8) bool {
	if sl == NoSlot || !s.occ.has(sl) || ctrl != s.active ||
		s.step < kop.StepAttackers || s.step > kop.StepEndCombat {
		return false
	}
	var buf [MaxOptions]uint8
	n := s.enteringDefenderCands(k, ctrl, &buf)
	if n == 0 {
		return false // no CR 508.4 defender: it enters but does not attack
	}
	if s.g.tokDefSet { // asked before the move (returnDefenderAsk)
		s.setEnterAttacker(k, sl, s.g.tokDef)
		return false
	}
	if n > 1 {
		s.res.ask, s.res.askP, s.res.chosen = raGenEnterDefender, ctrl, uint8(sl)
		return true
	}
	s.setEnterAttacker(k, sl, buf[0])
	return false
}

// setEnterAttacker applies the attacking entry to the permanent at sl, the
// fold for Attacking$ True (CR 508.4).
func (s *State) setEnterAttacker(k *prep, sl Slot, atk uint8) {
	pp := &s.perms[sl]
	if pp.flags&pfAttacking != 0 {
		pp.atk = atk // already attacking (Kira's entry beside a token's own): no second slot
		s.touchSl(sl)
		return
	}
	pp.flags |= pfAttacking
	pp.atk = atk
	pp.bord = s.nAttackers
	s.nAttackers++
	s.touchSl(sl)
}

// enteringDefenderCands lists the CR 508.4 defenders a creature entering
// attacking under ctrl may attack, in gorge enteringDefenders order: the
// opponent players first (the fallback chooseEnteringDefender moves to the
// front), then each face-up planeswalker the opponent controls, then every
// battle it protects (gorge battleDefenders, CR 310.7), each seat's
// battlefield in entry order.
func (s *State) enteringDefenderCands(k *prep, ctrl uint8, buf *[MaxOptions]uint8) int {
	n := 0
	for opp := uint8(0); opp < s.nSeats(); opp++ {
		if opp != ctrl && !s.pl[opp].lost {
			buf[n] = PlayerCode(opp) // each opponent player, AliveFrom(0)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	var order [MaxPerm]Slot
	for opp := uint8(0); opp < s.nSeats(); opp++ {
		if opp == ctrl || s.pl[opp].lost {
			continue
		}
		var m slotMask
		for q := s.ctrlM[opp]; q.any(); q = q.rest() {
			if sl := q.first(); s.typesGen(k, sl)&kop.TPlaneswalker != 0 && !s.faceDown(k, sl) {
				m = m.with(sl)
			}
		}
		c := s.battlefieldOrder(opp, m, order[:], 0)
		for _, sl := range order[:c] {
			if n < MaxOptions {
				buf[n] = uint8(s.perms[sl].obj)
				n++
			}
		}
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var bm slotMask
		for q := s.ctrlM[p]; q.any(); q = q.rest() {
			if sl := q.first(); s.battleAttackable(k, sl, ctrl) {
				bm = bm.with(sl)
			}
		}
		c := s.battlefieldOrder(p, bm, order[:], 0)
		for _, sl := range order[:c] {
			buf[n] = uint8(s.perms[sl].obj)
			n++
		}
	}
	return n
}

// parkEnterDefender parks a search's or dig's remaining work (res.left = kind)
// when the entry just made posed the CR 508.4 defender ask; the answer runs it
// (unparkTail), exactly as a queued as-enters ask parks it. false: no ask.
func (s *State) parkEnterDefender(kind uint8) bool {
	if s.res.ask != raGenEnterDefender {
		return false
	}
	s.res.left = kind
	return true
}

// differentNameCands is searchCands' library walk under DifferentNames$ True:
// a card sharing a name with one already found is no longer a candidate
// (gorge zone_search.go tags each option Group = name; Forge's
// ChangeZoneEffect filters fetchList by sharesNameWith one pick at a time).
// A name is a card id here: one CardDef per printed name.
//
//go:noinline
func (s *State) differentNameCands(k *prep, in *kop.Instr, d *Decision, list []Obj) {
	p := s.g.pickP
next:
	for _, o := range list {
		if s.picksHas(o) {
			continue
		}
		for i := 0; i < int(s.g.nlook); i++ {
			if s.objs[s.resArm().look[i]].card == s.objs[o].card {
				continue next
			}
		}
		if !s.tokenLeftBattlefield(o) && s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) { // a token off the battlefield is no card (CR 111.8)
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}

// clampDistinctNames caps a DifferentNames$ search's count at the number of
// distinct names among its eligible cards (gorge zone_search.go: max is
// clamped to the distinct names of the pool).
//
//go:noinline
func (s *State) clampDistinctNames(k *prep, in *kop.Instr) {
	p := s.g.pickP
	pl := &s.pl[p]
	// An OriginAlternative$ search has not held its zone election yet, so
	// the cap counts every zone it may search: the union over the elected
	// set can only be smaller, and an early pick that repeats a name ends
	// the search anyway.
	var lists [3][]Obj
	if altOp(in) {
		lists = s.altZoneLists(in) // zone_originalternative.go
	} else {
		lists[0] = pl.lib[pl.libLo:pl.libHi]
	}
	var seen [64]CardID
	n := 0
	for zi := range lists {
		for _, o := range lists[zi] {
			c := s.objs[o].card
			dup := false
			for i := 0; i < n; i++ {
				if seen[i] == c {
					dup = true
					break
				}
			}
			if dup || !s.matchCardGen(k, in.F, c, p, s.res.e.ctrl, o) {
				continue
			}
			if n < len(seen) {
				seen[n] = c
				n++
			}
		}
	}
	if int(s.g.pickLeft) > n {
		s.g.pickLeft = uint8(n)
	}
}

// returnWithSelf is OpReturnEx with MvAlsoSelf (ThisDefinedAndTgts$ Self,
// gorge effChangeZone: the extra objects are appended after the chosen
// targets, so the target returns first, then the source). More than one
// object onto the battlefield enter in one zone batch (BeginZoneBatch), so a
// "one or more" entry trigger fires once for both.
//
//go:noinline
func (s *State) returnWithSelf(k *prep, in *kop.Instr, host Slot) bool {
	in2 := *in
	in2.C &^= kop.MvAlsoSelf << 3
	// each object must still be in Origin$ (gorge changeZoneMoveSkip): the
	// source in the zone the target started in
	o1, o2 := s.objRefObj(in.Obj), s.objRefObj(kop.RefSelf)
	z1 := kop.ZoneNone
	if o1 != NoObj {
		z1 = s.objs[o1].zone
	}
	old := s.batchOpen()
	ask := s.execReturnObj(k, &in2, host, s.returningObj(&in2))
	if !ask && (z1 == kop.ZoneNone || o2 != NoObj && s.objs[o2].zone == z1) {
		in2.Obj = kop.RefSelf
		ask = s.execReturnObj(k, &in2, host, s.returningObj(&in2))
	}
	s.batchClose(old)
	return ask
}

// toLibraryShuffleSelf is OpToLibrary with kop.LibTopShuffleSelf: the target,
// then the source, go on top of their owners' libraries (one departure
// batch), then each distinct owner's library is shuffled once in that order,
// even when an object did not move (gorge objectPathShuffleOwners, CR
// 701.24c).
//
//go:noinline
func (s *State) toLibraryShuffleSelf(k *prep, in *kop.Instr) {
	o1, o2 := s.objRefObj(in.Obj), s.objRefObj(kop.RefSelf)
	old := s.batchOpen()
	s.moveRef(k, in.Obj, kop.ZoneLibrary, false)
	if s.res.ask == raGenCmdZone {
		s.batchClose(old)
		return // a CR 903.9b commander move parked: the source and the shuffle wait
	}
	s.moveRef(k, kop.RefSelf, kop.ZoneLibrary, false)
	s.batchClose(old)
	if s.res.ask == raGenCmdZone {
		return
	}
	p1 := uint8(255)
	if o1 != NoObj {
		p1 = s.objs[o1].owner()
		s.shuffleLib(p1)
	}
	if o2 != NoObj && s.objs[o2].owner() != p1 {
		s.shuffleLib(s.objs[o2].owner())
	}
}

// fireGraveAttackers queues the graveyard AttackersDeclared lines (Persistent
// Marshstalker's "whenever you attack with one or more Rats, ... return
// this card from your graveyard"), after the battlefield listeners like the
// phase triggers' graveyard walk (firePhaseGen).
//
//go:noinline
func (s *State) fireGraveAttackers(k *prep, atk []Slot) {
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigAttackersDeclared {
						continue
					}
					l := trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					n := int16(s.nAttackers)
					if a.Trig.Filter != 0 {
						n = s.attackersMatching(k, l, a, atk)
					}
					if n > 0 {
						s.fireAtkDecl(k, l, a, n, atk) // trig_attackers.go
					}
				}
			}
		}
	}
}
