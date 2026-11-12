package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The attack tax (kop.StatAttackTax): a printed S:Mode$ CantAttackUnless
// with a generic mana Cost$ (Propaganda, Ghostly Prison, Windborn Muse,
// Baird, Steward of Argive, Sphere of Safety, Onakke Oathkeeper; CR 508.1d,
// 508.1g-i). gorge (rules/attack_cost.go):
//
//   - prices each (attacker, defender) pair, the sum over the statics in
//     force whose ValidCard$ admits the creature and whose Target$ admits the
//     defender (attackPairCharge); a negative charge is unpriceable and the
//     pair fails closed;
//   - offers a charged pair only while the payer's sources other than the
//     creature reach its charge (attackOffers' combatChargeAffordable);
//   - budgets the declaration as Decision.MaxSum over each option's Value:
//     the pair's charge plus, while some offered pair is charged, the units
//     the creature itself would add (attackOptionBudgetValue), MaxSum being
//     the pool and every source's units (attackBudget);
//   - pays the declaration's charge before it commits: from the pool, else
//     through a window posing one tap per ask (askNextAttackPay). The
//     oracle answers it with the first source listed (dirac/oracle
//     gorge.go internal): the fixed-production sources in battlefield
//     order, then the choice-shaped ones pinned to their first producible
//     colour (attackManaSources), a declared attacker CR 508.1f taps
//     withheld.
//
// The kernel declares attackers one pick at a time, so the budget is
// re-derived at each pick from the picks already made (nothing a price reads
// changes while a declaration is made), and Done pays the charge
// (atkTaxPay). The mana a source adds and the units it counts are the
// prepared shapes' (atkTaxManaAb): a table that also holds a mana source of
// another shape, an attack requirement or a Taps watcher is refused before
// its game (StallAtkTax).

// StallAtkTax: an attack tax beside a part the kernel's attack-tax walk does
// not model (atkTaxGap): gorge plays the table, the kernel refuses it at its
// start (initStall).
const StallAtkTax = 186

func init() { stallNames[StallAtkTax] = "atktax" }

// DebugAtkTaxFree (test only, TestAttackTaxFreeMutation): no attack is
// taxed. DebugAtkTaxChoiceFirst (TestAttackTaxOrderMutation): the payment
// taps the choice-shaped sources before the fixed-production ones.
// DebugAtkTaxWithholdVigilant (TestAttackTaxVigilantMutation): the payment
// withholds a declared attacker with vigilance too.
var DebugAtkTaxFree, DebugAtkTaxChoiceFirst, DebugAtkTaxWithholdVigilant bool

// atkTaxOn reports whether the table carries an attack tax.
func atkTaxOn(k *prep) bool { return k.gen != nil && k.gen.anyAtkTax }

// prepAtkTax sets genPrep.anyAtkTax when a card of t has a StatAttackTax
// static, and atkTaxStall when another part of the table is one the walk
// does not model beside it.
func prepAtkTax(t *kop.Table, g *genPrep) {
	g.anyAtkTax = tableHasStatic(t, kop.StatAttackTax)
	if !g.anyAtkTax {
		return
	}
	g.atkTaxStall = g.anyGoad || g.ix[mechAttacksIfAble] != arenaNone || g.blgOn || g.manaTaps
	for i := range t.Cards {
		g.atkTaxStall = g.atkTaxStall || atkTaxGap(&t.Cards[i])
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMustAttack {
				g.atkTaxStall = true
			}
		}
	}
}

// atkTaxGap reports whether card c holds a part the attack-tax walk does not
// model: a mana ability of another shape than atkTaxManaAb's, one an effect
// grants (a latent mana ability), an activated ability adding mana, an
// attack requirement (a MustAttack or Goad static, a goad, an attack duty),
// "can't attack alone", a Taps watcher, a tap-for-extra-mana static, or mana
// that outlives the step (PersistentMana$, an Effect's UnspentMana).
func atkTaxGap(c *kop.CardDef) bool {
	if c.AloneAtk {
		return true
	}
	nMana, first := 0, -1
	for j := range c.Abilities {
		a := &c.Abilities[j]
		switch a.Kind {
		case kop.AbMana:
			if a.AFlags&kop.AfLatent != 0 || !atkTaxManaAb(a) {
				return true
			}
			if first < 0 {
				first = j
			}
			nMana++
		case kop.AbStatic:
			switch a.Static.Kind {
			case kop.StatMustAttack, kop.StatGoad, kop.StatTapExtra:
				return true
			}
		case kop.AbTriggered:
			if a.Trig.Event == kop.TrigTaps {
				return true
			}
		}
		for i := range a.Prog {
			in := &a.Prog[i]
			if in.Op == kop.OpGoad || in.Ext() == kop.ExtAttacksIfAble ||
				in.Op == kop.OpAddMana && (a.Kind == kop.AbActivated || in.B.K == kop.VConst && (in.B.N == 2 || in.B.N == 3)) {
				return true
			}
		}
	}
	return nMana > 1 && !atkTaxMultiMana(c, first)
}

