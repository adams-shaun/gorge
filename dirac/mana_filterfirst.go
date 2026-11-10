package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A filter as a card's first and only mana ability (CR
// 605.3a, 602.5b): the Devotees' "{1}: Add {W}, {B}, or {G}. Activate only
// once each turn", Three Tree Mascot's "{1}: Add one mana of any color.
// Activate only once each turn", Prophetic Prism's "{1}, {T}: Add one mana
// of any color" (kop.FilterManaAb). gorge's planner census prices none of
// them (pay.PlanLastResortCostOK rejects a mana part), so each is a paid
// census gap exactly like Hidden Grotto's second-ability filter (mana_filter.go),
// relaxed to one free unit of each colour it makes (potential_plan.go
// paymentPlanRelaxedAlternatives: its once-only cost is the {T} or the
// activation limit). It differs from Grotto's in three ways:
//
//   - it has no free ability: activating it poses only the colour ask
//     (rules activateManaFor resolves a one-ability source directly; the fee
//     is paid from the pool, no ask), Any's W U B R G or a Combo's colours
//     in printed order (effects.ComboColours; kop.Ability.ManaColOrd records
//     an out-of-WUBRG order, read by manaColourOrder), so filtStep's ans
//     stays 1+c;
//   - a once-each-turn one has no {T}: CR 302.6 does not bind it, a tapped
//     or summoning-sick Devotee still activates, and its use is
//     perm.actUsed (gorge's ManaActivate activation-limit marker), not the
//     tapped bit. gorge's search dedup (scriptStateKey) does not key on that
//     use, and filtKey does not either;
//   - gorge's census lists it only when its fee is payable from the
//     potential pool (paymentPlanCensusOf: manaAbilityPayablePool against
//     PotentialMana's fixpoint, where a source never funds its own fee):
//     Grotto's own {C} always pays its fee, a Devotee's needs other mana.
//
// It is never a funding step: it is always a census gap (a filter whose fee
// the potential pool cannot pay is not activatable in the search either),
// and the search funds with non-gap activate options only. In the payment
// window it is no unit (rules windowManaUnitsWith skips a non-free cost), as
// Grotto's filter is not: canTapForManaPC is false for it (pcard.manaTap
// needs a mana-free cost), so the plain walk never taps it, though the live
// window lists it once the pool pays its fee (windowFiltFirst).

// DebugFiltFirstNoLimit is a mutation (TestManaabmanacostMutation): a
// once-each-turn filter ignores its activation limit.
var DebugFiltFirstNoLimit bool

// prepFiltFirst records ability j of card c, its first mana ability, as a
// filter (genPrep.filt, gfFilt).
func (g *genPrep) prepFiltFirst(c CardID, j int) {
	if j >= 8 {
		panic("kernel: a once-each-turn filter mana ability past ability 7 (perm.actUsed)")
	}
	g.smallFor(c).filt = int8(j)
	g.setFlag(c, gfFilt)
	g.anyFilt, g.anyFiltFirst = true, true
}

// filtAb is the filter ability of the card at sl.
func (s *State) filtAb(k *prep, sl Slot) *kop.Ability {
	c := s.cardAt(sl)
	return &k.t.Cards[c].Abilities[k.gen.filt(c)]
}

// filtNoTap: the filter at sl costs no {T} (a once-each-turn one).
func (s *State) filtNoTap(k *prep, sl Slot) bool {
	return k.gen.anyFiltFirst && s.filtAb(k, sl).Cost.Flags&kop.CostTap == 0
}

// filtFirstOnly: the filter at sl is its card's first mana ability.
func (s *State) filtFirstOnly(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	return k.gen.anyFiltFirst && firstManaAb(&k.t.Cards[c]) == int(k.gen.filt(c))
}

// filtUsable: the filter at sl can be activated now, its fee aside: a {T}
// one untapped and (a creature) unsick, a once-each-turn one unused this
// turn (gorge's ActivationLimit$ gate), and not locked by a CantBeActivated
// static (cast_cantactivate.go), as a plain mana source is not.
func (s *State) filtUsable(k *prep, sl Slot) bool {
	if s.genLost(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	pm := &s.perms[sl]
	if s.filtNoTap(k, sl) {
		return DebugFiltFirstNoLimit || pm.actUsed&(1<<uint(k.gen.filt(s.cardAt(sl)))) == 0
	}
	return pm.flags&pfTapped == 0 && !(s.activationSick(k, sl))
}

// filtFirstMask is scriptFilt on a table with a first-only filter: p's
// usable filters, a first-only one only while other potential mana covers
// its fee.
//
//go:noinline
func (s *State) filtFirstMask(k *prep, p uint8) slotMask {
	var m slotMask
	other := -1
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if !k.gen.has(s.cardAt(sl), gfFilt) || !s.filtUsable(k, sl) {
			continue
		}
		if s.filtFirstOnly(k, sl) {
			if other < 0 {
				other = s.filtOtherMana(k, p)
			}
			if other < s.filtFee(k, sl) {
				continue
			}
		}
		m = m.with(sl)
	}
	return m
}

