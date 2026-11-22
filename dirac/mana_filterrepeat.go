package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A fee-only first mana ability (kop.FeeOnlyManaAb): Nomadic Elf's "{1}{G}:
// Add one mana of any color", Orochi Leafcaller's "{G}: ...", Agent of
// Stromgald's "{R}: Add {B}", Energy Refractor's "{2}: ...". It has no {T} and
// no activation limit, so it is repeatable: nothing marks a use (CR 302.6
// does not bind it either, a tapped or summoning-sick source still activates),
// which matches gorge's scriptStateKey, whose dedup keys on the pool, the
// tapped bits and the counters alone.
//
// gorge's census (potential_plan.go paymentPlanCensusOf) lists it as a gap
// when the full PotentialMana fixpoint pays its fee, own production included,
// and paymentPlanRelaxedAlternatives gives it no relaxed alternative (it is
// repeatable and not a bounded tap part). The census is then unrelaxable and
// unbounded, its funding fee x potentialScriptNodes, and every insufficient
// play among the first potentialScriptPlays gets the prefix search, whatever
// gap before it in zone order cleared both flags (Mystic Gate before Energy
// Refractor still funds Lavaclaw Reaches' animation).

// filtRepeat: the filter at sl is a repeatable fee-only one.
func (s *State) filtRepeat(k *prep, sl Slot) bool {
	return k.gen.anyFiltRepeat && kop.FeeOnlyManaAb(s.filtAb(k, sl))
}

// filtRepeatIn: some filter of f is a repeatable fee-only one.
func (s *State) filtRepeatIn(k *prep, f slotMask) bool {
	if !k.gen.anyFiltRepeat {
		return false
	}
	for ; f.any(); f = f.rest() {
		if s.filtRepeat(k, f.first()) {
			return true
		}
	}
	return false
}

// filtRepeatCost is the fee of the repeatable filter at sl, after the
// activated-ability cost reducers (CR 601.2f, 602.2b).
func (s *State) filtRepeatCost(k *prep, sl Slot) kop.ManaCost {
	a := s.filtAb(k, sl)
	if k.gen.anyAbReduce {
		return s.abReduced(k, a, sl, s.perms[sl].ctrl, a.Cost.Mana)
	}
	return a.Cost.Mana
}

// filtRepeatPip is the one coloured pip of the repeatable filter at sl's fee
// (a fee of generic mana alone has none). The pip is paid first, then the
// generic part, as gorge's ResolveMana does (pay/solve.go).
func (s *State) filtRepeatPip(k *prep, sl Slot) (col int, ok bool) {
	if !s.filtRepeat(k, sl) {
		return 0, false
	}
	for c, n := range s.filtAb(k, sl).Cost.Mana.Pips {
		if n != 0 {
			return c, true
		}
	}
	return 0, false
}

// filtRepeatFee is the whole fee of the repeatable filter at sl: its generic
// part after the reducers plus its pip.
func (s *State) filtRepeatFee(k *prep, sl Slot) int {
	n := int(s.filtRepeatCost(k, sl).Generic)
	if _, ok := s.filtRepeatPip(k, sl); ok {
		n++
	}
	return n
}

// filtFixed: the first-only filter at sl makes one fixed colour (the repeatable
// fee-only one, or a {pip}, {T} one: Apprentice Wizard's {U}, {T}: Add {C}{C}{C}),
// so its activation asks nothing (a one-ability source resolves directly).
func (s *State) filtFixed(k *prep, sl Slot) bool {
	if !s.filtRepeat(k, sl) {
		if _, pip := s.filtPip(k, sl); !pip || !s.filtFirstOnly(k, sl) {
			return false
		}
	}
	a := s.filtAb(k, sl)
	return a.AFlags == 0 && a.ManaColors != kop.ColAny
}

// filtRepeatPotential: the potential pool (gorge's full PotentialMana
// fixpoint, a repeatable source's own production included) pays the fee of
// the repeatable filter at sl.
func (s *State) filtRepeatPotential(k *prep, p uint8, sl Slot, av *avail, have *bool) bool {
	if !*have {
		s.availPotential(k, p, av)
		*have = true
	}
	c := s.filtRepeatCost(k, sl)
	return av.affords(&c, pipMaskOf(&c), 0)
}

