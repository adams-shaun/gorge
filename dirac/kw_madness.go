package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Madness (CR 702.35a-b; gorge rules/altcast.go, replacement.go,
// trigger_queue.go, stack.go): a discard of a madness card (every discard
// path reaches discardObj) puts it into exile instead, and queues a
// kernel-native keyword trigger (ab abMadness, source and object the
// card; gorge's "__kwMadnessCast", stack ability 255). As that trigger
// resolves, its owner is asked (gorge askMadnessCast, KTriggerOptional:
// ChOptional) to cast the card for its madness cost -- "yes" offered only
// when the card is castable -- or not: "no" puts it into the graveyard.
// The cast is begun once the trigger has left the stack, ignoring timing;
// it carries trigPlMadness (stackEntry.trigPl, which no spell otherwise
// uses beside trigPlSneak), which prices it at CardDef.MadnessCost and pays
// it through gorge's mana window (a cast begun mid-resolution has no
// payment witness).
const abMadness uint8 = 251

// trigPlMadness is in trigpl.go.

// debugMadnessMutate is the test-only mutation switch of
// TestPauperkwMadnessLockstepMutation: 1 lets the planner pay a madness
// cast at the mana step (gorge pays it through its mana window).
var debugMadnessMutate uint8

// SetDebugMadnessMutate sets the mutation switch; tests only.
func SetDebugMadnessMutate(m uint8) { debugMadnessMutate = m }

const raGenMadness = raGenManifestDread + 50 // the madness cast-or-graveyard ask (ChOptional)

// madnessExiled reports whether discarding hand card o exiles it (printed
// madness).
func (s *State) madnessExiled(k *prep, o Obj) bool {
	return k.gen != nil && k.gen.anyMadness && k.t.Cards[s.objs[o].card].HasMadness
}

// queueMadness queues the madness trigger of exiled card o.
//
//go:noinline
func (s *State) queueMadness(o Obj) {
	ob := &s.objs[o]
	p := ob.owner()
	s.enqueue(o, ob.ver, ob.card, abMadness, p, o, p, 0)
}

// resolveMadness poses the cast-or-graveyard ask, or finishes when the card
// is no longer in exile (gorge: the ability simply finishes).
//
//go:noinline
func (s *State) resolveMadness(k *prep) {
	e := &s.res.e
	if o := e.obj; s.objs[o].zone != kop.ZoneExile || s.objs[o].ver != e.srcVer {
		s.finishResolve(k, false)
		return
	}
	s.res.ask, s.res.askP = raGenMadness, e.ctrl
	s.ctl = ctlResolve
}

// madnessCastable: the owner can cast exiled card o for its madness cost
// (gorge askMadnessCast's gate): the cost payable and every target slot
// feasible. A card with Convoke or Improvise taps for part of the madness cost
// too (CR 702.51a, 702.126a: the keyword applies to the spell whichever cost
// it is cast for), so the tap-paid credit is taken first, as the hand offer
// does. The credit decides which read prices the offer, as gorge's gate does:
// with a live credit (a creature or artifact committed, or the card has
// delve) the planner's "insufficient" verdict is converted to "unsupported
// cost:credit" (potential_plan.go potentialModeCredit), which is no proof, so
// the offer stands whenever gorge's potential bound affords the credited cost;
// a creditless one keeps the proof, and the census and window price it.
func (s *State) madnessCastable(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	fh := s.objs[o].zone == kop.ZoneHand
	cost, used := s.convokeCreditUsed(k, p, c, o, fh, k.t.Cards[c].MadnessCost)
	credited := used.any() || s.hasCastDelve(k, p, c, o, fh)
	// the CR 601.2f modifiers compose over the credited cost (gorge
	// offerCastablePotential: the gate composes the very charge the payment
	// will make, engine/rules/mana.go), so a ReduceCost static may still
	// afford a credited residual the census refuses, and a Thalia-style
	// increase prices the offer the cast must pay
	cost = s.adjustCost(k, p, c, o, cost)
	winOK := s.affordsFor(k, p, &cost)
	painOK := s.painCastAffords(k, p, &cost)
	ok := winOK || s.ctrWindowAffords(k, p, &cost) || s.restrCastAffords(k, p, o, &cost) || s.filterCastAffords(k, p, o, &cost) || painOK
	// The window's pain read is an over-bound (painAvailMask's availWin arm:
	// a pain-only source's colour with no lethal guard, gorge's shared
	// windowManaUnitsWith census), so the window alone affording is no offer:
	// gorge's askMadnessCast gate still runs provenUnpayable, whose planner
	// census (painCastAffords' availFor, lethal guard included, CR 119.4) a
	// pain-only funding turns "insufficient", and whose proof legs below drop
	// the offer exactly as gorge's does
	if ok && winOK && !painOK && !credited && s.madnessPainDenied(k, p, o, &cost) {
		return false
	}
	var stall uint8
	if !ok {
		if credited {
			// the credited cast's gate is the potential bound alone (above)
			ok = s.potentialAffords(k, p, o, &cost)
		} else {
			// the failed window probe reports its fork's latched stall; this
			// drop is final for the madness cast, so it declares it rather
			// than losing the offer silently (tapXScript's shape)
			ok, stall = s.windowAffords(k, p, &cost)
			if !ok && (k.gen != nil && (k.gen.anyCondSplit || s.scriptTapsDefer(k, p).any() || k.gen.anyFilt && s.filtHybIn(k, s.scriptFilt(k, p))) || s.sacOtherManaLive(k, p)) {
				// gorge prices the madness cast against the potential pool
				// too, which counts a source the census defers (mana_condsplit.go's
				// River of Tears, and a Taps-deferred one: Burning Earth beside
				// the nonbasic lands whose tap fires its watcher -- its census
				// leaves the source unlisted and so cannot prove the cast
				// unpayable, mana_tapsdefer.go), and one that sacrifices another
				// permanent for its mana (Jungle Patrol's "Sacrifice a token
				// named Wood: Add {R}", potentialGapCost's CostSac arm; CR
				// 601.2g), and a hybrid-fee filter whose fee is no floating
				// mana (Sunken Ruins' "{U/B}, {T}: Add {U}{U}, {U}{B}, or
				// {B}{B}": a paid gap gorge's census cannot relax, so its
				// "insufficient" is never a proof, and a tap of another land
				// pays the fee, CR 605.3a); without any the census and window
				// say the same.
				// The over-bound alone is gorge's offerCastablePotential leg:
				// its gate still runs provenUnpayable (altcast.go askMadnessCast),
				// whose verdict here is the failed probe's "insufficient" and
				// whose census is incomplete -- the arm's premise -- so the
				// proof is the relaxed census's (paymentPlanRelaxProof: each
				// once-only relaxed alternative is one tap, its largest
				// production), the count's (potential_nogain.go
				// costOutrunsSupply, beside a census whose gaps net no mana),
				// or nothing, when a gap gains mana and gorge prices the offer
				// on the bound alone (madnessDeferProofClass)
				ok = s.potentialAffords(k, p, o, &cost)
				if ok {
					complete, relaxable, gains := s.madnessDeferProofClass(k, p, o)
					if complete ||
						relaxable && s.madnessRelaxProven(k, p, o, &cost) ||
						!gains && s.madnessCountProven(k, p, o, &cost) {
						ok = false // proven unpayable: the offer drops with gorge's
					}
				}
			}
		}
	}
	if !ok {
		if stall != 0 {
			s.capFull(stall)
		}
		return false
	}
	return s.spellTargetsFeasible(k, c, p)
}

// madnessPainDenied: the unless-window census affordsFor reads lists a
// pain-only source (Centaur Garden's "{T}: Add {G}. It deals 1 damage to you")
// with no lethal guard, but gorge's askMadnessCast gate drops the offer when
// its planner finds the cost insufficient and the census is complete
// (provenUnpayable): the planner never takes a payment whose damage would
// kill its payer (CR 119.4; payment_plan.go's lethal guard). The window read
// alone afforded the cost, the planner's census (painCastAffords, lethal
// guard included) does not, and no gap stands in the proof's way.
func (s *State) madnessPainDenied(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if !k.painMana {
		return false
	}
	// gorge provenUnpayable's legs, over the same census gaps the arms above
	// read (madnessDeferProofClass): a planner-deferred, restricted, paid
	// filter, counter-cost or sacrificed-other source is a gap the planner's
	// census leaves unpriced, so its "insufficient" is no proof (the offer
	// stays, priced on the potential bound); a complete census's own
	// "insufficient" is, and the relaxed-alternative and count proofs cover
	// the incomplete ones whose gaps gain nothing
	complete, relaxable, gains := s.madnessDeferProofClass(k, p, o)
	return complete ||
		relaxable && s.madnessRelaxProven(k, p, o, cost) ||
		!gains && s.madnessCountProven(k, p, o, cost)
}

// madnessDeferProofClass classes p's census gaps for the creditless madness
// cast's unpayability proofs (gorge provenUnpayable's legs, altcast.go
// askMadnessCast, over paymentPlanCensusOf's fields). complete prices every
// mana ability of p's in the census the arms above read (no gap: the census's
// own "insufficient" is the proof). relaxable is false when a gap cannot be
// relaxed to a once-only alternative -- a rider's or repeatable activation,
// an unpriced restricted or each-colour production, a store battery's, a
// deplete or tapX cost, a paid filter or counter-cost one (gorge
// paymentPlanRelaxedAlternatives: "a repeatable tap source is deliberately
// not relaxable"). gains mirrors paymentPlanCensus.gapGains: a gap that
// could still raise the mana the payment has to spend -- a sacrificed
// permanent (a census gap gfGapOther), a tap trigger's extra beside the tap
// (CR 605.4a), any of the repeatable shapes -- voids the count proof. The
// census's unpriced sources are the walk windowCensusComplete reads
// (act_waterbend.go), so the walk here is its, reading the same gaps into
// three booleans.
func (s *State) madnessDeferProofClass(k *prep, p uint8, o Obj) (complete, relaxable, gains bool) {
	complete, relaxable = true, true
	if k.gen == nil {
		return true, true, false
	}
	d := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(p, s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	for _, sl := range ord[:cnt] {
		pm := &s.perms[sl]
		lost := s.genLost(k, sl)
		// a tapped source's {T} can contribute nothing to this payment (CR
		// 602.2b), so it is no census gap; a lost printed source is skipped
		// when no grant survives it
		if pm.flags&pfTapped != 0 || s.activationSick(k, sl) ||
			lost && !s.manaGranted(k, sl) {
			continue
		}
		// a planner-deferred source (a Taps watcher, a Condition-split one, a
		// rider) is no unit of the census (CR 605.1b); its once-only relaxed
		// alternatives -- one tap, the census fold's largest production -- are
		// what madnessRelaxProven bounds. Its tap can gain mana, so it is a
		// gaining gap too (gorge's paymentPlanCensus.gapGains: neither
		// manaFeeNetsNoMana nor manaTapAbilityNetsNoMore holds a bare {T} one)
		// -- the count proof is void beside it, the relaxed one is not
		if s.windowCanTap(k, sl) && s.planDeferred(k, p, sl) {
			complete, gains = false, true
			if _, ok := s.riderAbilityOf(k, sl); ok {
				// a repeatable or sacrifice-self rider is no once-only
				// relaxation either
				relaxable = false
				continue
			}
			continue
		}
		if !lost {
			cd := &k.t.Cards[s.cardAt(sl)]
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				// an unpriced restricted ability the payment admits is a census
				// gap its restriction bounds (CR 106.6), no once-only unit
				if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 &&
					(a.ManaRestrict != 0 || a.ManaRestrictAb != 0) && s.restrSrcAdmits(k, p, sl, a, &d) &&
					!s.windowPricesRestr(k, sl, a) {
					complete, relaxable, gains = false, false, true
				}
				// an unpriced each-colour production (Bloom Tender's) is one too
				if k.gen.anyEachCol && k.gen.has(s.cardAt(sl), gfEachCol) && int(s.manaPCAt(k, sl).manaAb) < 0 &&
					!(k.gen.anyCantAct && s.actForbidden(k, sl, true)) {
					complete, relaxable, gains = false, false, true
				}
			}
		}
		if s.depleteCensusGap(k, p, sl) {
			complete, relaxable, gains = false, false, true // its SubCounter cost is unpriced
		}
		// a tap trigger that adds mana beside a source's tap makes a production
		// no census unit prices (CR 605.4a)
		if k.gen.anyTapExtra {
			pc := s.windowGrantPC(k, sl)
			// an all-at-once source's reflected pick (srcEachPick) is no unit
			// of this proof either: gorge defers the source
			if _, hit := s.srcExtraHit(k, sl, pc); hit || s.srcEachPick(k, sl, pc) > 0 {
				complete, relaxable, gains = false, false, true
			}
		}
	}
	if s.storeCensusGap(k, p) {
		complete, relaxable, gains = false, false, true // a storage battery's SubCounter<X> cost is unpriced
	}
	if s.tapXCensusGap(k, p).and(s.occ).any() {
		complete, relaxable, gains = false, false, true // its tapXType cost is unpriced
	}
	if s.scriptCtr(k, p).any() || s.scriptFilt(k, p).any() {
		complete, relaxable, gains = false, false, true // a counter-cost or paid-filter activation is repeatable
	}
	if s.sacOtherManaLive(k, p) {
		complete, relaxable, gains = false, false, true // the sacrificed permanent is no census source
	}
	return complete, relaxable, gains
}

// madnessRelaxProven: pool plus the capped census -- every source once, at its
// largest production, the planner-deferred ones included (gorge's
// paymentPlanRelaxedAlternatives relax a gap to one tap of the census fold's
// largest production) -- cannot fund the composed cost. The bound prices every
// relaxed run, so when it comes short, the verdict's "insufficient" is the
// proof (gorge paymentPlanRelaxProof, CR 601.2g-h). The restricted sources the
// spell may spend count through their admitted view (CR 106.6), as the count
// proof's admits read does.
func (s *State) madnessRelaxProven(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	var av avail
	s.availForCapped(k, p, &av)
	if k.restrSrc {
		d := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
		if t, _, nm := s.restrSrcAvailD(k, p, d, &av, NoSlot, true); nm == 0 {
			av = t
		}
	}
	return s.manaPoolTotal(k, p)+av.total < cost.MV()
}

// madnessCountProven mirrors gorge's costOutrunsSupply (potential_nogain.go):
// the pool and every priced census source (each tapped once, at its largest
// production) cannot fund the composed cost's mana count. A cost whose mana
// count can fall after announcement ({X}, a Phyrexian pip's life face) is
// never proven (gorge's c.X, XMin and Phyrexian arms).
func (s *State) madnessCountProven(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if cost.X != 0 || cost.XRed != 0 || cost.Phyrexian() {
		return false
	}
	var av avail
	s.availFor(k, p, &av)
	if k.restrSrc {
		d := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
		if t, _, nm := s.restrSrcAvailD(k, p, d, &av, NoSlot, false); nm == 0 {
			av = t
		}
	}
	return s.manaPoolTotal(k, p)+av.total < cost.MV()
}

// manaPoolTotal is p's floating pool's mana count.
func (s *State) manaPoolTotal(k *prep, p uint8) int {
	n := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		n += int(s.pl[p].pool[c])
	}
	return n
}

