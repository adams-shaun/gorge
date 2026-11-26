package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The Bend or Break pair inside a RepeatEach player body (dirac/kgen
// lower_twopiles.go twoPilesEach): the repeated player's opponent choice
// (ExtChoosePlayerEach) feeds a battlefield pile split whose separator is the
// repeated player and whose chooser is that opponent (ExtTwoPilesEach, gorge
// effTwoPiles' battlefield arm over the remembered player).

// choosePlayerEachOp is ExtChoosePlayerEach (gorge effChoosePlayer with
// Defined$ Remembered, Choices$ Player.OpponentOf Remembered, inside a
// RepeatEach player body): the repeated player (s.g.eachPl, the loop's
// binding) chooses one of their opponents, the pool of s.oppsFrom over them
// (gorge pools AliveFrom(chooser) matching Player.OpponentOf Remembered).
// With a single opponent the oracle answers the ask in place
// (forcedPlayerChoice), so the kernel records it unasked; with none gorge
// records an empty choice (choiceRecord drops the old player entries), which
// the readers find nothing in. The choice rides res.chPl (ExtChoosePlayerMid's
// slot), which the chain's ExtTwoPilesEach reads as its chooser.
//
//kmanifest:ext ExtChoosePlayerEach
func (s *State) choosePlayerEachOp(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.g.eachPl
	if p == 255 {
		return true, true
	}
	q := s.oppsFrom(p)
	switch {
	case q.n == 1:
		s.res.chPl = q.s[0] + 1
	case q.n == 0:
		s.res.chPl = 0
	default:
		// the raChoosePlayerMid ask poses the same pool: one "player" option
		// per opponent of askP in AliveFrom order (chosenPlayerMidOptions)
		s.res.ask, s.res.askP = raChoosePlayerMid, p
		return false, true
	}
	return true, true
}

// pilesEachOp is ExtTwoPilesEach (gorge effTwoPiles' battlefield arm with
// Defined$ Remembered: the pool is the repeated player's battlefield
// permanents matching F, read as the effect starts; nothing leaves while the
// asks are open). The separator is the repeated player, the chooser the
// opponent the chain's ExtChoosePlayerEach chose (gorge twoPilesPlayer's
// ChosenPlayer read; none stops the shape with no ask, as gorge's
// unresolvable Chooser$ does). They pick pile A one permanent at a time
// (raPileSplitSac, ChGenPileSplit: gorge's Min 0 subset ask, Done at every
// step, picking the whole pool ends it); a one-permanent pool poses no split
// (pile A is that permanent, as gorge) and the chooser still picks
// (raPileChooseSac). Pile A rides g.sacM and the pool player g.pickP, both
// cleared at the end; the destroy / tap of the piles runs in the pick's
// answer (applyAskPiles).
//
//kmanifest:ext ExtTwoPilesEach
func (s *State) pilesEachOp(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.g.eachPl
	if p == 255 || s.resChosenPlayer() == 255 {
		return true, true
	}
	pool := s.pilesPool(k, in, p)
	if !pool.any() {
		return true, true // gorge returns on an empty set: no ask
	}
	s.g.pickP, s.g.sacM = p, slotMask{}
	if !pool.many() {
		// gorge poses no split for a one-card set: pile A is it, the chooser
		// still picks (gorge effTwoPiles)
		s.g.sacM = pool
		s.res.ask, s.res.askP = raPileChooseSac, s.resChosenPlayer()
		return false, true
	}
	s.res.ask, s.res.askP = raPileSplitSac, p
	return false, true
}

// pilesEachDestroy is the chosen pile's ChosenPile$ body, DestroyAll
// Land.IsRemembered (gorge twoPilesRunPile: the body resolves with the pile as
// the fresh Ctx's Remembered, so the victims are the pile's members still on
// the battlefield that match F): OpDestroyAll's passes over them (destroy
// line, regeneration, shield counters and umbra, CR 122.1c then 702.89a).
func (s *State) pilesEachDestroy(k *prep, in *kop.Instr, pile slotMask) {
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := pile; m.any(); m = m.rest() {
		sl := m.first()
		if s.occ.has(sl) && s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) &&
			s.kw(k, sl)&kop.KwIndestructible == 0 {
			hit = hit.with(sl)
		}
	}
	s.destroyAllPasses(k, hit)
}

// destroyAllPasses is OpDestroyAll's passes over a matched hit (the destroy
// line, regeneration, shield counters and umbra, CR 122.1c then 702.89a;
// gorge effDestroyAll; kw_umbraarmor.go). OpDestroyAll's interp case calls it
// with the sweep's hit; the pile split (pilesEachDestroy) with its pile's.
func (s *State) destroyAllPasses(k *prep, hit slotMask) {
	hit = s.destroyLineAll(k, hit, true)
	if s.anyRegen(hit) {
		hit = s.regenAll(hit)
	}
	s.shieldUmbraAllLeave(k, hit)
}

// pilesEachTap is the unchosen pile's UnchosenPile$ body, TapAll
// Land.IsRemembered (gorge twoPilesRunPile): OpTapAll's walk over the pile --
// the untapped members still matching F, seat 0's battlefield first in entry
// order, each tap firing its Taps triggers as it happens, one tapping action
// (trig_tapall.go).
func (s *State) pilesEachTap(k *prep, in *kop.Instr, pile slotMask) {
	live := pile.and(s.occ)
	var ord [MaxPerm]Slot
	n := 0
	for seat := uint8(0); seat < s.nSeats(); seat++ {
		if s.out(seat) {
			continue
		}
		n = s.battlefieldOrder(seat, live.and(s.ctrlM[seat]), ord[:], n)
	}
	src := s.objRef(k, kop.RefSelf)
	tb := s.tapBatchOpen(k) // one TapAll resolution is one tapping action (trig_tapall.go)
	defer s.tapBatchClose(tb)
	for _, sl := range ord[:n] {
		p := &s.perms[sl]
		if p.flags&pfTapped != 0 || !s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) {
			continue
		}
		p.flags |= pfTapped
		s.touchSl(sl)
		if s.subs[kop.TrigTaps].any() {
			s.fireTapsBy(k, sl, s.effTapper(sl))
		}
		s.markTapped(k, sl)
	}
}