// filtFunding is the funding the prefix search may activate for the filter
// gaps f: a repeatable gap's census is unbounded, so the funding never binds
// (any value of at least potentialScriptNodes is gorge's fee x 160: a funding
// step costs a node); otherwise the fundable filters' fees summed, none when
// the search's funding is unrelaxable. A hybrid fee counts like any other:
// its activation spends the pip from floating mana (mana_filterhybrid.go), so
// the search must first tap a covered source to float it. A first-and-only
// filter's fee is a `paid` entry, which an unrelaxable census has cleared
// (filtFirstFreeFee): it funds nothing there, while a relaxable census counts
// it.
func (s *State) filtFunding(k *prep, p uint8, f slotMask) int {
	if s.filtRepeatIn(k, f) {
		return potentialScriptNodes
	}
	n := 0
	if !s.filtFundable(k, p, f) {
		return 0
	}
	relaxable := s.filtRelaxable(k, f)
	for ; f.any(); f = f.rest() {
		if !relaxable && s.filtFeeUncounted(k, f.first()) {
			continue // an unrelaxable census clears paid: a first-and-only filter's fee is no funding step (census.fees)
		}
		n += s.filtFee(k, f.first())
	}
	return n
}

// filtFeeUncounted reports a filter gap whose fee gorge's unrelaxable census
// does not count: any gap with a sound relaxed alternative (neither a hybrid
// fee nor a repeatable one) is no `fund` entry, and the unrelaxable census
// cleared the `paid` list that would have carried its fee, whether the
// filter is a card's first-and-only ability or a second one beside a free
// {T} ability (Prismatic Lens beside Sunken Ruins' hybrid fee: gorge's
// census funds 1, the Ruins' own fee).
func (s *State) filtFeeUncounted(k *prep, sl Slot) bool {
	if _, _, hyb := s.filtHyb(k, sl); hyb {
		return false
	}
	return !s.filtRepeat(k, sl)
}

// scriptAnyWalk: the prefix search's candidates are the any-mana walk, not the
// gaps walk, when gorge's census is unrelaxable (not relax). A fixed-colour
// repeatable fee-only filter (Agent of Stromgald's "{R}: Add {B}") is no
// exception: gorge's PotentialMana bound lifts its colour unbounded (CR
// 605.3a: it is activated again as long as its fee is paid, potential_repeat_fee.go),
// so the potential walk lists what the any-mana walk does.
func (s *State) scriptAnyWalk(k *prep, p uint8, filt slotMask, relax, unrelax bool) bool {
	return !relax
}

// windowFeeStalls: the payment window must not list the repeatable fee-only
// filter at sl, because activating it cannot bring the owed cost closer to
// paid. Its fee spends as much mana as it makes, so it only converts pool mana
// into a colour of its own, and that helps exactly when the colour fills a
// pip the pool still lacks (Bog Initiate's "{1}: Add {B}." against a generic
// remainder, or a {B} the pool already holds, converts B into B forever).
// gorge's feeOnlyConversionStalls is the same rule: the fee's pips are paid
// first, then its generic part from the pool in pay/solve.go's order ({C}, W,
// U, B, R, G), whatever the cost's pips still need; a hybrid pip, an any-type rider or no owed cost fails open.
func (s *State) windowFeeStalls(k *prep, p uint8, sl Slot, useful *kop.ManaCost, anyColor bool) bool {
	if useful == nil || anyColor || useful.Hyb[0] != 0 || s.g.winMode == winTrig || !s.filtRepeat(k, sl) {
		return false // a trigger's own cost window (winTrig) carries no pending cast, so gorge's paymentSourceUseful clamp never runs: it lists the filter
	}
	fee := s.filtRepeatCost(k, sl)
	ab := s.filtAb(k, sl)
	amount := int(ab.ManaN)
	if amount > int(fee.Generic)+pipSum(&fee.Pips) {
		return false
	}
	pool := s.pl[p].pool
	if m := s.restrPoolAdmits(k, p); m != 0 {
		s.rbUnits(p, m, &pool)
	}
	before := feeOnlyShort(&useful.Pips, &pool)
	if !feeOnlyPayFee(&fee, &pool) {
		return false
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if uint8(ab.ManaColors)&(1<<c) == 0 {
			continue
		}
		after := pool
		after[c] += uint8(amount)
		if feeOnlyShort(&useful.Pips, &after) < before {
			return false
		}
	}
	return true
}

// feeOnlyShort reads the fixed pips useful still lacks from pool.
func feeOnlyShort(useful, pool *[kop.NumManaKinds]uint8) (n int) {
	for c := range pool {
		if useful[c] > pool[c] {
			n += int(useful[c] - pool[c])
		}
	}
	return n
}

// feeOnlyPayFee pays a fee-only filter's fee from a copy of the pool: the
// fee's pips first, then its generic part in pay/solve.go's spend order
// ({C}, W, U, B, R, G), whatever the owed cost's pips still need. false
// fails the caller open: the pool cannot pay the fee.
func feeOnlyPayFee(fee *kop.ManaCost, pool *[kop.NumManaKinds]uint8) bool {
	for c := range pool {
		if fee.Pips[c] > pool[c] {
			return false
		}
		pool[c] -= fee.Pips[c]
	}
	for g := fee.Generic; g > 0; g-- {
		spent := false
		for _, c := range [...]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
			if pool[c] > 0 {
				pool[c]--
				spent = true
				break
			}
		}
		if !spent {
			return false
		}
	}
	return true
}