// sacOtherManaLive reports whether p controls a live mana ability that
// sacrifices another permanent for its mana (Jungle Patrol's "Sacrifice a token
// named Wood: Add {R}"): a census gap (gfGapOther) the unless window does not
// model but the potential walk prices (potentialGapCost's CostSac arm, CR
// 605.1a). The ability's cost need not tap, so a tapped source still counts;
// only a {T} cost reads the tap and summoning sickness.
func (s *State) sacOtherManaLive(k *prep, p uint8) bool {
	if k.gen == nil {
		return false
	}
	for b := s.ctrlM[p]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if !k.gen.has(c, gfGapOther) || s.genLost(k, sl) || s.pickedForMana(s.perms[sl].obj) {
			continue
		}
		j := k.gen.gapOther(c)
		if j < 0 || int(j) >= len(k.t.Cards[c].Abilities) {
			continue
		}
		a := &k.t.Cards[c].Abilities[j]
		if a.Kind != kop.AbMana || a.Cost.Flags&kop.CostSac == 0 || a.Cost.Flags&kop.CostSacSelf != 0 || a.Cost.N == 0 {
			continue
		}
		if a.Cost.Flags&kop.CostTap != 0 && (s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl)) {
			continue
		}
		if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if s.countCostSac(k, p, &a.Cost, sl) >= int(a.Cost.N) {
			return true
		}
	}
	return false
}

