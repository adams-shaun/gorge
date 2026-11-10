package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Waterbend (CR 701.67): an activated ability's cost may include N generic
// that can be paid by tapping that many untapped artifacts and/or creatures
// instead. The offer credit (waterbendCredit) reduces the mana the ability
// is offered for; the picks are announced after the tap costs and before X,
// pay one generic each in pick order at payment, and cannot also be mana
// sources in the window. Gorge: raise_cost_extra.go withWaterbendOfferCredit,
// pay.ConvokeAsk (Kind "waterbend_generic"), cast_commit.go EmitConvokeTaps.

const pkWaterbend pickKind = 0x2000

// Debug hooks for the lockstep and mutation tests.
var (
	// DebugWaterbendNoCredit drops the offer credit.
	DebugWaterbendNoCredit bool
	// DebugWaterbendSrcTap offers a {T} ability's source as its own
	// waterbend pick (mutation).
	DebugWaterbendSrcTap bool
	// DebugWaterbendPaid reports a paid ability: its card, the waterbend
	// picks, and how many of them are mana sources.
	DebugWaterbendPaid func(card CardID, picks, manaPicks int)
)

// prepWaterbend sets anyWaterbend: some ability costs CostWaterbend, or some
// spell carries a RaiseCost Waterbend<N> (CardDef.SpellWaterbend).
func prepWaterbend(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].SpellWaterbend != 0 || t.Cards[i].OptWaterbend != 0 {
			g.anyWaterbend = true
			return
		}
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.Flags&kop.CostWaterbend != 0 {
				g.anyWaterbend = true
				return
			}
		}
	}
}

// waterbendCandidate reports whether sl can be tapped for a waterbend cost:
// untapped, an artifact or creature, and no attached bestow card (gorge also
// names a reconfigured card, which kgen does not lower). An equipped Equipment
// qualifies (CR 701.67a).
func (s *State) waterbendCandidate(k *prep, sl Slot) bool {
	pp := &s.perms[sl]
	if pp.flags&pfTapped != 0 || s.bestowAttached(k, sl) {
		return false
	}
	return s.typesAt(k, sl)&(kop.TArtifact|kop.TCreature) != 0
}

// waterbendManaSource reports whether sl is an untapped mana source the
// offer credit must not count: gorge's withWaterbendOfferCredit excludes every
// untapped permanent with a usable mana ability (untappedManaSource ->
// availableManaAbilities), so a summoning-sick creature whose mana ability
// needs its {T} is NOT a source this turn and does earn the credit
// (featManaActivatable carries the same tap-sick gate).
//
// availableManaAbilities also prices the ability's own mana: a paid ability
// (a filter's {1}: add any colour, a granted one) is a source only while the
// live pool can pay its fee. Barrels of Blasting Jelly is a waterbend pick at
// the start of a payment (the pool is empty) and a mana source once an earlier
// tap has floated its fee; the prefix search prices the credit again at every
// node for exactly that reason (grantSearch's dynamic cost).
func (s *State) waterbendManaSource(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 ||
		k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) || !s.featManaActivatable(k, sl) {
		return false
	}
	p := pm.ctrl
	if k.gen != nil && s.manaGranted(k, sl) {
		gi := s.grantIdx(k, sl)
		if gi < 0 {
			return false
		}
		r := k.gen.grantOf[gi]
		return s.poolPays(p, &k.t.Cards[r.card].Abilities[r.ab].Cost.Mana)
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	sick := s.activationSick(k, sl)
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if a.Cost.Flags&kop.CostOncePerTurn != 0 && j < 8 && pm.actUsed&(1<<j) != 0 {
			continue
		}
		if a.Cost.Flags&kop.CostTap != 0 && sick {
			continue // a summoning-sick creature's {T} ability is no source (gorge gates each ability)
		}
		if s.poolPays(p, &a.Cost.Mana) {
			return true
		}
	}
	return false
}

// grantWaterbendFull is a waterbend activation's cost before the offer
// credit, for the prefix search to price again at every node (grantSrch.dyn).
func (s *State) grantWaterbendFull(k *prep, a Action) (kop.ManaCost, bool) {
	if a.Kind() != AkActivate {
		return kop.ManaCost{}, false
	}
	obj := Obj(a.Source())
	ob := &s.objs[obj]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return kop.ManaCost{}, false
	}
	ab := int(a.Mode())
	cd := &k.t.Cards[s.cardAt(ob.slot)]
	if ab < 0 || ab >= len(cd.Abilities) || cd.Abilities[ab].Cost.Flags&kop.CostWaterbend == 0 {
		return kop.ManaCost{}, false
	}
	cost, _, ok := s.activationMana(k, obj, a.Mode())
	return cost, ok
}