// atkTaxManaAb reports whether mana ability a has a shape the walk prices
// exactly as gorge's attack window lists it (rules windowManaUnits and
// attackChoiceManaSources): a cost of {T} alone, a literal amount, no spend
// restriction, rider, gate, deferral or simultaneous production, and one
// colour, a choice of colours or the chosen colour, or the commander's colour
// identity (ManaIdentity), which the window never lists (atkTaxShape).
func atkTaxManaAb(a *kop.Ability) bool {
	return a.Cost == (kop.Cost{Flags: kop.CostTap}) && a.ManaN > 0 && len(a.Prog) == 0 &&
		(a.AFlags&^(kop.AfManaChoice|kop.AfManaChosen) == 0 || a.ManaIdentity && a.AFlags&^(kop.AfManaChoice|kop.AfManaReflect) == 0) && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		!a.ManaEach && !a.ManaDeferred && a.SplitGate == 0 && a.ManaHasteValid == 0 && a.ManaRider == 0 &&
		a.ActZone == kop.ZoneNone && a.Gate == (kop.Gate{}) && !a.Exhaust && a.Once == 0
}

// atkTaxMultiMana reports whether card c's several mana abilities are the
// two shapes whose first ability gorge's window lists first (prepMultiMana):
// one fixed colour each, distinct, in W U B R G C order (a dual land's
// intrinsic abilities), or a fixed first ability beside a bare chosen-colour
// one (Quirion Elves; kop.ChosenSecondManaAb).
func atkTaxMultiMana(c *kop.CardDef, first int) bool {
	var seen kop.ColorMask
	n, second := 0, -1
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana {
			continue
		}
		if n++; n == 2 {
			second = j
		}
		m := a.ManaColors & 0x3f
		if a.AFlags != 0 || a.ManaN != 1 || bits.OnesCount8(uint8(m)) != 1 || m <= seen {
			return n == 2 && kop.ChosenSecondManaAb(c, first, second) && atkTaxOnlyTwo(c, second)
		}
		seen |= m
	}
	return true
}

// atkTaxOnlyTwo reports whether second is card c's last mana ability.
func atkTaxOnlyTwo(c *kop.CardDef, second int) bool {
	for j := second + 1; j < len(c.Abilities); j++ {
		if c.Abilities[j].Kind == kop.AbMana {
			return false
		}
	}
	return true
}

// atkTaxStallPair is pairRefusalN's StallAtkTax half: the text and the two
// sides, the cards with an attack tax and the cards with a gap.
func atkTaxStallPair(t *kop.Table, g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	text := "an attack tax beside an attack requirement, a can't-attack-alone creature, a Taps watcher, a land-type grant or a mana source of another shape: gorge's budget and payment window for that part are not modelled (combat_attacktax.go)"
	a := abilityOn(func(x *kop.Ability) bool { return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatAttackTax })
	b := func(c *kop.CardDef, _ int) bool {
		if atkTaxGap(c) {
			return true
		}
		for j := range c.Abilities {
			x := &c.Abilities[j]
			if !g.blgOn || x.Kind != kop.AbStatic || x.Static.Kind != kop.StatContinuous {
				continue
			}
			for _, sub := range g.blgSub {
				if sub != subOutside && x.Static.AddSub.Has(int(sub)) {
					return true // a basic land-type grant (static_landtype.go)
				}
			}
		}
		return false
	}
	return text, a, b
}

// atkTaxCharge is the generic mana the creature at sl attacking tgt (a
// player code or a battlefield object) is charged, and false when a
// matching static's charge is negative (gorge's unpriceable pair).
func (s *State) atkTaxCharge(k *prep, sl Slot, tgt uint8) (n int, ok bool) {
	ok = true
	if DebugAtkTaxFree {
		return 0, true
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAttackTax || !s.statCond(k, st) ||
			!s.atkTaxScope(k, &a.Static, st.ctrl, tgt) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		v := int(a.Static.A)
		if a.Static.StFlags&kop.StDynamic != 0 {
			v = s.evalV(k, a.Static.VA, s.ctxStatic(st))
		}
		if v < 0 {
			ok = false
		} else {
			n += v
		}
		return true
	})
	return n, ok
}