// painCastAffords: a painland's painful colour funds the madness cost (CR
// 601.2g: Llanowar Wastes' {B}, its damage not lethal). gorge's
// offerCastablePotential prices the cast against the potential pool and its
// planner, which both take that colour; the unless-window census affordsFor
// reads (availWindowFor, painAvailMask) lists a painland by its {C} alone.
// The offer walk's census (availFor) is the planner's, lethal guard included.
func (s *State) painCastAffords(k *prep, p uint8, cost *kop.ManaCost) bool {
	if !k.painMana {
		return false
	}
	var av avail
	s.availFor(k, p, &av)
	return av.affords(cost, pipMaskOf(cost), 0)
}

// restrCastAffords: the restricted sources the madness cast of card o may
// spend (CR 106.6: an Ancient Ziggurat's mana pays a creature spell) fund cost.
// gorge's offerCastablePotential prices the cast with this spell's payment
// descriptor, so such a source counts; the plain census does not take it.
func (s *State) restrCastAffords(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if !k.restrSrc {
		return false
	}
	var av avail
	s.availWindowFor(k, p, &av)
	spell := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
	return s.restrSrcAffords(k, p, spell, cost, &av, NoSlot, 0)
}

// filterCastAffords: the paid filter sources (Simic Signet's "{1}, {T}: Add
// {G}{U}") fund the madness cost beside the plain sources, each fee charged
// from the rest (CR 605.3a, 601.2g-h). gorge's offerCastablePotential prices a
// madness cast through its potential planner, which weighs a filter that makes
// more mana than its fee takes; the plain census leaves every filter out. The
// gaps census (offerSourcesAfford) is the hand cast's read of the same
// sources, a net-negative filter counting for less, never for more.
func (s *State) filterCastAffords(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.gen.anyFilt || !s.filtFundsMadness(k, p) {
		return false
	}
	// The gaps census lets a filter's own tap stand as its fee's mana (Unknown
	// Shores alone reads {1}{T}: any colour beside its free {C}); one tap makes
	// one of them (CR 605.1a), and gorge's askMadnessCast gate always runs the
	// potential bound first, which leaves the source's own mana out of its fee.
	return s.offerSourcesAfford(k, p, s.objs[o].card, o, cost) && s.potentialAffords(k, p, o, cost)
}