// grantDynamicCost is grantWaterbendFull's cost with the offer credit
// recomputed on this search node: the live pool decides whether a paid mana
// ability (Barrels of Blasting Jelly) or a spent once-a-turn one is a source,
// exactly as gorge's potentialPlayVerdict reprices each node.
func (s *State) grantDynamicCost(k *prep, p uint8, a Action, full kop.ManaCost) kop.ManaCost {
	obj := Obj(a.Source())
	ob := &s.objs[obj]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return full
	}
	ab := int(a.Mode())
	cd := &k.t.Cards[s.cardAt(ob.slot)]
	if ab < 0 || ab >= len(cd.Abilities) {
		return full
	}
	return s.waterbendCredit(k, p, ob.slot, &cd.Abilities[ab], full)
}

// waterbendCredit returns cost less the greedy waterbend offer credit for
// ability a: in p's battlefield order, each untapped artifact/creature that is
// not an untapped mana source pays one generic, up to the ability's amount. The
// source counts unless its own {T} is part of the cost (CR 701.67a, 118.3).
// Gorge: withWaterbendOfferCredit.
func (s *State) waterbendCredit(k *prep, p uint8, src Slot, a *kop.Ability, cost kop.ManaCost) kop.ManaCost {
	if a.Cost.Flags&kop.CostTap == 0 {
		src = NoSlot
	}
	n := int(a.Cost.N)
	if waterbendIsX(a) {
		n = costXMin(&a.Cost) // gorge want = WaterbendPartX*xMin: the cost priced at the floor
	}
	return s.waterbendCreditN(k, p, src, n, cost)
}

// waterbendIsX reports an ability's Waterbend<X> cost (Katara, Water Tribe's
// Hope): CostWaterbend with N 0 beside the {X}, the announced X the amount.
func waterbendIsX(a *kop.Ability) bool {
	return a.Cost.Flags&kop.CostWaterbend != 0 && a.Cost.N == 0 && a.Cost.Mana.X != 0
}

// waterbendXBuilding reports whether the activation being built pays a
// Waterbend<X> cost.
func (s *State) waterbendXBuilding(k *prep) bool {
	if k.gen == nil || !k.gen.anyWaterbend {
		return false
	}
	e := &s.bld().e
	return e.kind == seAbility && e.ab != 255 && e.ab < abDelayed && waterbendIsX(&k.t.Cards[e.card].Abilities[e.ab])
}

// waterbendXCost folds x into cost and pays one generic of it per announced
// waterbend pick, false when the picks exceed x (gorge waterbendCap: the taps
// pay only the waterbend amount, CR 701.67a) or the generic runs out.
func (s *State) waterbendXCost(cost kop.ManaCost, x int) (kop.ManaCost, bool) {
	n := s.waterbendPickCount()
	cost = xFolded(cost, x)
	if n > x || n > int(cost.Generic) {
		return cost, false
	}
	cost.Generic -= uint8(n)
	return cost, true
}

// waterbendXCharge is the charge of a Waterbend<X> activation being paid: the
// announced X folded and the picks' generic taken off (gorge applyConvoke
// after WithX); any other cost goes back as it came.
func (s *State) waterbendXCharge(k *prep, e *stackEntry, cost kop.ManaCost) kop.ManaCost {
	if !s.waterbendXBuilding(k) {
		return cost
	}
	if c, ok := s.waterbendXCost(cost, int(e.x)); ok {
		return c
	}
	return cost
}

// waterbendOwnPart reports a spell's own waterbend amount: card c's
// SpellWaterbend, owed only by the card's owner (the static's Activator$ You;
// o is NoObj when the card is priced bare, which counts as the owner's).
func (s *State) waterbendOwnPart(k *prep, p uint8, c CardID, o Obj) int {
	n := int(k.t.Cards[c].SpellWaterbend)
	if n == 0 || o != NoObj && s.objs[o].owner() != p {
		return 0
	}
	return n
}

// waterbendCastPart is the waterbend amount the cast being built owes: the
// card's own RaiseCost part plus, on the kicked cast, its OptionalCost
// Waterbend<N> (gorge foldRaiseExtra folds both into mods.Waterbend).
func (s *State) waterbendCastPart(k *prep, e *stackEntry) int {
	n := s.waterbendOwnPart(k, e.ctrl, e.card, e.obj)
	if e.flags&sfKicked != 0 {
		n += int(k.t.Cards[e.card].OptWaterbend)
	}
	return n
}