// atkTaxScope reports whether the static's Target$ scope admits tgt (gorge
// combat.RestrictionTargetMatches): StAtkTaxPlayer an attack on the static's
// controller, StAtkTaxWalker one on a face-up permanent with a printed
// planeswalker type that player controls, neither every defender.
func (s *State) atkTaxScope(k *prep, st *kop.Static, ctrl, tgt uint8) bool {
	sc := st.StFlags & (kop.StAtkTaxPlayer | kop.StAtkTaxWalker)
	switch {
	case sc == 0:
		return true
	case isPlayerTarget(tgt):
		return sc&kop.StAtkTaxPlayer != 0 && CodeSeat(tgt) == ctrl
	case sc&kop.StAtkTaxWalker == 0 || isPlayerOrNone(tgt):
		return false
	}
	ob := &s.objs[tgt]
	if ob.zone != kop.ZoneBattlefield {
		return false
	}
	q := ob.slot
	return !s.faceDown(k, q) && k.t.Cards[s.cardAt(q)].Types&kop.TPlaneswalker != 0 && s.perms[q].ctrl == ctrl
}

// atkTaxShape is gorge's attack window's reading of the mana source at sl
// (attackManaSources over atkTaxManaAb's shapes, its first ability being the
// first alternative listed): the colour that alternative adds, the units it
// adds, whether it is a fixed production (listed before every choice-shaped
// source) and whether the window lists the source at all. A choice is
// pinned to its first producible colour in W U B R G C order; a chosen-colour
// ability lists only with a recorded colour, and a colour identity outside a
// commander game makes nothing.
func (s *State) atkTaxShape(k *prep, sl Slot) (col, n int, fixed, ok bool) {
	if !s.canTapForMana(k, sl) {
		return 0, 0, false, false
	}
	a := &k.t.Cards[s.cardAt(sl)].Abilities[s.manaPCGen(k, sl).manaAb]
	if a.ManaIdentity {
		// gorge's attack window excludes the commander-identity shape: the
		// parser claims no colour for it (rules attackChoiceManaSources)
		return 0, 0, false, false
	}
	m := uint8(a.ManaColors) & 0x3f
	switch {
	case a.AFlags&kop.AfManaChosen != 0:
		ch := s.perms[sl].chosen
		if ch >= 5 {
			return 0, 0, false, false
		}
		m |= 1 << ch
	case a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(m) == 1:
		return bits.TrailingZeros8(m), int(a.ManaN), true, true
	case m == 0:
		return 0, 0, false, false
	}
	return bits.TrailingZeros8(m), int(a.ManaN), false, true
}

// atkTaxUnits is what the source at sl adds toward a charge (gorge
// attackSourceUnits: its largest alternative; every prepared shape's
// alternatives add the same).
func (s *State) atkTaxUnits(k *prep, sl Slot) int {
	_, n, _, ok := s.atkTaxShape(k, sl)
	if !ok {
		return 0
	}
	return n
}

// atkTaxBudget is the active player's attack budget (gorge attackBudget):
// the pool and every source's units.
func (s *State) atkTaxBudget(k *prep) int {
	p := s.active
	n := 0
	for _, v := range s.pl[SeatIx(p)].pool {
		n += int(v)
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		n += s.atkTaxUnits(k, m.first())
	}
	return n
}

// atkTaxTargets fills tg with the defenders attackerOptions offers the
// active player's creatures (each other seat still in the game, the
// planeswalkers it controls, the attackable battles) and returns how many.
func (s *State) atkTaxTargets(k *prep, tg *[MaxPerm + MaxSeats]uint8) int {
	n := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == s.active || s.out(q) {
			continue
		}
		tg[n] = PlayerCode(q)
		n++
		for pm := s.ctrlM[q]; pm.any(); pm = pm.rest() {
			if w := pm.first(); s.typesAt(k, w)&kop.TPlaneswalker != 0 {
				tg[n] = uint8(s.perms[w].obj)
				n++
			}
		}
	}
	if k.gen.anyBattle {
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			for q := s.ctrlM[p]; q.any(); q = q.rest() {
				if sl := q.first(); s.battleAttackable(k, sl, s.active) {
					tg[n] = uint8(s.perms[sl].obj)
					n++
				}
			}
		}
	}
	return n
}

// atkTaxCandidate reports whether the creature at sl is in gorge's offer
// list: declared in this declaration, or able to attack.
func (s *State) atkTaxCandidate(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfAttacking != 0 || s.canAttack(k, sl)
}

