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

// DebugAtkTaxCountUnlisted (test only, TestFourSeatAtkTaxUnlistedLockstep):
// the walk counts and taps a source whose first mana ability the window never
// lists (a Signet, a filter land, Prophetic Prism) as a fixed source of its
// production.
var DebugAtkTaxCountUnlisted bool

// DebugAtkTaxCountRestricted (test only, TestFourSeatAtkTaxRestrictedLockstep):
// the walk counts and taps a source whose mana is restricted (Giada's {W} for
// Angels) as a source of its production.
var DebugAtkTaxCountRestricted bool

// DebugAtkTaxEachOne (test only, TestFourSeatAtkTaxBounceLockstep): a bounce
// land's tap adds one unit of its first colour, not every colour at once.
var DebugAtkTaxEachOne bool

// DebugAtkTaxReflPin (test only, TestFourSeatAtkTaxReflectedLockstep): a
// reflected source tapped with several colours on offer makes the first.
var DebugAtkTaxReflPin bool

// DebugAtkTaxPaidLast (test only, TestFourSeatAtkTaxPaidLockstep): the
// window taps a self-sacrificing source with the choice-shaped ones, in
// battlefield order, not between them and the fixed-production ones.
var DebugAtkTaxPaidLast bool

// The window's census passes (gorge attackManaSources): the free
// fixed-production sources, then the paid-cost ones windowManaUnitsPaid
// appends (a self-sacrifice, a fixed life cost), then the choice-shaped ones
// attackChoiceManaSources appends.
const (
	atkPassFixed = iota
	atkPassPaid
	atkPassChoice
)

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
			if a.AFlags&kop.AfLatent != 0 {
				return true
			}
			if atkTaxUnlisted(a) {
				continue
			}
			if !atkTaxManaAb(a) && !atkTaxPaidAb(a) && !atkTaxPainPair(c, first, j) && !atkTaxReflSecond(c, first, a) {
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
	if f := atkTaxFirstMana(c); f >= 0 && nMana > 0 && atkTaxUnlisted(&c.Abilities[f]) {
		return true // the source view is the unlisted first ability, not the listed one
	}
	return nMana > 1 && !atkTaxMultiMana(c, first)
}

// atkTaxFirstMana is the index of card c's first mana ability, -1 for none
// (prepare's pcard.manaAb: the one the kernel's source view reads).
func atkTaxFirstMana(c *kop.CardDef) int {
	for j := range c.Abilities {
		if c.Abilities[j].Kind == kop.AbMana {
			return j
		}
	}
	return -1
}

// atkTaxUnlisted reports whether mana ability a is one gorge's attack window
// never lists (attackManaSources lists a free {T} ability, a self-sacrifice or
// a fixed life cost: windowManaUnitsPaid, paidCostShape): a mana fee (a
// Signet's or a filter land's {1}, {T}; Prophetic Prism), a counter removed or
// added (a Vivid land, Wall of Roots), a tapped creature or a sacrificed
// permanent besides the source. It adds no unit to the budget and is never
// tapped, so the walk ignores it (CR 605.3a: the payer may still activate it
// outside the window).
func atkTaxUnlisted(a *kop.Ability) bool {
	const other = kop.CostSac | kop.CostRemoveCtr | kop.CostAddCtr | kop.CostDiscard | kop.CostTapX | kop.CostExileGrave
	f := a.Cost.Flags
	fee := a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && (fee || f&other != 0) && !a.Cost.DiscardHand &&
		f&^(kop.CostTap|kop.CostOncePerTurn|kop.CostPayLife|other) == 0 || atkTaxRestricted(a)
}

// atkTaxRestricted reports whether mana ability a spends only on some
// payments (RestrictValid$: Secluded Courtyard's creature spells of the
// chosen type, Giada's Angels, a Powerstone's nonspell costs). gorge's
// windows list no restricted ability (windowManaUnits, appendPaidManaUnits and
// attackChoiceManaSources each skip RestrictValid$): its batch could not pay
// an attack cost it does not name (CR 106.6).
func atkTaxRestricted(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0 || a.AFlags&kop.AfManaNonSpell != 0)
}

