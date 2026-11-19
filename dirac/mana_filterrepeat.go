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
// step costs a node); otherwise the fees summed, none for an unrelaxable
// census.
func (s *State) filtFunding(k *prep, p uint8, f slotMask) int {
	if s.filtRepeatIn(k, f) {
		return potentialScriptNodes
	}
	n := 0
	if s.filtFunded(k, p, f) {
		for ; f.any(); f = f.rest() {
			n += s.filtFee(k, f.first())
		}
	}
	return n
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

func pipSum(p *[kop.NumManaKinds]uint8) (n int) {
	for _, v := range p {
		n += int(v)
	}
	return n
}

// filtFeeColStalls: the fee-only repeatable filter at sl producing col
// cannot bring the play closer to paid: once the fee is paid from the pool,
// col fills no fixed pip the play's cost still lacks (windowFeeStalls' one
// colour arm; gorge's feeOnlyConversionStalls; CR 605.3a: the activation is
// repeatable, so a line that skips it holds at least as much of every colour
// as one that spent a fee it cannot get back). The search would otherwise
// walk every colour of every repeat activation: Energy Refractor's "{2}:
// Add one mana of any color" beside a full battlefield spent the whole node
// budget on those branches and starved the line that pays the play without
// the filter, whose offer then went missing (Vivid Meadow's counter arm, the
// only second {R}). useful nil fails open: the branch is explored.
func (s *State) filtFeeColStalls(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int) bool {
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
	if m := s.restrPoolAdmits(k, p); m != 0 {
		s.rbUnits(p, m, &pool)
	}
	before := feeOnlyShort(&useful.Pips, &pool)
	if !feeOnlyPayFee(&fee, &pool) {
		return false
	}
	pool[col] += uint8(amount)
	return feeOnlyShort(&useful.Pips, &pool) >= before
}

// colArmStalls: the any-colour arm of the activation at sl making col
// cannot be on a successful line when the same activation can make a colour
// that fills a fixed pip the pool still lacks (fs.useful) and col fills
// none: the line that makes that colour holds as much of every colour as
// the col one, the missing pip included (CR 601.2b: the caster announces
// each pip's colour), so the search walks only the better arms. A live
// restriction, a restricted ability or an unread cost fails open: the made
// unit's restriction, not its colour, may decide the line.
func (fs *filtSearch) colArmStalls(t *State, k *prep, ab *kop.Ability, col int) bool {
	if !fs.usefulOK || ab.ManaRestrict != 0 || t.restrPoolAdmits(k, fs.p) != 0 || t.g.winMode == winTrig {
		return false
	}
	pool := t.pl[SeatIx(fs.p)].pool
	short := func(c int) int {
		p := pool
		if c >= 0 && c < kop.NumManaKinds && uint8(ab.ManaColors)&(1<<uint(c)) != 0 {
			p[c]++
		}
		n := 0
		for i := range p {
			if fs.useful.Pips[i] > p[i] {
				n += int(fs.useful.Pips[i] - p[i])
			}
		}
		return n
	}
	sc := short(col)
	for c := 0; c < kop.NumManaKinds; c++ {
		if c == col || uint8(ab.ManaColors)&(1<<uint(c)) == 0 {
			continue
		}
		if short(c) < sc {
			return true
		}
	}
	return false
}

// filtUsefulCost is play a's printed mana cost: the fixed pips the fee-stall
// clamp (filtFeeColStalls) reads. A plain cast's mana cost, a flashback
// one's flashback cost (FeatPoolPays' mode mapping), an activation's printed
// base (playBaseCost). ok false for a play whose printed pips the search
// does not read -- a kicked, bargained or retrace cast's cost is not the
// printed one, and an {X}, snow, hybrid or twobrid pip is no fixed one --
// and the clamp then fails open.
func (s *State) filtUsefulCost(k *prep, a Action) (m kop.ManaCost, ok bool) {
	switch a.Kind() {
	case AkCast:
		if a.Mode() == 2 {
			o := Obj(a.Source())
			if o == NoObj || int(o) >= len(s.objs) {
				return m, false
			}
			m = s.flashbackCostOf(k, o)
		} else {
			var ok2 bool
			if m, _, ok2 = s.playBaseCost(k, a); !ok2 {
				return kop.ManaCost{}, false
			}
		}
	case AkActivate:
		var ok2 bool
		if m, _, ok2 = s.playBaseCost(k, a); !ok2 {
			return kop.ManaCost{}, false
		}
	default:
		return kop.ManaCost{}, false
	}
	if m.X != 0 || m.Snow != 0 || m.Hyb != ([4]kop.ColorMask{}) || m.Tb != 0 {
		return kop.ManaCost{}, false
	}
	return m, true
}

// filtChainDoomed: every activation of the fee-only repeatable filter at sl
// spends more spendable mana than it makes (FeeOnlyManaAb: one unit made, a
// mana-only fee of at least one), so a line that skips it holds at least as
// much spendable mana as one that uses it once, k times: k times at most the
// pool plus the printed production of the sources whose activations do not
// shrink the total. When that bound cannot reach the play's whole cost, no
// line activates the filter, whichever colour it would make, and the search
// walks none of those branches: Energy Refractor's "{2}: Add one mana of any
// color" beside a full battlefield spent the whole node budget on a
// conversion that cannot pay the play, and the line that pays without the
// filter lost its nodes (Vivid Meadow's counter arm, the only second {R}).
// The fee's spend may come from restricted units the play refuses, which the
// activation would turn into spendable mana: when those cover the fee alone
// the made unit is spendable mana the bound must keep (CR 106.6).
func (s *State) filtChainDoomed(k *prep, p uint8, sl Slot, fs *filtSearch) bool {
	if !fs.usefulOK || fs.costUnits == 0 {
		return false
	}
	fee := s.filtRepeatCost(k, sl)
	feeTotal := int(fee.Generic) + pipSum(&fee.Pips)
	var av avail
	s.availFor(k, p, &av)
	n := av.total
	var rb, adm [kop.NumManaKinds]uint8
	s.rbUnits(p, s.rbAll(p), &rb)
	if m := s.restrPoolAdmits(k, p); m != 0 {
		s.rbUnits(p, m, &adm)
	}
	nRb, nAdm := 0, 0
	for _, u := range rb {
		nRb += int(u)
	}
	for _, u := range adm {
		nAdm += int(u)
	}
	n += nAdm // the units the play's restriction admits spend like the pool
	if nRb-nAdm >= feeTotal {
		return false // the fee may come from the refused units: the made unit is spendable mana
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl2 := m.first()
		pc := s.manaPCGen(k, sl2)
		if !s.canTapForManaPC(k, sl2, pc) || pc.manaAb < 0 {
			continue
		}
		if ab := &k.t.Cards[s.cardAt(sl2)].Abilities[pc.manaAb]; kop.FeeOnlyManaAb(ab) {
			n-- // one unit made, a fee of at least one: the activation only shrinks the spendable total
		}
	}
	return n <= fs.costUnits
}