func pipSum(p *[kop.NumManaKinds]uint8) (n int) {
	for _, v := range p {
		n += int(v)
	}
	return n
}

// filtFeeColStalls: the fee-only repeatable filter at sl producing col
// cannot bring the play closer to paid: once the fee is paid from the pool,
// col fills no fixed pip the play's cost still lacks (windowFeeStalls' one
// colour arm; gorge's scriptSearch.feeOnlyAskPrune, engine/rules/
// potential_script.go; CR 605.3a: the activation is repeatable, so a line
// that skips it holds at least as much of every colour as one that spent a
// fee it cannot get back). The search would otherwise walk every colour of
// every repeat activation: Energy Refractor's "{2}: Add one mana of any
// color" beside a full battlefield spent the whole node budget on those
// branches and starved the line that pays the play without the filter, whose
// offer then went missing. useful nil (the play's printed pips unread,
// filtUsefulCost) fails open: the branch is explored.
func (s *State) filtFeeColStalls(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int, own Slot, d *payDesc) bool {
	return s.filtFeeColCut(k, p, sl, useful, col, own, true, d)
}

// filtFeeColDoomed is filtFeeColStalls' first cut alone: col fills no fixed pip
// the play still lacks once the fee is paid. The activation-site prune reads
// only this (gorge's scriptSearch.feeOnlyGapDoomed, potential_script.go, has
// no free-tap-colour test): a fixed {B} filter (Bog Initiate's "{1}: Add {B}")
// beside a Swamp is still the only source of a second {B}, and the swapped
// line the second cut relies on needs the colour ask the fixed production
// never poses.
func (s *State) filtFeeColDoomed(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int, own Slot, d *payDesc) bool {
	return s.filtFeeColCut(k, p, sl, useful, col, own, false, d)
}

// filtFeeColCut is the one body of both: second arms the free-tap-colour cut.
// d is the searched play's payment (filtPlayDesc): the pool the cut reads
// holds the restricted batches THAT payment may spend, as gorge's stash does
// (scriptSearch.actPool, pay.AvailableFor over the play's descriptor). A
// Powerstone's artifact-only {C} is no mana of a nonartifact cast, so a fee
// the plain pool cannot pay fails the cut open (the fee is paid with it at
// the activation, CR 106.6, but the stash never held it). nil reads the
// payment being made (payDescNow): the grant search's.
func (s *State) filtFeeColCut(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int, own Slot, second bool, d *payDesc) bool {
	if useful == nil || useful.Hyb[0] != 0 || s.g.winMode == winTrig || !s.filtRepeat(k, sl) {
		return false
	}
	fee := s.filtRepeatCost(k, sl)
	ab := s.filtAb(k, sl)
	amount := int(ab.ManaN)
	if amount > int(fee.Generic)+pipSum(&fee.Pips) {
		return false
	}
	if col < 0 || col >= kop.NumManaKinds || uint8(ab.ManaColors)&(1<<col) == 0 {
		return false
	}
	pool := s.pl[p].pool
	m := uint8(0)
	if d != nil {
		m = s.restrFloatAdmits(k, p, d)
	} else {
		m = s.restrPoolAdmits(k, p)
	}
	if m != 0 {
		s.rbUnits(p, m, &pool)
	}
	before := feeOnlyShort(&useful.Pips, &pool)
	if !feeOnlyPayFee(&fee, &pool) {
		return false
	}
	pool[col] += uint8(amount)
	if feeOnlyShort(&useful.Pips, &pool) >= before {
		return true // the colour fills no fixed pip the play still lacks (windowFeeStalls' arm)
	}
	// The second cut: a colour a free bare-tap source can make is dominated
	// even when it fills a pip. This is the swapped-line domination written
	// at gorge's home too (scriptSearch.feeOnlyFreeTapColour,
	// engine/rules/potential_script.go): the fee-only repeatable source S
	// makes colour c for fee F, but a first-only free source X (a bare tap)
	// can make c instead, leaving c filled and F floating; S stays untapped
	// (it has no {T}), so every continuation of the dominated branch is
	// reachable -- if the continuation uses X again, S makes the same colour
	// for F, paid from the saved F, and the endpoint after both activations
	// is the same. A repeatable free source is the same argument with X never
	// spent. S makes any colour, so it can stand in for every colour X can;
	// a source whose cost also spends a counter, a permanent or life is a
	// different class (the swap would lose that other use) and fails open.
	return second && s.filtFreeTapColourOwn(k, p, sl, own, col)
}