// filtFundsMadness: some filter of p's may fund the madness cast. A
// hybrid-fee filter (Graven Cairns' "{B/R}, {T}: Add {B}{B}, {B}{R}, or
// {R}{R}") funds nothing unless p already floats its fee or a printed source
// of p's can make one of its two faces: gorge's census cannot relax it
// (mana_filterhybrid.go) and its potential bound refuses it without such a
// source, while the gaps census charges its {B/R} as generic mana, so a Plains
// and a green land would look like they fund {1}{R}{R} (CR 107.4c, 601.2g-h),
// where a Swamp beside the Cairns does (the Swamp's {B} is the fee).
func (s *State) filtFundsMadness(k *prep, p uint8) bool {
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		a, c, hyb := s.filtHyb(k, b.first())
		if !hyb || s.filtPaid(k, p, b.first()) || s.printedMakesAny(k, p, uint8(1)<<uint(a)|uint8(1)<<uint(c)) {
			return true
		}
	}
	return false
}

// printedMakesAny: some printed source of p's (the planner census, a filter
// or gap not among them) can make a colour of mask m.
func (s *State) printedMakesAny(k *prep, p uint8, m uint8) bool {
	var av avail
	s.availFor(k, p, &av)
	return av.canMakeAny(m)
}

// keywordCastAffords: p can pay cost for card o's keyword cast its resolving
// trigger offers (madness, miracle). gorge's offerCastablePotential prices the
// cost against the payment's potential pool (potentialManaFor), an over-bound
// in which a source of indeterminate amount (Nardole, Resourceful Cyborg's {U}
// per counter, at no counters) is open in every colour, so a cost the census
// cannot fund may still be offered; the census and the window go first.
func (s *State) keywordCastAffords(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	ok := s.affordsFor(k, p, cost)
	var stall uint8
	if !ok {
		// the failed window probe reports its fork's latched stall; the
		// potential bound below is the sibling that may still price the cast
		ok, stall = s.windowAffords(k, p, cost)
	}
	if ok {
		return true
	}
	if s.potentialAffords(k, p, o, cost) {
		return true
	}
	if stall != 0 {
		// every route failed and the probe's fork latched a rule gap the kernel
		// stalls by name: declare it, not a dropped keyword cast (tapXScript's
		// shape)
		s.capFull(stall)
	}
	return false
}

