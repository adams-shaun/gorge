package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Extort (CR 702.101a): "Whenever you cast a spell, you may pay {W/B}. If you
// do, each opponent loses 1 life and you gain that much life." kgen lowers the
// printed K:Extort expansion's SpellCast trigger (and the latent trigger a
// layer-6 AddKeyword$ Extort grant carries) to a program holding one
// OpExt ExtExtort (kop/ext.go). The trigger is an ordinary stack object; this
// op poses the pay/decline election when it resolves, then drains.
//
// The election is gorge effExtort's two KModes options, both always offered
// even when the pool has neither colour: option 0 pays, option 1 declines.
// The generated ChGenPayMode choice is the same canonical pair (mode 1 = pay,
// mode 0 = decline; dirac/canon/translate.go), so a lockstep compares it
// decision for decision.

// askOptionsExtort builds the Extort election; false: not this ask.
//
//go:noinline
func (s *State) askOptionsExtort(k *prep, d *Decision) bool {
	if s.res.ask != raGenExtort {
		return false
	}
	d.Sub = ChGenPayMode
	d.add(MakeAction(AkChoose, 0, 0, 1)) // pay {W/B} (gorge option 0)
	d.add(MakeAction(AkChoose, 0, 0, 0)) // decline (gorge option 1)
	return true
}

// applyAskExtort applies the election and resumes the resolution. Paying
// charges one W or B from the pool; the drain runs only on that charge, as
// gorge's extortAnswerRecord reports it. A pool with neither colour is a
// decline that drains nothing, exactly PayExtortPip's false.
//
//go:noinline
func (s *State) applyAskExtort(k *prep, a Action) bool {
	if s.res.ask != raGenExtort {
		return false
	}
	p := s.res.askP
	s.res.ask = raNone
	if a.Mode() == 1 && s.payExtortPip(p) {
		s.extortDrain(k, p)
	}
	s.continueRes(k, 0)
	return true
}

// payExtortPip charges Extort's {W/B} hybrid pip: one fresh W then B before
// either persistent unit, the ordinary-first order gorge pay.PayExtortPip
// reads (state.Player.PersistentMana is the tally, not a separate pool). The
// kernel's payFromPool follows the same spend through artSync/persistSync, so
// the persistent tally stays min(tally, pool).
func (s *State) payExtortPip(p uint8) bool {
	pl := &s.pl[p]
	pm := &s.pm[SeatIx(p)]
	var c int
	switch {
	case int(pl.pool[kop.W])-int(pm[kop.W]) > 0:
		c = int(kop.W)
	case int(pl.pool[kop.B])-int(pm[kop.B]) > 0:
		c = int(kop.B)
	case pm[kop.W] > 0:
		c = int(kop.W)
	case pm[kop.B] > 0:
		c = int(kop.B)
	default:
		return false
	}
	var cost kop.ManaCost
	cost.Pips[c] = 1
	s.payFromPool(p, &cost, 0)
	return true
}

// extortDrain is a paid Extort's drain: each opponent still in the game loses
// 1 life, then the controller gains one per opponent. The walk starts after
// the controller and wraps (gorge extortDrain over AliveFrom(controller)),
// skipping the controller and any lost seat.
func (s *State) extortDrain(k *prep, p uint8) {
	n := 0
	seats := s.nSeats()
	for i := uint8(1); i < seats; i++ {
		q := (p + i) % seats
		if s.pl[q].lost {
			continue
		}
		s.loseLife(k, q, 1)
		n++
	}
	if n > 0 {
		s.gainLife(k, p, n)
	}
}
