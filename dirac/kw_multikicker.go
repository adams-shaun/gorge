package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Multikicker (CR 702.43, Forge's K:Multikicker:<cost>): "you may pay [cost]
// any number of times as you cast this spell". gorge offers the multikicked
// cast as its own priority option paying the base plus ONE payment
// (legal_walk_hand.go) and then asks the count 0..max before Convoke/X and the
// targets (cast_asks.go multikickAsk); the answered count folds that many
// payments into the total cost and rides the pay-time CastInfo's FlagMultikicked
// Amount into Object.TimesKicked, which Count$TimesKicked reads on the spell
// and on the permanent it becomes (events/apply_cast.go, effects/count_body_paid.go).
//
// The kernel mirrors that shape:
//   - the card's kicked cast (AkCast mode bit 0, sfKicked) is offered when the
//     base plus one payment is affordable (legal.go), and its build cost adds
//     the chosen count's payments (cast.go buildCost);
//   - the count ask is a DkChoose/ChMultikick stage after the tap costs and
//     before Convoke/X (buildProgram's bsMulti), bounded by what the payment
//     window can still make (multikickMax);
//   - a zero answer clears sfKicked: an exactly plain cast, as gorge's
//     modeFlags maps a count-0 multikicked mode to "";
//   - the count lives in cold.mkObj[obj] while the spell is on the stack and
//     moves to cold.mk[slot] as the permanent enters, where
//     Count$TimesKicked (kop.ExKicked N 1) reads it (exprs.go timesKicked).

// maxMultikick bounds the count ask, gorge multikickAsk's own 64-iteration
// cap. A real board's mana runs out first.
const maxMultikick = 64

// genPrep.anyMultikick bits: mkAny marks a table with any repeated additional
// cost (the per-object cold.mkObj count); mkPerm marks a pure Multikicker
// whose count a battlefield reader needs (the per-slot cold.mk).
const (
	mkAny = 1 << iota
	mkPerm
)

// noteRepeatedCost marks a table's repeated-cost cold needs for one card:
// mkAny for any Multikicker/Squad/Replicate count on the stack, and mkPerm
// only for a pure Multikicker, whose permanent reads cold.mk. A Squad
// self-ETB reads the count captured at enqueue, and a Replicate count is
// only read on the stack, so their pools need no per-slot region.
func (g *genPrep) noteRepeatedCost(c *kop.CardDef) {
	if !c.HasMultikicker {
		return
	}
	g.anyMultikick |= mkAny
	if c.RepeatedKind != kop.RepeatSquad && c.RepeatedKind != kop.RepeatReplicate {
		g.anyMultikick |= mkPerm
	}
}

// coldMk is the counts per battlefield slot for a table with a Multikicker
// card that reads them from the permanent; coldMkObj is per object while the
// spell is on the stack, for any repeated additional cost (Multikicker,
// Squad or Replicate). A Squad self-ETB reads the count captured at enqueue
// (pendingTrig.defSeat), so a Squad/Replicate pool allocates only mkObj;
// keeping the per-slot region out of a large tail pool keeps State + cold
// inside the 44 KiB bar (TestEveryPoolFitsTheStateBar).
var (
	coldMk = newCold[uint8]("mk", func(g *genPrep) int {
		if g.anyMultikick&mkPerm != 0 {
			return MaxPerm
		}
		return 0
	}).view(func(c *coldArrays) *[]uint8 { return &c.mk }).perSlot()
	coldMkObj = newCold[uint8]("mkObj", func(g *genPrep) int {
		if g.anyMultikick&mkAny != 0 {
			return MaxObj
		}
		return 0
	}).view(func(c *coldArrays) *[]uint8 { return &c.mkObj })
)

// multikickCount is the multikicker payment count of the spell e (0 for a
// permanent or a table without Multikicker).
func (s *State) multikickCount(e *stackEntry) int {
	if e == nil || int(e.obj) >= len(s.cold.mkObj) {
		return 0
	}
	return int(s.cold.mkObj[e.obj])
}

// setMultikick records e's payment count.
func (s *State) setMultikick(e *stackEntry, n int) {
	if int(e.obj) < len(s.cold.mkObj) {
		s.cold.mkObj[e.obj] = uint8(n)
	}
}

// kickerPayment is the per-payment cost a kicked cast adds: Multikicker for a
// Multikicker card, else the plain Kicker (an Offspring or OptWaterbend card
// keeps its cost in Kicker too).
func kickerPayment(cd *kop.CardDef) kop.ManaCost {
	if cd.HasMultikicker {
		return cd.Multikicker
	}
	return cd.Kicker
}

// addKickerPayments is c plus a kicked cast's additional cost: the chosen
// count of Multikicker payments (CR 702.43), else the plain Kicker once. The
// one home of that pricing for the cast build, the may-play cost and the
// target affordability walk.
func (s *State) addKickerPayments(cd *kop.CardDef, e *stackEntry, c kop.ManaCost) kop.ManaCost {
	if !cd.HasMultikicker {
		return addCost(c, cd.Kicker)
	}
	for i := 0; i < s.multikickCount(e); i++ {
		c = addCost(c, cd.Multikicker)
	}
	return c
}

// kickedAffords is the hand-cast kicked offer's mana gate for the total kc.
// A Kicker or Multikicker spell that pays with taps or the graveyard (a
// printed or granted Convoke or Improvise, or a Delve) is offered by gorge's
// pool-only legacy gate alone: its potential walk declines every
// credit-bearing kicked mode (potentialModeCastPlan's cost:credit: "kicked"
// and "multikicked" are no altCastModes row), so lands the cast could still
// tap do not make the kicked option (CR 702.33, 702.43). The pool gate folds
// the delve credit the way offerCastableUsing does (FeatPoolPays mode 1 ->
// delveOfferCredit); without that credit the sources count.
func (s *State) kickedAffords(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost, cav *avail) bool {
	if k.gen != nil && s.contribPays(k, p, c, o, true) {
		return s.FeatPoolPays(p, o, 1)
	}
	return s.castAffordsR(k, p, c, s.restrOriginOfObj(o), kc, cav)
}

// offerKickedHand is the hand walk's kicked offer: the base plus one payment
// (Multikicker's, else the plain Kicker) with the CR 601.2f modifiers and the
// any-type conversion composed in, offered when the real pool affords it --
// or, under gorge's autopay surface, when gorge's potential planner prices it
// against the potential pool (kickedPotentialPays).
func (s *State) offerKickedHand(k *prep, d *Decision, p uint8, c CardID, o Obj, base kop.ManaCost, cav *avail) {
	kc := addCost(base, kickerPayment(&k.t.Cards[c]))
	if s.statM.any() || k.gen != nil {
		kc = s.adjustCost(k, p, c, o, kc) // the reduction applies to the total (CR 601.2f)
	}
	if k.gen != nil && k.gen.anyTypeCreat && s.convAnyType(k, p, o) {
		kc = anyType(kc) // the kicked total under Vizier's conversion too (mana_convertcreature.go)
	}
	aff := s.kickedAffordsOrPoolPays(k, p, c, o, &kc, cav)
	cd := &k.t.Cards[c]
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX == 0 && s.spellTgtDep(k, p, c, o) {
		// A target-priced cast (Esior, Wardwing Familiar's raise on a spell
		// that may target it): gorge's witness prices the composed total
		// from the planner's census and the census/prefix fallback declines
		// a target-priced cast's prefix search (potentialTargetPriced), so a
		// census gap beside the raise (Song of Freyalise's creature mana
		// funding the kicker's {B}, the dr-mbc-bbab Tear Asunder repro at
		// decision 621) leaves the play census_incomplete and the autopay
		// surface drops it (so does the lowering surface: Imps' Taunt's buyback
		// beside Esior and a granted sacrifice mana ability). The plain census read (kickedAffords) does not
		// hold here: the search that prices a play the covered sources
		// cannot pay is the one gorge never runs (CR 601.2f).
		aff = s.ungrantedPaysNoDH(k, p, c, o, &kc)
	} else if !aff && s.cfg&cfgGorgeAutopay != 0 && s.g.availX == 0 {
		// the real pool's refusal is not the whole gate under gorge's
		// autopay surface: gorge keeps the kicked cast as one of its
		// potential plays (gorgeOffersKicked) and its potential planner
		// prices it against the potential pool (pair 98: Caligo
		// Skin-Witch paid through Rain of Filth's grants)
		spell := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
		aff = s.kickedPotentialPays(k, p, c, o, &kc, &spell)
	}
	// A Squad/Replicate offer is gorge's replicate/squad option (mode word,
	// not the kicked mode the multikicker/offspring offer rides), so it needs
	// no gorgeOffersKicked cache read: the real affordability gate above is
	// the same one legal_walk_hand.go's offer uses.
	gorgeOK := cd.RepeatedKind == kop.RepeatSquad || cd.RepeatedKind == kop.RepeatReplicate || s.gorgeOffersKicked(p, o)
	if aff && s.kickedChainFeasible(k, c, p) && gorgeOK && (!k.sacMana || s.kickWitness(k, p, c, o, &kc)) && (k.gen == nil || !k.gen.anyKickCost || s.kickerCostOffer(k, p, c, o)) &&
		!s.dhWithholdsCast(k, p, o, &kc) { // a hand cast the Diamonds alone fund: gorge's planner prices no plan (census_incomplete, mana_discardhand.go)
		d.add(MakeAction(AkCast, TCode(o), 0, 1))
	}
}

// offerKickedPotentialOnly is the hand walk's kicked offer for a card whose
// plain cast the real census cannot pay: under gorge's autopay surface the
// planner still prices the kicked cast against the potential walk's bound
// (potentialModeCastPlan), so a census gap (Sandstone Needle's {R}{R}, beside
// Gemhide Sliver's any-colour grant) that the plain cast needs too is no
// reason to drop the kicked offer (offerKickedHand's potential arm decides).
// It runs the plain cast's own feasibility gates first (targets, additional
// costs), which the hand walk's `continue` on the affordability gate skipped.
func (s *State) offerKickedPotentialOnly(k *prep, d *Decision, p uint8, c CardID, o Obj, base kop.ManaCost, preDelve, plainBase *kop.ManaCost, cav, av *avail) {
	if s.cfg&cfgGorgeAutopay == 0 || s.g.availX != 0 || s.castHeldOut(o) { // gorge holds the card's whole cast out after a second undo (CR 733.2)
		return
	}
	if !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastableFrom(k, c, p, o, preDelve, plainBase, false, false, av) {
		return
	}
	s.offerKickedHand(k, d, p, c, o, base, cav)
}

// kickedArmOffer reports whether action a is a kicked cast (the mode-1 AkCast
// offerKickedHand makes) on a kicker or multikicker card: grantCap's
// walk-not-listed pruning does not drop it, because the walk's plainOptions
// prices no kicked cast there (the arm runs only at availX == 0, against the
// real census, while the walk's census is the relaxed one) and gorge's
// planner prices the mode cast itself (potentialModeCastPlan).
func (s *State) kickedArmOffer(k *prep, a Action) bool {
	if a.Kind() != AkCast || a.Mode()&3 != 1 {
		return false
	}
	c := s.objs[Obj(a.Source())].card
	return k.cards[c].kw&kop.KwKicker != 0 || k.t.Cards[c].HasMultikicker
}

// kickedPotentialPays is the kicked offer's potential-pool arm under gorge's
// autopay surface: gorge keeps the kicked cast as one of its potential plays
// (gorgeOffersKicked) and its potential planner prices it against the
// potential walk's bound (rules potentialModeCastPlan, the same pool gate
// fbInstances prices the flashback instances with), so a kicked cast the
// floats and the untapped production pay is offered even when the real pool
// cannot (pair 98: Caligo Skin-Witch's {4}{B}{B} through Rain of Filth's
// sacrifice grants). An open production makes that fold unbounded in every
// colour, so there the planner's census is the bound: the walk's units' per
// tap, each open source's natural production once (availPotentialFor's
// units, the relaxed census paymentPlanRelaxProof bounds the gaps the same
// way — pair 102: Gemhide Sliver made Keldon Strike Team's kicked cast look
// payable that the census cannot pay, pair 98's arm beside them). The
// planner's declines are mirrored (gorgeLowersRider's list) so only plans
// gorge itself produces are mirrored.

func (s *State) kickedPotentialPays(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost, d *payDesc) bool {
	if s.contribPays(k, p, c, o, true) {
		return false // the potential planner refuses a credit-bearing cast, and "kicked" is no altCastModes row (cost:credit)
	}
	if s.waterbendOwnPart(k, p, c, o) != 0 {
		return false // ... and a waterbend one (potentialModeCastPlan, "shape:contribution")
	}
	if kc.X != 0 {
		return false // the planner prices the largest X the sources fund: unmodelled here
	}
	if sacXCost(spellAddCost(k, c)) {
		return false // an announced Sac<X> the composition declines too, so only the floating pool offers it (cost_sacx.go)
	}
	if ac := spellAddCost(k, c); ac.Flags&kop.CostTapX != 0 || ac.Flags&kop.CostSac != 0 && k.sacMana {
		// a permanent tapped for the part is no mana source, and a source that
		// sacrifices itself can use up what the part must sacrifice (CR 601.2g-h;
		// gorge planReserving)
		if !s.spellPartDisjoint(k, p, c, o, ac, kc) {
			return false
		}
	}
	if k.t.Cards[c].HasAltAdd {
		return false // the either-or part's composition is the plain one (potentialAltAddPlan); unmodelled for a kicked total
	}
	// gorge's potential walk (PotentialMana) lists the play: the floats and
	// what the untapped sources could produce (availPotentialFor's fixpoint,
	// the granted sacrifice-only abilities and each open source's natural
	// production once included, units). The over-bound fold prices a
	// fixed-choice production once per colour, while the planner's real
	// plans make one mana per tap (CR 118.3c), so the offer needs the
	// per-tap view too: {2}{G}{B} is no play when the board makes 3 mana, a
	// "Combo U B" dual beside two Forests among them (the recorded Yahenni
	// repro's Bog Badger at decision 104). With an open production the fold
	// is unbounded in every colour and no proof: there the census (the
	// units' per-tap view, gorge's relaxed census bounds the gaps the same
	// way) decides, and an incomplete record has no bounded view and
	// declines.
	var pa, px avail
	var pu [maxPotUnits]potUnit
	open := s.availPotentialFor(k, p, &pa, d, false, &pu)
	if !pa.affords(kc, pipMaskOf(kc), 0) {
		return false
	}
	if !potUnitsAvail(&px, &pu, int(s.pl[SeatIx(p)].life)) {
		if open {
			return false // no bounded census beside an unbounded fold
		}
		return true // the record is incomplete: keep the over-bound answer
	}
	return px.affords(kc, pipMaskOf(kc), 0)
}

// multikickMax is the largest multikicker count gorge multikickAsk would
// offer: base + N payments walked through the same affordability check the
// payment uses, bounded by the floating pool for an {X}-less spell (the
// oracle lowering has already tapped base + one payment into it) and by the
// window for an {X} spell. The build cost is recomputed at each count so a
// reduction applies to the total exactly as at payment.
func (s *State) multikickMax(k *prep) int {
	e := &s.bld().e
	cd := &k.t.Cards[e.card]
	if e.flags&sfKicked == 0 || !cd.HasMultikicker {
		return 0
	}
	var av avail
	fromHand := s.castFromHandBy(e, e.ctrl)
	fromSources := cd.Cost.X != 0
	if fromSources {
		// An {X} multikicked cast's oracle witness taps for the X too, so its
		// pool already holds what the sources can make; the count ask reads
		// the same window the X ask does.
		var fav avail
		s.availXWindow(k, e.ctrl, &av)
		av = *s.castGrantAvail(k, e.ctrl, e.obj, &av, &fav)
	} else {
		// gorge multikickAsk bounds a non-X count by the FLOATING POOL at ask
		// time (its "pool-only at ask time"): the oracle's lowering witness
		// has tapped base + one payment into it. Untapped sources do not
		// count, exactly as an {X}-less cast's pool-only bound. Gorge's pool
		// holds its restricted units (RestrictedMana is provenance beside
		// them), so the units the cast of this card may spend count here
		// (restrSpendable, CR 106.6) — Karn, Legacy Reforged's artifact-only
		// {C} pays a multikicker {2} beside an artifact spell. Dirac's pool
		// holds them beside it in rb; this is availForBuilt's tail on the
		// pool-only walk.
		pl := &s.pl[e.ctrl]
		for c := 0; c < kop.NumManaKinds; c++ {
			if n := pl.pool[c]; n > 0 {
				av.addUnits(1<<uint(c), n)
			}
		}
		if s.rbAny(e.ctrl) {
			s.restrAvail(k, e.ctrl, e.card, s.payDescNow(NoObj).og, &av)
		}
		// a twobrid multikicker cost takes the delve credit per face (the
		// affordsTb leaf, gorge FeasibleAny's delve argument); the plain
		// generic credit is gateDelveCredit's below.
		av.delve = s.tbDelveCredit(k, e.ctrl, e.card, e.obj, fromHand)
	}
	save := s.multikickCount(e)
	max := 0
	for n := 1; n <= maxMultikick; n++ {
		s.setMultikick(e, n)
		cost := s.buildCost(k)
		// gorge countComposedCost folds the Convoke/Improvise reduction the
		// caster can still announce (convokeCountCredit) into the candidate,
		// then countCandPayable takes the cast's delve credit before the
		// mana gate, so a granted Delve (Teval) raises the payable count;
		// the delve picks are announced after this ask, so the bound reads
		// the whole graveyard, as gorge's does. K'rrik's PayLifeInsteadOf:B
		// grant is no dirac static (kgen refuses it), so no life credit is
		// folded here.
		cost = s.convokeCreditFrom(k, e.ctrl, e.card, e.obj, fromHand, cost)
		cost = s.gateDelveCredit(k, e.ctrl, e.card, e.obj, fromHand, cost)
		if !av.affords(&cost, pipMaskOf(&cost), 0) {
			break
		}
		// A Squad/Replicate payment's non-mana part is paid once per payment
		// (gorge countCandPayable's composed cost): the count stops where the
		// N-th pick is no longer payable (kw_squadreplicate.go).
		if rc := repeatPart(cd); rc != nil {
			scaled, ok := repeatScale(*rc, n)
			if !ok || !s.pickCostPayable(k, e.ctrl, &scaled, e.obj) {
				break
			}
		}
		max = n
	}
	s.setMultikick(e, save)
	return max
}

// multikickOptions poses the count ask, 0..max. false: not a repeated-cost
// kicked cast, the card's keyword belongs to another stage, or nothing is
// payable (the explicitly chosen mode degrades to the plain cast, gorge
// multikickAsk's max==0 arm). stage is the ask's build stage: Replicate
// before Multikicker, Squad after it (kw_squadreplicate.go).
func (s *State) multikickOptions(k *prep, d *Decision, stage uint8) bool {
	e := &s.bld().e
	cd := &k.t.Cards[e.card]
	if e.kind != seSpell || !cd.HasMultikicker || e.flags&sfKicked == 0 {
		return false
	}
	kind := cd.RepeatedKind
	if kind == 0 {
		kind = kop.RepeatMultikicker // a hand-built CardDef (tests) with only HasMultikicker
	}
	if repeatAskStage(kind) != stage {
		return false
	}
	max := s.multikickMax(k)
	if max == 0 {
		e.flags &^= sfKicked
		return false
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChMultikick, e.ctrl
	for n := 0; n <= max; n++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
	}
	return true
}

// answerMultikick settles the chosen count. Zero declines the optional cost
// and leaves an exactly plain cast; a positive count rides the spell object.
func (s *State) answerMultikick(k *prep, a Action) {
	e := &s.bld().e
	n := int(a.Mode())
	if DebugSquadCountOffByOne && n > 0 && k.t.Cards[e.card].RepeatedKind == kop.RepeatSquad {
		n-- // the mutation test: the stored count is one payment short
	}
	if n == 0 {
		e.flags &^= sfKicked
		s.setMultikick(e, 0)
	} else {
		s.setMultikick(e, n)
	}
	s.nextStage(k)
}