// filtOtherMana bounds the potential mana a first-only filter's fee may
// draw on: the pool and every source the plain walk taps (each other
// first-only filter is admitted to gorge's fixpoint only once these pay its
// own fee, so it adds nothing a fee of one could need), plus the
// counter-cost sources.
func (s *State) filtOtherMana(k *prep, p uint8) int {
	pl := &s.pl[SeatIx(p)]
	n := poolTotal(pl) + s.rbTotal(p, s.rbAll(p))
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if s.perms[sl].flags&pfTapped == 0 && s.canTapForMana(k, sl) {
			n += int(max(s.manaPCGen(k, sl).manaN, 1))
		}
	}
	if k.ctrMana {
		n += s.scriptCtr(k, p).count()
	}
	if k.gen.anyDeferred {
		// a deferred source's {R} (mana_deferred.go): gorge's PotentialMana
		// counts it, so Pyromancer's Goggles alone funds Three Tree Mascot's {1}
		n += s.scriptDeferred(k, p).count()
	}
	return n
}

// windowFiltFirstUsable: the live payment window lists the first-only filter at
// sl: it is usable and the floating mana pays its fee (gorge's window abilities,
// manaAbilityPayablePool). It is still no unit of the census above.
func (s *State) windowFiltFirstUsable(k *prep, p uint8, sl Slot) bool {
	c := s.cardAt(sl)
	return k.gen.has(c, gfFilt) && s.filtFirstOnly(k, sl) && s.filtUsable(k, sl) && poolTotal(&s.pl[SeatIx(p)]) >= s.filtFee(k, sl)
}

// windowFiltFirst activates that filter in the window, for the first colour it
// makes that useful still owes (gorge paymentUsefulColours), in the ability's
// ask order (a Combo's printed order, colourAskOrder). Nothing owed answers the
// lowest W U B R G colour, as payexec.WindowAnswer does; a window whose ask
// carries no cost (a trigger's own cost, noOwed, with no clamp) has nothing to
// owe, so it takes the same lowest.
func (s *State) windowFiltFirst(k *prep, p uint8, sl Slot, useful *kop.ManaCost, noOwedUnclamped bool) {
	ab := s.filtAb(k, sl)
	mask := uint8(ab.ManaColors)
	c := uint8(bits.TrailingZeros8(mask))
	if !noOwedUnclamped && useful != nil {
		// gorge's ask lists what the cost still owes first (owedColoursFirst),
		// in the ability's own order, from the pool the payment may spend
		// (windowUseful, CR 106.6): a pip its restricted mana covers is not
		// owed. Nothing owed falls back to the lowest W U B R G colour, which
		// is what payexec.WindowAnswer answers then. A hybrid pip leaves the
		// verdict undecided (windowUseful prices nothing): gorge keeps every
		// colour there, so the census alone gives the demand.
		want, _ := s.windowUseful(k, p, useful)
		if useful.Hyb[0] != 0 {
			want = s.windowDemand(k, p, useful)
		}
		if a := askOwedColour(ab, mask, want); a >= 0 {
			c = uint8(a)
		}
	}
	s.filtDo(k, p, filtStep{sl: sl, ans: uint8(1 + c), gap: true})
}

// windowFiltFirstFallback is windowFiltFirst for a window whose ask carries no
// owed pips (a PaymentFallback with no ManaPayment: a cast's altAddWindowTap,
// and every noOwed window that keeps a clamp: convoke, Spree, a lowered or free
// cast, waterbend, pain): it answers the lowest colour of the source, narrowed
// only to what the clamp keeps once it has decided (windowUseful, gorge
// paymentUsefulColours). Gorge asks the colour after the fee is paid, so the
// clamp reads the pool the fee leaves, spent as filtDo spends it (the
// restricted batches the activation admits first, then the pool): an unsettled
// generic leaves every colour listed, and Abzan Devotee's {W}{B}{G} answers {W}
// while a {B} pip is still owed (CR 601.2g).
func (s *State) windowFiltFirstFallback(k *prep, p uint8, sl Slot, useful *kop.ManaCost) {
	mask := uint8(s.filtAb(k, sl).ManaColors)
	var want uint8
	var decided bool
	if _, pip := s.filtPip(k, sl); pip {
		want, decided = s.windowUseful(k, p, useful)
	} else {
		c := s.fork()
		pool := &c.pl[SeatIx(p)].pool
		fee := s.filtFee(k, sl)
		fee -= c.filtFeeRestr(k, p, sl, fee) // filtDo's fee: restricted batches first
		for _, col := range [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
			for fee > 0 && pool[col] > 0 {
				pool[col]--
				fee--
			}
		}
		want, decided = c.windowUseful(k, p, useful)
		s.unfork()
	}
	if decided && mask&want != 0 {
		mask &= want
	}
	s.filtDo(k, p, filtStep{sl: sl, ans: uint8(1 + bits.TrailingZeros8(mask)), gap: true})
}

// filtUse spends the filter at sl: its {T}, or its use this turn.
func (s *State) filtUse(k *prep, sl Slot) {
	if s.filtNoTap(k, sl) {
		s.perms[sl].actUsed |= 1 << uint(k.gen.filt(s.cardAt(sl)))
	} else {
		s.perms[sl].flags |= pfTapped
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
}
