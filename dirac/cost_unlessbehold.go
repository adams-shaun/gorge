package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Behold<1/Type> as a resolution-time cost (CR 701.4a): an UnlessCost$ (Elven
// Passage's "you may behold an Elf. If you do, untap that land") or a printed
// trigger's own cost (Sarkhan, Dragon Ascendant). gorge poses the pay /
// decline election (pay only while every part has its candidates), then the
// "beholdcost" pick over the payer's matching permanents followed by their
// matching hand cards, unasked with exactly the count of candidates
// (rules/pay unlessflow.go AdvanceUnless, trigger_cost_behold.go). Nothing
// moves or is marked. It rides PayDiscard with B.N kop.PayBBehold (Instr.C's
// nibble is full); beholdPayKind widens it to payKindBehold.

// payKindBehold is the pay kind (execPay's kind, pickZ) of a resolution
// Behold<1/Type>.
const payKindBehold = 22

// beholdPayKind widens a PayDiscard whose B.N is PayBBehold to payKindBehold.
func beholdPayKind(kind int, bn int16) int {
	if kind == kop.PayDiscard && bn == kop.PayBBehold {
		return payKindBehold
	}
	return kind
}

// unlessBeholdCands counts player p's candidates for the behold filter f,
// adding each to d when d is set: the matching permanents p controls in
// battlefield order, then the matching hand cards in hand order.
//
//go:noinline
func (s *State) unlessBeholdCands(k *prep, p uint8, f kop.FilterID, d *Decision) int {
	n := 0
	for m := s.ctrlM[p&1]; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPerm(k, f, sl, p&1, NoSlot) {
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			}
		}
	}
	pl := &s.pl[p&1]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.matchCardGen(k, f, s.objs[o].card, p&1, p&1, o) {
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
	return n
}

// unlessBeholdOptions poses the pick (ChCostBehold).
//
//go:noinline
func (s *State) unlessBeholdOptions(k *prep, d *Decision, p uint8) {
	d.Sub = ChCostBehold
	s.unlessBeholdCands(k, p, s.genPickFilter(k), d)
}