// spellWaterbends: card c's cast may owe a waterbend (act_waterbend.go).
func spellWaterbends(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	return cd.SpellWaterbend != 0 || cd.OptWaterbend != 0
}

// optWaterbendAffords prices the kicked cast of an OptionalCost Waterbend<N>
// card: its cost with the {N} (CardDef.Kicker), reduced, less the greedy
// offer credit for the whole waterbend amount (gorge mana.go's
// withWaterbendOfferCredit over base.Waterbend), against p's sources.
func (s *State) optWaterbendAffords(k *prep, p uint8, c CardID, o Obj) bool {
	var av avail
	s.availFor(k, p, &av)
	cd := &k.t.Cards[c]
	cost := s.adjustFit(k, &av, p, c, o, addCost(k.cards[c].cost, cd.Kicker), false)
	cost = s.waterbendCreditN(k, p, NoSlot, s.waterbendOwnPart(k, p, c, o)+int(cd.OptWaterbend), cost)
	return av.affords(&cost, pipMaskOf(&cost), 0)
}

// waterbendSpellCredit is waterbendCredit for a spell's own Waterbend<N>:
// cost less the greedy offer credit (gorge withWaterbendOfferCredit; a spell
// is on no battlefield, so it never excludes a source).
func (s *State) waterbendSpellCredit(k *prep, p uint8, c CardID, o Obj, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyWaterbend {
		return cost
	}
	return s.waterbendCreditN(k, p, NoSlot, s.waterbendOwnPart(k, p, c, o), cost)
}

// waterbendCreditN is the greedy credit for an amount of n, src (when not
// NoSlot) being the permanent whose own {T} is part of the cost.
func (s *State) waterbendCreditN(k *prep, p uint8, src Slot, n int, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyWaterbend || DebugWaterbendNoCredit || n <= 0 || cost.Generic == 0 {
		return cost
	}
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ord[:], 0)
	credit := 0
	for _, sl := range ord[:cnt] {
		if credit >= n || credit >= int(cost.Generic) {
			break
		}
		if sl == src || !s.waterbendCandidate(k, sl) || s.waterbendManaSource(k, sl) {
			continue
		}
		credit++
	}
	cost.Generic -= uint8(credit)
	return cost
}

// waterbendPlanCost is cost less the offer credit when object o's ability ab costs a
// waterbend: what a source cannot fund, the taps may, so gorge's planner prices the
// credited cost before it calls a play unpayable (planAbilityPaymentLifePips).
func (s *State) waterbendPlanCost(k *prep, p uint8, o Obj, ab uint8, cost kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if !k.gen.anyWaterbend || ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return cost
	}
	a := &k.t.Cards[s.cardAt(ob.slot)].Abilities[ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return cost
	}
	return s.waterbendCredit(k, p, ob.slot, a, cost)
}

// waterbendPoolPaidOffer reports whether the activation being built is a
// waterbend ability whose offer credit (waterbendCredit) the floating pool
// alone pays: gorge's offer gate priced that credited cost, so it is an
// offered option, not a potential play the witness lowers. The picks are
// asked next and waterbendManualPay settles or aborts the payment.
func (s *State) waterbendPoolPaidOffer(k *prep, p uint8, cost kop.ManaCost) bool {
	credited, ok := s.waterbendBuildCredit(k, p, cost)
	return ok && s.poolPays(p, &credited)
}

// waterbendBuildCredit is waterbendCredit for the activation being built; false when
// it isn't a waterbend ability, and cost goes back as it came. A witness is priced on
// the credit too (gorge planAbilityPaymentLifePips).
func (s *State) waterbendBuildCredit(k *prep, p uint8, cost kop.ManaCost) (kop.ManaCost, bool) {
	if k.gen == nil || !k.gen.anyWaterbend {
		return cost, false
	}
	e := &s.bld().e
	if e.kind != seAbility || e.ab == 255 || e.ab >= abDelayed {
		return cost, false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return cost, false
	}
	return s.waterbendCredit(k, p, s.srcSlot(), a, cost), true
}

// waterbendApplyPicks subtracts the announced waterbend picks from cost in
// pick order (gorge applyConvoke's waterbend branch).
func (s *State) waterbendApplyPicks(cost kop.ManaCost) kop.ManaCost {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend && cost.Generic > 0 {
			cost.Generic--
		}
	}
	return cost
}