// potentialAffords: cost fits the payment's potential pool for card o's cast
// (gorge offerCastablePotential's over-bound).
func (s *State) potentialAffords(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	spell := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
	var pa avail
	s.availPotentialFor(k, p, &pa, &spell, false)
	return pa.affords(cost, pipMaskOf(cost), 0)
}

// ctrWindowAffords: gorge's potential search for the madness cast activates a
// counter-cost mana source (Vivid Creek's any-colour ability: {T}, remove a
// charge counter) that neither the planner's sources nor windowAffords' gap
// kinds count: its census proves the cost over the gaps walk's units
// (availGaps counts each live counter-cost source's mana, mana_countercost.go).
func (s *State) ctrWindowAffords(k *prep, p uint8, cost *kop.ManaCost) bool {
	if !k.ctrMana || !s.scriptCtr(k, p).any() {
		return false
	}
	var av avail
	x := s.g.availX
	s.g.availX |= availGaps
	s.availFor(k, p, &av)
	s.g.availX = x
	return av.affords(cost, pipMaskOf(cost), 0)
}

// madnessWindowLive: the cast being built is a madness cast paid through the
// plain mana window (madnessPlainPay).
func (s *State) madnessWindowLive() bool {
	return s.buildLive() && s.bld().e.kind == seSpell && s.bld().e.trigPl == trigPlMadness
}

// madnessReversed: a madness card whose cast was reversed was not cast (CR
// 702.35b), so it goes to its owner's graveyard from the exile the reversal
// returned it to (gorge cast_commit.go abortCast: madnessDeclined).
func (s *State) madnessReversed(o Obj) {
	if s.objs[o].zone == kop.ZoneExile {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneGraveyard, false, from, false)
	}
}

// madnessPlainPay: e is a madness cast paid through the plain mana window; one
// whose cost is part tap-paid (madnessConvokes) goes through payConvoke instead.
func (s *State) madnessPlainPay(k *prep, e *stackEntry) bool {
	return e.trigPl == trigPlMadness && !s.madnessConvokes(k, e)
}

// madnessConvokes: the madness cast of e pays part of its cost by tapping
// creatures or artifacts (Convoke, Improvise; CR 702.51a), so its payment
// goes through payConvoke, which applies the announced picks, not the plain
// window.
func (s *State) madnessConvokes(k *prep, e *stackEntry) bool {
	return k.gen != nil && k.gen.anyConvoke && s.tapPaysEntry(k, e)
}