// atkTaxManaAb reports whether mana ability a has a shape the walk prices
// exactly as gorge's attack window lists it (rules windowManaUnits and
// attackChoiceManaSources): a cost of {T} alone, a literal amount, no spend
// restriction, rider, gate, deferral or simultaneous production, and one
// colour, a choice of colours or the chosen colour, or the commander's colour
// identity (ManaIdentity), which the window never lists (atkTaxShape).
func atkTaxManaAb(a *kop.Ability) bool {
	return a.Cost == (kop.Cost{Flags: kop.CostTap}) && a.ManaN > 0 && len(a.Prog) == 0 &&
		(a.AFlags&^(kop.AfManaChoice|kop.AfManaChosen) == 0 || a.ManaIdentity && a.AFlags&^(kop.AfManaChoice|kop.AfManaReflect) == 0 || atkTaxReflAb(a)) && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		(!a.ManaEach || atkTaxEachAb(a)) && !a.ManaDeferred && a.SplitGate == 0 && a.ManaHasteValid == 0 && a.ManaRider == 0 &&
		a.ActZone == kop.ZoneNone && a.Gate == (kop.Gate{}) && !a.Exhaust && a.Once == 0
}

// atkTaxPaidAb reports whether mana ability a is a self-sacrifice source of
// the shape gorge's paid-cost census lists (paidCostShape: Sac<1/CARDNAME>,
// alone or with {T}: a Treasure's "{T}, Sacrifice this token: Add one mana of
// any color", an Eldrazi Spawn's "Sacrifice this creature: Add {C}", a Gold
// token): one colour or a choice of colours, a literal amount, no rider.
func atkTaxPaidAb(a *kop.Ability) bool {
	f := a.Cost.Flags
	return a.Kind == kop.AbMana && (f == kop.CostSacSelf || f == kop.CostTap|kop.CostSacSelf) && a.Cost.Mana == (kop.ManaCost{}) &&
		!a.Cost.DiscardHand && a.ManaN > 0 && len(a.Prog) == 0 && a.AFlags&^kop.AfManaChoice == 0 && a.ManaColors != 0 && a.ManaColors&^0x3f == 0 &&
		a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && !a.ManaEach && !a.ManaDeferred && a.SplitGate == 0 && a.ManaHasteValid == 0 &&
		a.ManaRider == 0 && a.ActZone == kop.ZoneNone && a.Gate == (kop.Gate{}) && !a.Exhaust && a.Once == 0 && !a.ManaIdentity
}

// atkTaxPaid is atkTaxShape for a self-sacrifice source: gorge's census lists
// it after every free fixed-production source (appendPaidManaUnits) and, a
// choice of colours pinned per colour (paidManaAlts), its first alternative
// the lowest colour. A tapped one is out only when its cost taps (a sacrifice
// alone needs no untapped source, CR 605.1a: ActivationTapCostUnavailable).
func (s *State) atkTaxPaid(k *prep, sl Slot, pc *pcard, a *kop.Ability) (col, n, pass int, ok bool) {
	if a.Cost.Flags&kop.CostTap != 0 && !s.canTapForManaPC(k, sl, pc) || a.Cost.Flags&kop.CostTap == 0 && !s.canSacOnlyNow(k, sl, pc) {
		return 0, 0, 0, false
	}
	pass = atkPassPaid
	if DebugAtkTaxPaidLast {
		pass = atkPassChoice
	}
	return bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f), int(a.ManaN), pass, true
}

// atkTaxReflAb reports whether mana ability a is a reflected "one mana of any
// color that a land an opponent controls could produce" (AB$ ManaReflected,
// ReflectProperty$ Produce, ColorOrType$ Color: Fellwar Stone, Exotic
// Orchard).
func atkTaxReflAb(a *kop.Ability) bool {
	return a.AFlags&^(kop.AfManaChoice|kop.AfManaReflect) == 0 && a.AFlags&kop.AfManaReflect != 0 && a.ManaProduce && !a.ManaProduceType &&
		!a.ManaIdentity && !a.ManaGrave && a.ManaAmong != 0 && a.ManaN == 1
}

