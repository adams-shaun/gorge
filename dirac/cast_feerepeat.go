package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The hand-cast offer gate's read of a live repeatable fee-only mana ability
// (kop.FeeOnlyManaAb, Signpost Scarecrow's "{2}: Add one mana of any color",
// pool tail-45-c004 decision 777). gorge's priority decision carries the
// potential plays its payment planner prices (rules payment_plan.go
// PotentialPaymentPlans, "lower the planner's witness": the cast is there once
// some way to pay it exists), and the kernel's mirror of that route is
// gorgeOffers's FeatPlannedCast arm -- but the affordability read this gate
// sits in refuses the cast first, so the planner route is never consulted.
// gorge's planner prices the Historian of Zhalfir's {2}{U}{U} from the payment
// the cast's window executes: the pool the untapped sources float covers the
// Scarecrow's {2} fee, whose activation while paying (CR 601.2i) makes the
// second blue pip the printed sources cannot (one Island beside three Forests,
// a Sandstone Needle and the Scarecrow; CR 605.1a, 106.6).
//
// The kernel's window walk prices the same payment: its first-only filter arm
// lists the live repeatable fee-only filters (windowPick's filtFirst arm,
// windowFeeStalls' first cut) and spends the fee from the floating pool
// (windowDryFilter's filtFeeSpend), so windowDry -- the same dry probe the
// kernel's own payment machinery reads (paymentStrandedPlan) -- settles the
// cost exactly when the window pays it. The gate admits the cast only then:
// the offer and the payment land together by construction, and a bound whose
// unbounded lift prices a cast no payment funds (Vivien, Arkbow Ranger's
// {2}{G}{G} beside two Scarecrow copies at decision 444, whose whole yield is
// two mana) is never read.
//
// Not in the scripted any-colour and gaps walks (scriptOffers): their censuses
// price their own lines, and a play they list is gorge's potential play, which
// its prefix search prices, not this gate. The read is the oracle surfaces':
// a base surface has no kernel mirror of the planner route to model.
func (s *State) castGateFeeRepeat(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) bool {
	if k.gen == nil || !k.gen.anyFiltRepeat || s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) == 0 ||
		s.g.availX&(availAny|availGaps) != 0 || !s.filtRepeatIn(k, s.scriptFilt(k, p)) {
		return false
	}
	ok, known := s.windowDry(k, p, cost, false)
	if !known {
		// The plain-pool census walk cannot price a source the payment may
		// still spend (a Powerstone's restricted {C}, CR 106.6) or one that
		// pays without a tap. Its unknown is not a refusal: the real walk on
		// a fork decides, as act_waterbend's probe does (act_waterbend.go).
		probe := s.fork()
		ok, _ = probe.windowTapRider(k, p, cost, false, false)
		s.unfork()
	}
	s.dbgFeeGate(k, p, c, o, cost, true, ok, known, true)
	return ok
}