func (s *State) waterbendPicked(o Obj) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i] == pkWaterbend {
			return true
		}
	}
	return false
}

func (s *State) waterbendPickCount() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend {
			n++
		}
	}
	return n
}

// waterbendPart is the waterbend cost part of the cast or activation being
// built: the amount n, whether the source's {T} is in the cost (it then is
// no pick), the part count the ask is capped at (slots), and an X in the cost.
type waterbendPart struct {
	n      int
	slots  int
	tapSrc bool
	hasX   bool
}

// waterbendPartFor is the one reader of a build's waterbend part, for an
// activated ability's Waterbend<N> cost or a spell's own RaiseCost
// (CardDef.SpellWaterbend). false when the build pays no waterbend, or its
// cost holds no generic for the picks to pay (gorge ConvokeAsk).
func (s *State) waterbendPartFor(k *prep, e *stackEntry) (waterbendPart, bool) {
	switch e.kind {
	case seAbility:
		if e.ab == 255 || e.ab >= abDelayed {
			return waterbendPart{}, false
		}
		a := &k.t.Cards[e.card].Abilities[e.ab]
		if waterbendIsX(a) {
			// gorge ConvokeAsk with HasX and WaterbendOpen: Max is the
			// option count, no amount or slot cap (the X ask bounds the taps)
			return waterbendPart{n: maxXAsk, slots: maxXAsk, tapSrc: a.Cost.Flags&kop.CostTap != 0, hasX: true}, true
		}
		if a.Cost.Flags&kop.CostWaterbend == 0 || a.Cost.N <= 0 || a.Cost.Mana.Generic == 0 {
			return waterbendPart{}, false
		}
		return waterbendPart{n: int(a.Cost.N), slots: int(a.Cost.Mana.Generic), tapSrc: a.Cost.Flags&kop.CostTap != 0, hasX: a.Cost.Mana.X != 0}, true
	case seSpell:
		if e.obj == NoObj || !spellWaterbends(k, e.card) {
			return waterbendPart{}, false
		}
		n := s.waterbendCastPart(k, e)
		if n == 0 {
			return waterbendPart{}, false
		}
		if s.bld().donePicks&pkWaterbend != 0 || s.bld().bfl&bflCostFixed != 0 {
			return waterbendPart{n: n}, true // the ask is over: the build's fixed cost already holds the picks
		}
		cost := s.waterbendFullCost(k)
		if cost.Generic == 0 {
			return waterbendPart{}, false
		}
		return waterbendPart{n: n, slots: cost.MV()}, true
	}
	return waterbendPart{}, false
}

// waterbendFullCost is the build's cost before any waterbend pick. A probe:
// the fixedGen cache keeps the build's own price.
func (s *State) waterbendFullCost(k *prep) kop.ManaCost {
	b := s.bld()
	kinds, fg := b.pickKs, b.fixedGen
	for i := range b.pickKs[:b.pickN] {
		if b.pickKs[i] == pkWaterbend {
			b.pickKs[i] = 0
		}
	}
	cost := s.buildCost(k)
	b.pickKs, b.fixedGen = kinds, fg
	return cost
}

// waterbendOptions poses the next waterbend pick: one option per eligible
// permanent in battlefield order, Done closing the announcement. The ask
// commits once it reaches gorge's Max = min(#options, N); a first ask with
// no option is skipped. Done waits for the taps the payment can't do without
// (waterbendTapsNeeded).
//
//go:noinline
func (s *State) waterbendOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.gen == nil || !k.gen.anyWaterbend || s.bld().donePicks&pkWaterbend != 0 {
		return false
	}
	part, ok := s.waterbendPartFor(k, e)
	if !ok {
		return false
	}
	n := part.n
	// gorge castasks.go:76-80 (SourceTaps, b986f411f): a {T} ability's
	// source taps for {T}, so it is never one of its own waterbend picks
	// (CR 118.3). Same rule as the tap-X pick (cast.go:1578).
	self := NoSlot
	if part.tapSrc && !DebugWaterbendSrcTap {
		self = s.srcSlot()
	}
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	offered := 0
	for _, sl := range ord[:cnt] {
		if !s.waterbendCandidate(k, sl) || sl == self {
			continue
		}
		if s.pickedKind(s.perms[sl].obj, pkTapX) {
			continue
		}
		offered++
	}
	maxPicks := offered
	if n < maxPicks {
		maxPicks = n
	}
	if part.slots < maxPicks {
		maxPicks = part.slots
	}
	if s.waterbendPickCount() >= maxPicks {
		return false
	}
	d.N = 0
	d.Kind, d.Sub, d.Player = DkChoose, ChWaterbend, e.ctrl
	pose := false
	for _, sl := range ord[:cnt] {
		if !s.waterbendCandidate(k, sl) || sl == self {
			continue
		}
		o := s.perms[sl].obj
		if s.waterbendPicked(o) || s.pickedKind(o, pkTapX) {
			continue
		}
		pose = true
		if d.N >= MaxOptions-1 {
			s.capFull(StallTapPick)
			break
		}
		d.add(MakeAction(AkChoose, uint8(o), 0, 0))
	}
	if !pose && s.waterbendPickCount() == 0 {
		return false
	}
	if s.waterbendPickCount() >= s.waterbendTapsNeeded(k, e, part) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	s.bld().pickK = pkWaterbend
	return true
}