// atkTaxEachAb reports whether mana ability a adds one fixed set of colours at
// once (a bounce land's "{T}: Add {W}{U}", Produced$ "W U": ManaEach). gorge's
// window lists it as a fixed unit of every colour's units together
// (windowManaUnits: a non-choice production, counts over its slots), so it
// pays two units and floats both colours (CR 106.1, 605.3a).
func atkTaxEachAb(a *kop.Ability) bool {
	return a.ManaEach && a.AFlags == 0 && a.ManaN > 0 && bits.OnesCount8(uint8(a.ManaColors&0x3f)) >= 2 && a.ManaColors&^0x3f == 0
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
		if a.Kind != kop.AbMana || atkTaxUnlisted(a) {
			continue
		}
		if n++; n == 2 {
			second = j
			if atkTaxPainPair(c, first, second) {
				return true
			}
		}
		if n == 2 && atkTaxReflSecond(c, first, a) && atkTaxOnlyTwo(c, j) {
			return true
		}
		m := a.ManaColors & 0x3f
		if a.AFlags != 0 || a.ManaN != 1 || bits.OnesCount8(uint8(m)) != 1 || m <= seen {
			return n == 2 && kop.ChosenSecondManaAb(c, first, second) && atkTaxOnlyTwo(c, second)
		}
		seen |= m
	}
	return true
}

// atkTaxReflSecond reports whether a is a reflected one-mana ability (Plaza of
// Heroes' "one mana of any color among legendary permanents you control")
// beside card c's plain fixed first ability, one unit as well. gorge's unit
// lists the first alternative first and the oracle taps it, so the reflected
// one is never tapped, and its one unit adds nothing to the unit's maximum.
func atkTaxReflSecond(c *kop.CardDef, first int, a *kop.Ability) bool {
	if first < 0 || !atkTaxManaAb(&c.Abilities[first]) {
		return false
	}
	f := &c.Abilities[first]
	return a.AFlags&^(kop.AfManaChoice|kop.AfManaReflect) == 0 && a.AFlags&kop.AfManaReflect != 0 && !a.ManaIdentity && !a.ManaGrave &&
		a.ManaN == 1 && f.ManaN == 1 && f.AFlags == 0 && a.Cost == (kop.Cost{Flags: kop.CostTap}) && len(a.Prog) == 0 && a.ManaAmong != 0 &&
		!a.ManaDeferred && a.SplitGate == 0 && a.ManaHasteValid == 0 && a.ManaRider == 0 && a.ActZone == kop.ZoneNone && a.Gate == (kop.Gate{}) &&
		!a.Exhaust && a.Once == 0 && !a.ManaEach && !a.ManaProduceType
}

// atkTaxPainPair reports whether card c's two mana abilities, first and
// second, are a painland's or a Talisman's: a plain fixed {C} or one-colour
// ability, then a {T}-only choice of colours (or one colour) whose rider is
// the painland damage (manaPain). gorge's attack window taps the first
// listed source in the fixed pass, so the land pays {C} and the painful
// alternative never joins the free census (windowManaUnits drops it; the
// choice pass lists it last). A pain-only source or a first ability that is
// a choice orders differently and stays a gap.
func atkTaxPainPair(c *kop.CardDef, first, second int) bool {
	if first < 0 || second <= first || second >= len(c.Abilities) {
		return false
	}
	for j := range c.Abilities {
		if j != first && j != second && c.Abilities[j].Kind == kop.AbMana && !atkTaxUnlisted(&c.Abilities[j]) {
			return false
		}
	}
	f, a := &c.Abilities[first], &c.Abilities[second]
	if f.Kind != kop.AbMana || a.Kind != kop.AbMana || f.AFlags != 0 || !atkTaxManaAb(f) || f.ManaN != 1 || bits.OnesCount8(uint8(f.ManaColors&0x3f)) != 1 {
		return false
	}
	if _, ok := manaPain(a); !ok {
		return false
	}
	return a.Cost == (kop.Cost{Flags: kop.CostTap}) && a.ManaN <= 1 && a.ManaColors != 0 && a.AFlags&^kop.AfManaChoice == 0 &&
		a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && !a.ManaEach && !a.ManaDeferred && a.SplitGate == 0 && a.ManaHasteValid == 0 &&
		a.ManaRider == 0 && a.ActZone == kop.ZoneNone && a.Gate == (kop.Gate{}) && !a.Exhaust && a.Once == 0
}

