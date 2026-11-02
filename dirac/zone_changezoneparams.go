package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the Mill / ChangeZone choice params gorge reads
// (Mill Optional$, Attacking$, DifferentNames$, ThisDefinedAndTgts$ Self).
// Cold and out of line: each hot site tests one flag bit and calls in here.

// raGenMayMill is OpMill with MvMayMill: the acting player's yes/no before
// milling (gorge effMill's mill_optional, ChGenConfirm; yes then no). The
// player asked is s.g.pickP, the player after it s.g.pickZ (255: none) and
// the amount s.g.pickLeft.
const raGenMayMill = raGenManifestDread + 3

// mayMill starts the per-player mill_optional asks. gorge asks each acting
// player in turn, when the amount is above zero, before the MillProposal
// replacement and even with an empty library; false: the first ask is
// posed.
//
//go:noinline
func (s *State) mayMill(p1, p2 uint8, n int) bool {
	if p1 == 255 {
		p1, p2 = p2, 255
	}
	if p1 == 255 {
		return true
	}
	if n > 255 {
		n = 255
	}
	s.g.pickP, s.g.pickZ, s.g.pickLeft = p1, p2, uint8(n)
	s.res.ask, s.res.askP = raGenMayMill, p1
	return false
}

// millTop mills player p's top n cards one at a time (OpMill's loop).
func (s *State) millTop(p uint8, n int, remember bool) {
	pl := &s.pl[p]
	for i := 0; i < n && pl.libLo < pl.libHi; i++ {
		o := pl.lib[pl.libLo]
		pl.libLo++
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFrom(o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
		if remember {
			s.remember(o)
		}
	}
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
		p, next := s.g.pickP, s.g.pickZ
		if a.Mode() != 0 {
			s.millTop(p, int(s.g.pickLeft), s.prog(k)[s.res.pc].C&kop.MvRemember != 0)
		}
		if next != 255 {
			s.g.pickP, s.g.pickZ = next, 255
			s.res.ask, s.res.askP = raGenMayMill, next
			return true
		}
		s.continueRes(k, 0)
	default:
		return false
	}
	return true
}

// enterAttacking is Attacking$ True on a card just put onto the battlefield
// at sl under ctrl: it enters attacking the opponent (gorge TokenAttacks
// fold: IsAttacking, Attacking = the defender; nothing is declared, so no
// Attacks trigger fires, CR 508.4) when the step is declare attackers
// through end of combat and ctrl is the active player (gorge
// enteringDefenderFallback; every lowered carrier is an attack trigger, whose
// captured defender is the same opponent). Outside that window it enters and
// does not attack. With a planeswalker or battle in play gorge asks the
// CR 508.4 defender (entering_defender): the game stalls instead.
//
//go:noinline
func (s *State) enterAttacking(k *prep, sl Slot, ctrl uint8) {
	if sl == NoSlot || !s.occ.has(sl) || ctrl != s.active ||
		s.step < kop.StepAttackers || s.step > kop.StepEndCombat {
		return
	}
	if s.enteringCandidates(k, ctrl) {
		s.capFull(StallEnterDefender)
	}
	pp := &s.perms[sl]
	pp.flags |= pfAttacking
	pp.atk = PlayerTarget0 + (1 - ctrl&1)
	pp.bord = s.nAttackers
	s.nAttackers++
	s.touchSl(sl)
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
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
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
	var seen [64]CardID
	n := 0
	for _, o := range pl.lib[pl.libLo:pl.libHi] {
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
	ask := s.execReturn(k, &in2, host)
	if !ask && (z1 == kop.ZoneNone || o2 != NoObj && s.objs[o2].zone == z1) {
		in2.Obj = kop.RefSelf
		ask = s.execReturn(k, &in2, host)
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
	s.moveRef(k, kop.RefSelf, kop.ZoneLibrary, false)
	s.batchClose(old)
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
		for pp := uint8(0); pp < 2; pp++ {
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