// waterbendTapsNeeded is the fewest taps the announcement can make and still
// leave the mana window able to finish (CR 601.2g-h, 701.67a). Gorge reads it
// once, before any pick, so a pick still counts here as the mana source it would
// otherwise be. An unknown amount or a window the walk can't price needs none.
// Gorge: waterbendTapsNeeded.
func (s *State) waterbendTapsNeeded(k *prep, e *stackEntry, part waterbendPart) int {
	if part.hasX {
		return 0
	}
	// CR 602.2b: the ability's own {T} taps for its cost, so it is never a
	// window source this probe may count (gorge castWindowUnits withholds
	// pc.cost.Tap's source, cast_paymentplan.go). The tap-cost source's
	// granted any-colour ability is not the unit the pool lacks, so the
	// census and the window walk below still demand the waterbend pick the
	// payment needs. The cost itself prices unmasked: a reduction read over
	// the battlefield counts the source (gorge manaToPay).
	occ, hideSrc := s.occ, NoSlot
	if part.tapSrc {
		if sl := s.srcSlot(); sl != NoSlot {
			occ, hideSrc = s.occ, sl
		}
	}
	if hideSrc != NoSlot {
		s.occ = occ.without(hideSrc)
	}
	complete := s.windowCensusComplete(k, e)
	if hideSrc != NoSlot {
		s.occ = occ
	}
	if !complete {
		return 0
	}
	b := s.bld()
	kinds := b.pickKs
	for i := range b.pickKs[:b.pickN] {
		if b.pickKs[i] == pkWaterbend {
			b.pickKs[i] = 0
		}
	}
	defer func() { b.pickKs = kinds }()
	cost := s.buildCost(k)
	if hideSrc != NoSlot {
		s.occ = occ.without(hideSrc)
		defer func() { s.occ = occ }()
	}
	for n := 0; n <= min(part.n, int(cost.Generic)); n++ {
		c := cost
		c.Generic -= uint8(n)
		ok, known := s.windowDry(k, e.ctrl, &c, b.bfl&bflAnyColor != 0)
		if !known {
			return 0
		}
		if ok {
			return n
		}
	}
	return 0
}

// sacAnyManaGap: p controls a source with a self-sacrifice any-colour mana
// ability (Produced$ Any, the Treasure token's shape) that a Can'tSacrifice
// static blocks (Quark, Humble Proprietor's "You can't sacrifice Treasure
// tokens"). The shared window census prices a usable self-sacrifice source but
// skips a blocked one, while the probe still walks the ability and cannot
// price its CountsAny production (gorge castWindowSourceExcluded, CR 605.1b):
// the census is incomplete and a pool-short target menu keeps its candidates.
// A usable Treasure is priced, a tapped {T} source is not offered, and a Combo
// WUBRG production has priced counts, so none of those is a gap.
//
//go:noinline
func (s *State) sacAnyManaGap(k *prep, p uint8) bool {
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) || !s.sacBlocked(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 || a.Cost.Flags&kop.CostSacSelf == 0 {
				continue
			}
			if a.ManaColors != kop.ColAny || a.AFlags&(kop.AfManaChosen|kop.AfManaChoice|kop.AfManaEachColor|kop.AfManaReflect) != 0 {
				continue
			}
			if a.Cost.Flags&kop.CostTap != 0 {
				if pm := &s.perms[sl]; pm.flags&pfTapped != 0 ||
					s.activationSick(k, sl) {
					continue
				}
			}
			return true
		}
	}
	return false
}