// atkTaxRead is the declaration's budget at a pick: budget (MaxSum), taxed
// (some offered pair is charged: each Value then folds in its creature's
// own units, gorge attackTaxed) and spent (the declared picks' Values).
func (s *State) atkTaxRead(k *prep, tg []uint8) (budget int, taxed bool, spent int) {
	budget = s.atkTaxBudget(k)
	for m := s.ctrlM[s.active]; m.any() && !taxed; m = m.rest() {
		sl := m.first()
		if !s.atkTaxCandidate(k, sl) {
			continue
		}
		u := s.atkTaxUnits(k, sl)
		for _, t := range tg {
			if n, ok := s.atkTaxCharge(k, sl, t); ok && n > 0 && n+u <= budget {
				taxed = true
				break
			}
		}
	}
	if !taxed {
		return budget, false, 0
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 {
			n, _ := s.atkTaxCharge(k, sl, s.perms[sl].atk)
			spent += n + s.atkTaxUnits(k, sl)
		}
	}
	return budget, true, spent
}

// atkTaxFilter drops the attack picks gorge's offer list and budget refuse:
// a charged pair the sources other than its creature cannot pay, and a pick
// whose Value the budget left by the declared picks cannot carry (the
// oracle's admissible over Decision.MaxSum). Done stays: the declared picks
// fit by construction.
//
//go:noinline
func (s *State) atkTaxFilter(k *prep, d *Decision) {
	var tg [MaxPerm + MaxSeats]uint8
	budget, taxed, spent := s.atkTaxRead(k, tg[:s.atkTaxTargets(k, &tg)])
	j := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkAttack {
			sl := s.objs[a.Source()].slot
			n, ok := s.atkTaxCharge(k, sl, a.Target())
			u := s.atkTaxUnits(k, sl)
			if !ok || n > 0 && n+u > budget || taxed && spent+n+u > budget {
				continue
			}
		}
		d.Options[j] = a
		j++
	}
	d.N = uint16(j)
}

// atkTaxAnyOffer is anyAttacker under an attack tax: gorge poses no
// attackers decision when every pair is priced out (askAttackers' empty
// option list).
//
//go:noinline
func (s *State) atkTaxAnyOffer(k *prep) bool {
	var tg [MaxPerm + MaxSeats]uint8
	nt := s.atkTaxTargets(k, &tg)
	budget := s.atkTaxBudget(k)
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.canAttack(k, sl) {
			continue
		}
		u := s.atkTaxUnits(k, sl)
		for _, t := range tg[:nt] {
			if n, ok := s.atkTaxCharge(k, sl, t); ok && (n == 0 || n+u <= budget) {
				return true
			}
		}
	}
	return false
}

// atkTaxNext is the source the oracle's window answer taps next, the first
// gorge's attack window lists: the first fixed-production source in
// battlefield order, else the first choice-shaped one, a declared attacker
// without vigilance withheld (CR 508.1f taps it before 508.1h's mana
// abilities). NoSlot when none is left.
func (s *State) atkTaxNext(k *prep, p uint8) (sl Slot, col, n int) {
	var order [MaxPerm]Slot
	no := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
	for pass := 0; pass < 2; pass++ {
		for _, o := range order[:no] {
			if s.perms[o].flags&pfAttacking != 0 && (s.kw(k, o)&kop.KwVigilance == 0 || DebugAtkTaxWithholdVigilant) {
				continue
			}
			if c, u, fixed, ok := s.atkTaxShape(k, o); ok && u > 0 && fixed == (pass == 0 != DebugAtkTaxChoiceFirst) {
				return o, c, u
			}
		}
	}
	return NoSlot, 0, 0
}

// atkTaxPay pays the declaration's attack tax as Done is answered (CR
// 508.1g-i; gorge handleAttackers): the pool first, then the window's taps
// until the pool covers the charge, then the charge from the pool (generic,
// C W U B R G first: payFromPool). When the sources run out first the
// declaration is abandoned, the taps standing (gorge emits the empty
// DeclareAttackers); the budget offers no such declaration.
//
//go:noinline
func (s *State) atkTaxPay(k *prep) {
	p := s.active
	total := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 {
			n, _ := s.atkTaxCharge(k, sl, s.perms[sl].atk)
			total += n
		}
	}
	if total == 0 {
		return
	}
	pool := &s.pl[SeatIx(p)].pool
	for {
		have := 0
		for _, v := range pool {
			have += int(v)
		}
		if have >= total {
			break
		}
		sl, col, n := s.atkTaxNext(k, p)
		if sl == NoSlot {
			for m := s.ctrlM[p]; m.any(); m = m.rest() {
				if a := m.first(); s.perms[a].flags&pfAttacking != 0 {
					s.leaveCombat(a)
				}
			}
			return
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.poolAdd(p, col, n)
		s.artTag(k, p, sl, col)
	}
	for total > 0 {
		c := kop.ManaCost{Generic: uint8(min(total, 255))}
		total -= int(c.Generic)
		s.payFromPool(p, &c, 0)
	}
}
