package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A printed trigger's own optional cost with hybrid mana ("you may pay
// {G/W}", Emiel the Blessed; kgen lower_trighybrid.go). gorge's
// triggered-cost window (rules respay.go) offers the pay when the pool or the
// potential bound covers the whole cost, funds it with the planner's witness
// (planHybridCost: the first pip resolution the sources afford) and charges
// it from the pool. Before its pay/decline election it announces each pip
// (rules cumulative.go pipAnnounceAsk, CR 601.2b): "pay_<A>" and
// "pay_<B>" in printed order, with no pool pre-filter -- a face nothing can
// fund ends in the decline-only election. The PayMana instruction carries
// the pips in F beside the life (kop.PayManaF); the announced faces ride
// State.payHyb, and every read of the parked cost folds them in
// (resPayCost, canPayGen, trigCostPotentialPays).

// raGenPayPip: a trigger's own hybrid cost asks the next pip's face
// (ChGenPip, mode the colour) before its raGenPay election.
const raGenPayPip = raGenManifestDread + 30

// startPayPips parks the trigger's own hybrid PayMana ask on its first pip
// announcement; the election's pick state is set as execPay sets it.
func (s *State) startPayPips(p uint8, skip uint16, amt int) bool {
	s.payHyb = 0
	s.g.pickF, s.g.pickZ, s.g.pickLeft, s.g.pickP = skip, kop.PayMana, uint8(amt), p
	s.res.ask, s.res.askP = raGenPayPip, p
	return false
}

// payPipOptions offers the next pip's two faces in printed order.
func (s *State) payPipOptions(k *prep, d *Decision) {
	d.Sub = ChGenPip
	c := s.resPayCost(k)
	first, second := hybColours(c, 0)
	d.add(MakeAction(AkChoose, 0, 0, uint8(first)))
	d.add(MakeAction(AkChoose, 0, 0, uint8(second)))
}

// applyPayPip records the face; once every pip is announced the election
// is asked, decline-only when the announced cost is unpayable (execPay's
// gate over the announced cost).
func (s *State) applyPayPip(k *prep, a Action) {
	n := 0
	for v := s.payHyb; v != 0; v >>= 3 {
		n++
	}
	s.payHyb |= (a.Mode() + 1) << (3 * n)
	if c := s.resPayCost(k); c.Hyb[0] != 0 {
		return // the next pip's face
	}
	in := &s.prog(k)[s.res.pc]
	p, amt := s.g.pickP, int(s.g.pickLeft)
	if !s.canPayGen(k, p, kop.PayMana, amt, in.B.N, in.F) && !s.trigCostPotentialPays(k, p, kop.PayMana, s.g.pickF, amt, in.B.N, in.F) {
		s.g.pickF |= pfNoPay
	}
	s.res.ask = raGenPay
}

// announcedHyb resolves c's leading hybrid pips to the faces announced so
// far (none outside a trigger's own hybrid ask).
func (s *State) announcedHyb(c *kop.ManaCost) {
	for v := s.payHyb; v != 0 && c.Hyb[0] != 0; v >>= 3 {
		*c = c.Resolve(int(v&7) - 1)
	}
}

// resPayCost is the parked PayMana ask's cost: pickLeft generic, the
// instruction's coloured pips (NumTimes-scaled) and, on a PayMana
// instruction (whose F is no filter), its hybrid pips.
func (s *State) resPayCost(k *prep) kop.ManaCost {
	in := &s.prog(k)[s.res.pc]
	c := manaOf(int(s.g.pickLeft), s.numTimesPips(k, in.B.N))
	if in.IsPayMana() {
		kop.PayManaHyb(in.F, &c)
		s.announcedHyb(&c)
	}
	return c
}