// windowPricesRestr: gorge's window probe prices restricted mana ability a of the source
// at sl for a payment its restriction admits (castWindowRestrictedSpendable), so the
// census holds its units (The Emperor of Palamecia's {U} or {R} for a noncreature
// spell). An amount read off the board is priced only when it is positive.
func (s *State) windowPricesRestr(k *prep, sl Slot, a *kop.Ability) bool {
	if !restrSrcAb(a) {
		return false
	}
	return a.AFlags&kop.AfManaDynamic == 0 || s.dynManaN(k, a, sl) > 0
}

// windowCensusComplete reports whether gorge's window census prices every source of
// the payer: a permanent that can tap for a restricted mana ability this payment may
// spend, and the window probe does not price, understates what the window can produce
// (CR 106.6). Gorge: castWindowCensusComplete.
func (s *State) windowCensusComplete(k *prep, e *stackEntry) bool {
	// a source the build committed (a convoke pick, withheld from s.occ) is
	// no census source at all (gorge's census skips convokeCommitted)
	if k.tapMana && s.tapXCensusGap(k, e.ctrl).and(s.occ).any() {
		return false // its tapXType cost is no shape the census prices (gorge castWindowSourceExcluded)
	}
	if !k.gen.anyRestr && !k.gen.anyRestrSrc && !k.gen.anyAnyR && !k.gen.anyTapExtra && !k.gen.anyEachCol &&
		!k.gen.anyTapsDefer && !k.gen.anySelfTaps {
		return true
	}
	d := s.payDescNow(NoObj)
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	for _, sl := range ord[:cnt] {
		pm := &s.perms[sl]
		lost := s.genLost(k, sl)
		// A source that lost all abilities can still hold a mana ability an
		// effect grants it in the same layer (Honest Work's Humble Merchant):
		// gorge's census lists that source too, so a lost printed source is
		// skipped only when no grant survives.
		if pm.flags&pfTapped != 0 || s.activationSick(k, sl) ||
			lost && !s.manaGranted(k, sl) {
			continue
		}
		// gorge's manaProductionAltered (paymentPlanSourceInterference): a mana
		// source whose {T} fires a Defer-class Taps watcher is not a unit the
		// census prices, so the window's reach is unprovable (CR 605.1b). The
		// watcher may be another object's or the source's own (Compassionate
		// Healer's self watcher beside Great Divide Guide's granted {T} ability).
		if s.windowCanTap(k, sl) &&
			(k.gen.anyTapsDefer && s.tapsDeferred(k, e.ctrl, sl) || k.gen.anySelfTaps && s.selfTapsDefer(k, sl)) {
			return false
		}
		if !lost {
			cd := &k.t.Cards[s.cardAt(sl)]
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				if a.Kind == kop.AbMana && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) && s.restrSrcAdmits(k, e.ctrl, sl, a, &d) &&
					!s.windowPricesRestr(k, sl, a) {
					return false
				}
				// An unpriced each-colour source (Bloom Tender): the probe admits its
				// ability and prices none of its production (gorge's census leaves no
				// unit for a Produced$ Special EachColorAmong), so the census
				// understates what the window can offer (gorge castWindowSourceExcluded,
				// CR 601.2g). A slot the census does price carries no such gap (gorge
				// skips priced sources); a CantBeActivated lock keeps the ability off
				// gorge's probe (availableManaAbilitiesForWindow).
				if k.gen.anyEachCol && k.gen.has(s.cardAt(sl), gfEachCol) && int(s.manaPCAt(k, sl).manaAb) < 0 &&
					!(k.gen.anyCantAct && s.actForbidden(k, sl, true)) {
					return false
				}
			}
		}
		// A tap trigger that adds mana to this source's production is one the
		// window census doesn't price (gorge manaProductionAltered, CR 605.1b).
		// Read the ability the window would activate: a granted mana ability is
		// no printed one, and a tap extra can still ride it (Honest Work's
		// Humble Merchant under Badgermole Cub).
		if k.gen.anyTapExtra {
			if _, hit := s.srcExtraHit(k, sl, s.windowGrantPC(k, sl)); hit {
				return false
			}
		}
	}
	return true
}