// filtFreeTapColour reports whether p controls an untapped source other than
// sl whose bare-tap mana ability can make col: the free unit the plain source
// census prices (pcard.manaTap), and the kernel mirror of gorge's
// scriptSearch.feeOnlyFreeTapColour (engine/rules/potential_script.go).
// A source whose ability also pays life (Mana Confluence's "{T}, Pay 1 life")
// is not this class (pc.painLife): the swapped line would spend that life,
// which the dominated branch saved, so the endpoint after both activations is
// not the same and the cut fails open (gorge's PaymentPlanTapOnlyCost reads
// the same class, pay.ManaFreeCost's c.Life == 0; canTapForManaPC only holds
// the lethal band out, life < painN, which is not enough here). The play's own
// {T} source (filtFreeTapColourOwn) is no free alternative either: its tap is
// the play's cost, so activating it for mana first leaves that cost unpayable
// (CR 602.2b, 601.2h; gorge's scriptSearch.ownTapSource).
func (s *State) filtFreeTapColour(k *prep, p uint8, skip Slot, col int) bool {
	return s.filtFreeTapColourOwn(k, p, skip, NoSlot, col)
}

// filtFreeTapColourOwn is filtFreeTapColour for a play whose own {T} source
// (own) cannot stand in for the filter: an activation's source is tapped by
// the cost, so its mana is no free alternative (CR 602.2b).
func (s *State) filtFreeTapColourOwn(k *prep, p uint8, skip, own Slot, col int) bool {
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if sl == skip || sl == own || s.perms[sl].flags&pfTapped != 0 || s.genLost(k, sl) || s.activationSick(k, sl) {
			continue
		}
		if k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		var pc *pcard
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		} else {
			pc = &k.cards[s.cardAt(sl)]
		}
		if pc.manaAb < 0 || !pc.manaTap || pc.manaSac || pc.painLife || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		if s.srcMaskAt(pc, sl)&(1<<uint(col)) != 0 {
			return true
		}
	}
	return false
}

// filtOneShotColStalls: the one-shot flexible gap at sl producing col cannot
// bring the play closer to paid while a fixed pip is still owed: col fills no
// pip the pool lacks, and a covered colour pays the generic part just as
// well. gorge's oneShotAskPrune (potential_script.go, cut D) is the same
// rule. The class is a gap that is not the fee-only repeatable one
// (filtFeeColStalls); the pool read is the activation's opening one, before
// any ability fee is paid, as gorge's actPool is; anything else fails open.
func (s *State) filtOneShotColStalls(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int) bool {
	if useful == nil || useful.Hyb[0] != 0 || s.g.winMode == winTrig || s.filtRepeat(k, sl) {
		return false
	}
	a := s.filtAb(k, sl)
	if col < 0 || col >= kop.NumManaKinds || uint8(a.ManaColors)&(1<<col) == 0 {
		return false
	}
	pool := s.pl[p].pool
	if m := s.restrPoolAdmits(k, p); m != 0 {
		s.rbUnits(p, m, &pool)
	}
	before := feeOnlyShort(&useful.Pips, &pool)
	if before == 0 {
		return false // no pip owed: the prune fails open (gorge's before == 0 arm)
	}
	pool[col]++
	return feeOnlyShort(&useful.Pips, &pool) >= before
}

// filtUsefulCost is play a's printed mana cost: the fixed pips the fee-stall
// clamp (filtFeeColStalls) reads (gorge's scriptPipCounts on the same play,
// engine/rules/potential_script.go). A plain cast's mana cost, an
// activation's printed base (playBaseCost). ok false for a play whose
// printed pips the search does not read -- a kicked, bargained, kicked-mode
// or flashback cast's cost is not the printed one, and an {X}, snow, hybrid,
// twobrid or Phyrexian pip is no fixed one -- and the clamp then fails open.
// A generic part is fine (any colour may pay it); an all-generic cost reads
// as no fixed pip, and the clamp then drops the filter's every colour: a
// conversion that never raises the spendable total cannot help pay it.
func (s *State) filtUsefulCost(k *prep, a Action) (m kop.ManaCost, ok bool) {
	if m, _, ok = s.playBaseCost(k, a); !ok {
		return kop.ManaCost{}, false
	}
	if m.X != 0 || m.Snow != 0 || m.Hyb != ([4]kop.ColorMask{}) || m.Tb != 0 {
		return kop.ManaCost{}, false
	}
	return m, true
}

// filtPlayDesc is the payment of the play a filter search prices, as gorge's
// scriptPlayDesc names it for the fee-stall prune's pool: a cast's spell, an
// activation's ability. ok false for any other play (gorge's prune fails open
// there, and the kernel keeps reading the payment being made).
func (s *State) filtPlayDesc(k *prep, a Action) (d payDesc, ok bool) {
	switch a.Kind() {
	case AkCast, AkActivate:
		return s.grantPayDescOf(k, a), true
	}
	return payDesc{}, false
}