// atkTaxOnlyTwo reports whether second is card c's last mana ability.
func atkTaxOnlyTwo(c *kop.CardDef, second int) bool {
	for j := second + 1; j < len(c.Abilities); j++ {
		if c.Abilities[j].Kind == kop.AbMana && !atkTaxUnlisted(&c.Abilities[j]) {
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
func (s *State) atkTaxCharge(k *prep, sl Slot, tgt TCode) (n int, ok bool) {
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
			// A RememberingAttacker$ charge (kop.RefTaxedAttacker) reads the
			// priced creature's own counters: gorge attackUnlessCharge binds
			// ctx.Remembered = [{Obj: attacker}] and evaluates X live.
			c := s.ctxStatic(st)
			c.taxed1 = s.perms[sl].obj + 1
			v = s.evalV(k, a.Static.VA, c)
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
func (s *State) atkTaxScope(k *prep, st *kop.Static, ctrl uint8, tgt TCode) bool {
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
func (s *State) atkTaxShape(k *prep, sl Slot) (col, n, pass int, ok bool) {
	if DebugAtkTaxCountUnlisted {
		if col, n, ok = s.atkTaxDebugUnlisted(k, sl, false); ok {
			return col, n, atkPassFixed, true
		}
	}
	if DebugAtkTaxCountRestricted {
		if col, n, ok = s.atkTaxDebugUnlisted(k, sl, true); ok {
			return col, n, atkPassFixed, true
		}
	}
	pc := s.manaPCGen(k, sl)
	cd := &k.t.Cards[s.cardAt(sl)]
	a := manaAbView(cd, pc)
	if a != nil && atkTaxPaidAb(a) {
		return s.atkTaxPaid(k, sl, pc, a)
	}
	if a != nil && atkTaxRestricted(a) {
		return 0, 0, 0, false
	}
	if !s.canTapForManaPC(k, sl, pc) {
		return 0, 0, 0, false
	}
	if a == nil || a.Kind != kop.AbMana {
		// a granted basic land type's intrinsic view (a card whose own
		// abilities name no mana ability: manaAb 0 marks the intrinsic slot,
		// and ability 0 of a card with abilities is not one — Ashaya's own
		// statics): the view's facts name the production, and the window
		// lists it as the fixed single colour it is
		if pc.blgFirst == 0 {
			return 0, 0, 0, false
		}
		m := uint8(pc.manaCol) & 0x3f
		if m == 0 {
			return 0, 0, 0, false
		}
		return bits.TrailingZeros8(m), int(pc.manaN), atkPassFixed, true
	}
	if a.ManaIdentity {
		// gorge's attack window excludes the commander-identity shape: the
		// parser claims no colour for it (rules attackChoiceManaSources)
		return 0, 0, 0, false
	}
	m := uint8(a.ManaColors) & 0x3f
	if atkTaxReflAb(a) {
		// the view's colours are what the opponents' lands could produce now
		// (manaReflView); none and gorge lists no ability (considerReflected)
		if m = uint8(pc.manaCol) & 0x3f; m == 0 {
			return 0, 0, 0, false
		}
		return bits.TrailingZeros8(m), int(a.ManaN), atkPassFixed, true
	}
	if a.ManaEach && atkTaxEachAb(a) {
		if DebugAtkTaxEachOne {
			return bits.TrailingZeros8(m), int(a.ManaN), atkPassFixed, true
		}
		return bits.TrailingZeros8(m), int(a.ManaN) * bits.OnesCount8(m), atkPassFixed, true
	}
	switch {
	case a.AFlags&kop.AfManaChosen != 0:
		ch := s.perms[sl].chosen
		if ch >= 5 {
			return 0, 0, 0, false
		}
		m |= 1 << ch
	case a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(m) == 1:
		return bits.TrailingZeros8(m), int(a.ManaN), atkPassFixed, true
	case m == 0:
		return 0, 0, 0, false
	}
	return bits.TrailingZeros8(m), int(a.ManaN), atkPassChoice, true
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

// atkTaxTargets fills tg with the defenders attackerOptions offers the
// active player's creatures (each other seat still in the game, the
// planeswalkers it controls, the attackable battles) and returns how many.
func (s *State) atkTaxTargets(k *prep, tg *[MaxPerm + MaxSeats]TCode) int {
	n := 0
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == s.active || s.out(q) {
			continue
		}
		tg[n] = PlayerCode(q)
		n++
		for pm := s.ctrlM[q]; pm.any(); pm = pm.rest() {
			if w := pm.first(); s.typesAt(k, w)&kop.TPlaneswalker != 0 {
				tg[n] = TCode(s.perms[w].obj)
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
					tg[n] = TCode(s.perms[sl].obj)
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
// own units, gorge attackTaxed) and spent (the declared picks' Values). The
// declaration's picks are the lifted set (attackerOptions clears their
// pfAttacking marks while the offers are read), so served names them beside
// the marked pose attackers.
func (s *State) atkTaxRead(k *prep, tg []TCode, served slotMask) (budget int, taxed bool, spent int) {
	budget = s.manaBudget(k, s.active)
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
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 || served.has(sl) {
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
func (s *State) atkTaxFilter(k *prep, d *Decision, served slotMask) {
	var tg [MaxPerm + MaxSeats]TCode
	budget, taxed, spent := s.atkTaxRead(k, tg[:s.atkTaxTargets(k, &tg)], served)
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
// option list). Its candidates carry the plain branch's gates beside the
// price (combat.go): a scoped CantAttack static (Rite of the Raging Storm's
// tokens, combat_cantattack.go) or a sickness exemption against no live
// defender leaves the creature no pair, tax or no tax, so gorge's empty
// option list records the forced empty declaration without asking.
//
//go:noinline
func (s *State) atkTaxAnyOffer(k *prep) bool {
	var tg [MaxPerm + MaxSeats]TCode
	nt := s.atkTaxTargets(k, &tg)
	budget := s.manaBudget(k, s.active)
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.canAttack(k, sl) || s.atkHasteNoPair(k, sl) || s.cantAtkEveryPair(k, sl) {
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
func (s *State) atkTaxNext(k *prep, p uint8) (sl Slot, col, n, pass int) {
	var order [MaxPerm]Slot
	no := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
	passes := [3]int{atkPassFixed, atkPassPaid, atkPassChoice}
	if DebugAtkTaxChoiceFirst {
		passes = [3]int{atkPassChoice, atkPassFixed, atkPassPaid}
	}
	for _, want := range passes {
		for _, o := range order[:no] {
			if s.perms[o].flags&pfAttacking != 0 && (s.kw(k, o)&kop.KwVigilance == 0 || DebugAtkTaxWithholdVigilant) {
				continue
			}
			if c, u, ps, ok := s.atkTaxShape(k, o); ok && u > 0 && ps == want {
				return o, c, u, ps
			}
		}
	}
	return NoSlot, 0, 0, 0
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
	pend, pendSl := -1, NoSlot // the colour a pending reflected-mana ask (atkTaxAddMana) would add, and its source
	for {
		have := 0
		for _, v := range pool {
			have += int(v)
		}
		if have >= total {
			break
		}
		sl, col, n, pass := s.atkTaxNext(k, p)
		if sl == NoSlot {
			for m := s.ctrlM[p]; m.any(); m = m.rest() {
				if a := m.first(); s.perms[a].flags&pfAttacking != 0 {
					s.leaveCombat(a)
				}
			}
			s.atkTaxPendingAsk(k, p, pend, pendSl)
			return
		}
		pend, pendSl = s.atkTaxTap(k, p, sl, col, n, pass), sl
	}
	for total > 0 {
		c := kop.ManaCost{Generic: uint8(min(total, 255))}
		total -= int(c.Generic)
		s.payFromPool(p, &c, 0)
	}
}

// atkTaxDebugUnlisted is the debug switches' reading of the untapped source at
// sl: its first mana ability's lowest colour and amount when that ability is
// unlisted (DebugAtkTaxCountUnlisted) or, restricted set, restricted
// (DebugAtkTaxCountRestricted).
func (s *State) atkTaxDebugUnlisted(k *prep, sl Slot, restricted bool) (col, n int, ok bool) {
	if s.perms[sl].flags&pfTapped != 0 {
		return 0, 0, false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	f := atkTaxFirstMana(cd)
	if f < 0 || restricted != atkTaxRestricted(&cd.Abilities[f]) || !atkTaxUnlisted(&cd.Abilities[f]) {
		return 0, 0, false
	}
	m := uint8(cd.Abilities[f].ManaColors) & 0x3f
	if m == 0 {
		return 0, 0, false
	}
	return bits.TrailingZeros8(m), int(max(cd.Abilities[f].ManaN, 1)), true
}

// atkTaxTap is one tap the window's answer makes: the source at sl taps for
// its mana (atkTaxAddMana) or, a self-sacrifice source (atkPassPaid), adds its
// mana and is sacrificed with no tap (CR 605.1a; fundSacOnly's order). It
// returns the colour of the reflected-mana ask the tap leaves posed, -1 for none.
func (s *State) atkTaxTap(k *prep, p uint8, sl Slot, col, n, pass int) int {
	if pass == atkPassPaid || DebugAtkTaxPaidLast && s.atkTaxIsPaid(k, sl) {
		s.poolAdd(p, col, n)
		s.artTag(k, p, sl, col)
		s.sacFilterSource(k, sl)
		return -1
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	return s.atkTaxAddMana(k, p, sl, col, n)
}

// atkTaxIsPaid reports whether the source at sl is a self-sacrifice source.
func (s *State) atkTaxIsPaid(k *prep, sl Slot) bool {
	a := manaAbView(&k.t.Cards[s.cardAt(sl)], s.manaPCGen(k, sl))
	return a != nil && atkTaxPaidAb(a)
}

// atkTaxPendingAsk answers the colour ask an abandoned window leaves posed:
// no tap ask displaces it, so gorge answers it with its first colour.
func (s *State) atkTaxPendingAsk(k *prep, p uint8, pend int, sl Slot) {
	if pend >= 0 {
		s.poolAdd(p, pend, 1)
		s.artTag(k, p, sl, pend)
	}
}

// atkTaxAddMana floats the mana the tapped source at sl made: n of col, or
// for an all-at-once source (atkTaxEachAb) each of its colours' amount. A
// reflected source with several colours on offer (atkTaxReflAb) makes none:
// gorge's resolution poses a colour ask (askManaColor) the window's next tap
// ask displaces, and the colour it returns is the ask's first (the lowest).
func (s *State) atkTaxAddMana(k *prep, p uint8, sl Slot, col, n int) (pend int) {
	pc := s.manaPCGen(k, sl)
	a := manaAbView(&k.t.Cards[s.cardAt(sl)], pc)
	if a != nil && atkTaxReflAb(a) && bits.OnesCount8(uint8(pc.manaCol&0x3f)) >= 2 && !DebugAtkTaxReflPin {
		return col
	}
	if a != nil && !DebugAtkTaxEachOne && atkTaxEachAb(a) {
		for c, m := 0, uint8(a.ManaColors)&0x3f; m != 0; c, m = c+1, m>>1 {
			if m&1 != 0 {
				s.poolAdd(p, c, int(a.ManaN))
				s.artTag(k, p, sl, c)
			}
		}
		return -1
	}
	s.poolAdd(p, col, n)
	s.artTag(k, p, sl, col)
	return -1
}