// selfTapsDefer reports whether a Taps watcher on sl itself would fire when
// the window taps it for mana. kgen lowers a self Taps trigger only on a card
// with no printed mana ability, so this can only matter to a granted mana
// ability; gorge's paymentPlanTapObservers defers such a source for any body
// the census doesn't price. It skips the carriers that walk skips: a permanent
// that lost all abilities has no printed watcher left (Honest Work's Humble
// Merchant keeps only the granted {T}), and a latent line is never subscribed
// (tapsWatchClass's genLost/AfLatent gates).
func (s *State) selfTapsDefer(k *prep, sl Slot) bool {
	if s.genLost(k, sl) {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags&kop.TfSelf != 0 &&
			a.AFlags&kop.AfLatent == 0 &&
			(a.Trig.Zone == kop.ZoneNone || a.Trig.Zone == kop.ZoneBattlefield) {
			return true
		}
	}
	return false
}

// windowTapXLive: p's tap-another-permanent mana sources that can be activated
// now (Supportive Parents, Springleaf Drum). The window census prices no
// tapXType cost, so each leaves it incomplete.
func (s *State) windowTapXLive(k *prep, p uint8) slotMask {
	if !k.tapMana {
		return slotMask{}
	}
	return s.scriptTapX(k, p)
}

// waterbendDone closes the waterbend pick; the next buildStep advances past
// bsTapCost.
func (s *State) waterbendDone() {
	s.bld().donePicks |= pkWaterbend
	s.bld().pickK, s.bld().pickLeft = 0, 0
}

// waterbendCastWindow pays a waterbend cast's mana (manaStep) as a convoke
// cast's: gorge's planner declines the cast ("shape:contribution"), so its
// manual CR 601.2g window offers the first source useful to the composed
// cost before the picks (paymentUsefulColours reads manaToPay, which the
// picks don't reduce), repeating until the post-pick cost is paid; a pick is
// no source of it (convokeCommitted). payWaterbend taps the picks after.
func (s *State) waterbendCastWindow(k *prep, e *stackEntry, cost, full *kop.ManaCost, anyColor bool) {
	var hide slotMask
	for i := 0; i < int(s.bld().pickN); i++ {
		if o := s.bld().picks[i]; s.bld().pickKs[i] == pkWaterbend && s.objs[o].zone == kop.ZoneBattlefield {
			hide = hide.with(s.objs[o].slot)
		}
	}
	occ := s.occ
	s.occ = s.occ.andNot(hide)
	s.windowTapUseful(k, e.ctrl, cost, full, true, anyColor, s.bld().pipN == 0, false)
	s.occ = occ
}

// payWaterbend taps the announced permanents in pick order after the mana
// cost is paid (gorge EmitConvokeTaps); a pick that has left the
// battlefield is skipped.
func (s *State) payWaterbend(k *prep) {
	picks, manaPicks := 0, 0
	tb := false
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkWaterbend {
			continue
		}
		picks++
		o := s.bld().picks[i]
		if s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		sl := s.objs[o].slot
		if s.canTapForMana(k, sl) {
			manaPicks++
		}
		was := s.perms[sl].flags & pfTapped
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		if was == 0 {
			if k.gen != nil && s.subs[kop.TrigTaps].any() {
				tb = tb || s.tapBatchOpen(k) // the announcement's taps are one tapping action (gorge EmitConvokeTaps; trig_tapall.go)
				s.fireTaps(k, sl)
			}
			s.markTapped(k, sl) // only a tap that happens marks
		}
	}
	s.tapBatchClose(tb)
	if e := &s.bld().e; DebugWaterbendPaid != nil && (e.kind == seSpell || e.ab != 255 && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostWaterbend != 0) {
		DebugWaterbendPaid(s.bld().e.card, picks, manaPicks)
	}
}

// waterbendManualPay reports whether the ability entry e pays its cost the
// way gorge's fallback window does. The offer arms a plan for the printed cost,
// or for the cost less the offer credit when the sources cannot fund the printed
// one (potential_plan.go). Once the picks settle the cost, gorge plans again for
// it without them, and falls back to the manual window only when no plan exists
// (rearmAbilityAutoPay): the window taps sources in battlefield order until the
// reduced cost is paid, filtering them by the composed full cost and excluding the
// picks (convokeCommitted). saved holds which picks holdCostTaps tapped. A plan
// that stands is tapped here, so the pool pays what follows.
func (s *State) waterbendManualPay(k *prep, e *stackEntry, saved *[len(build{}.picks)]uint8) bool {
	if k.gen == nil || !k.gen.anyWaterbend || e.kind != seAbility || e.ab == 255 {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return false
	}
	full := a.Cost.Mana
	if s.waterbendPickCount() == 0 {
		// every credited pick declined: the offer gate priced the reduced
		// cost, so the printed cost may be unpayable and gorge's window then
		// aborts the activation (CR 733.1) after tapping as it walked. The
		// probe is gorge's planner, which holds printed sources only: a
		// granted source is no unit, so a cost only it pays takes the window
		// (pay.PaymentAbility; mana_grantedoffers.go)
		cost := s.buildCost(k)
		var chk planner
		chk.dry, chk.noGrant = true, true
		s.tapForPK(k, e.ctrl, &cost, int(e.x), NoObj, false, false, &chk)
		return chk.needT > 0 && !chk.found
	}
	if s.windowPoolPays(e.ctrl, &full, false) {
		return false
	}
	s.releaseCostTaps(k, saved)
	credited := s.waterbendCredit(k, e.ctrl, s.srcSlot(), a, full)
	s.retakeCostTaps(saved)
	cost := s.buildCost(k)
	if !s.abilityPlanSettles(k, e, full, credited, cost, saved) {
		return true
	}
	if s.waterbendWitnessStands(k, e, full, credited, cost, saved) {
		return false
	}
	s.tapFor(k, e.ctrl, &cost, int(e.x), NoObj)
	return false
}

// waterbendWitnessStands taps the witness the offer armed when gorge's payment keeps
// it. The plan was ranked as the activation began, with every pick still untapped
// and so still a source (a Bender's Waterskin is one), for the offer credit's price
// because the sources could not fund the printed cost. When the picks bring the
// cost to exactly that price and the witness taps none of them, the check finds
// it unchanged (paymentPlanCheckUnits) and it runs as armed; otherwise gorge plans
// again without the picks, which tapFor does. It reports whether the armed witness
// was tapped.
func (s *State) waterbendWitnessStands(k *prep, e *stackEntry, full, credited, cost kop.ManaCost, saved *[len(build{}.picks)]uint8) bool {
	if credited != cost || credited.X != 0 || full.X != 0 {
		return false
	}
	s.releaseCostTaps(k, saved)
	var chk planner
	chk.dry, chk.noGrant = true, true
	s.tapForPK(k, e.ctrl, &full, 0, NoObj, false, false, &chk)
	if chk.needT == 0 || chk.found {
		s.retakeCostTaps(saved) // armed for the printed cost, which the picks changed
		return false
	}
	probe := s.fork()
	s.tapFor(k, e.ctrl, &credited, 0, NoObj)
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend && s.perms[s.objs[s.bld().picks[i]].slot].flags&pfTapped != 0 {
			s.CopyFrom(probe) // the witness names a pick: gorge plans again
			s.unfork()
			s.retakeCostTaps(saved)
			return false
		}
	}
	s.unfork()
	s.retakeCostTaps(saved)
	return true
}

// waterbendTargetAbort reports whether the pending waterbend activation can
// no longer be completed: the cost less the announced picks is unpayable from
// the pool and the CR 601.2g window (the picks are withheld, as at payment).
// Gorge's affordableTargetCandidates probes the same window after the picks
// are committed; every candidate is withheld, so a required target reverses
// the activation ("cast aborted: no legal target") and an up-to target asks
// nothing and reverses at payment.
func (s *State) waterbendTargetAbort(k *prep) bool {
	if k.gen == nil || !k.gen.anyWaterbend {
		return false
	}
	e := &s.bld().e
	if _, ok := s.waterbendPartFor(k, e); !ok {
		return false
	}
	// buildCost already subtracts the announced picks (gorge's
	// affordableTargetCandidates folds them through applyConvoke before the
	// affordability probe), so the probe prices the cost the payment will
	// charge. The picks are withheld from the window, as at payment.
	cost := s.buildCost(k)
	saved := s.fork()
	defer s.unfork()
	var hide slotMask
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield {
			hide = hide.with(s.objs[s.bld().picks[i]].slot)
		}
	}
	s.occ = s.occ.andNot(hide)
	// an incomplete census keeps the offer (gorge affordableTargetCandidates);
	// the census is of the sources the window still has, a committed pick being
	// no unit of it (castWindowCensusComplete skips convokeCommitted)
	if !s.windowCensusComplete(k, e) {
		s.CopyFrom(saved)
		return false
	}
	ok, _ := s.windowTapRider(k, e.ctrl, &cost, false, s.bld().bfl&bflAnyColor != 0)
	s.CopyFrom(saved)
	return !ok
}
