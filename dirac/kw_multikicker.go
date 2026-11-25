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

// multikickWitnessCost is the witness's cost for a Multikicker cast whose
// count is not asked yet: the base plus one payment, then the cost statics
// (CR 601.2f: a reduction takes its generic off the total with the additional
// cost in it, as offerKickedHand prices the offer). plain is the cast's cost
// without the payment, already reduced: it stands when no static changes the
// total.
func (s *State) multikickWitnessCost(k *prep, plain kop.ManaCost) kop.ManaCost {
	e := &s.bld().e
	cd := &k.t.Cards[e.card]
	if !(s.statM.any() || k.gen != nil) {
		return addCost(plain, cd.Multikicker)
	}
	fl, fixed := e.flags, s.bld().fixedGen
	n := s.multikickCount(e)
	e.flags |= sfKicked
	s.setMultikick(e, 1)
	cost := s.buildCost(k)
	e.flags = fl
	s.setMultikick(e, n)
	s.bld().fixedGen = fixed
	return cost
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
	if aff && s.cfg&cfgGorgeAutopay != 0 && k.gen != nil && k.gen.anyTapsMana && kc.X == 0 {
		// gorge prices the kicked cast through its potential planner, whose
		// summed lethal guard the real census does not carry for a
		// TapsForMana watcher's damage (Manabarbs at 4 life: six taps)
		aff = s.kickedPotentialPainOK(k, p, c, o, &kc)
	}
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
		if aff && k.gen != nil && k.gen.anyFilt && s.scriptFilt(k, p).any() && s.mixBoard(k, p) {
			// a census gap that is a filter beside another gap kind (School of
			// the Unseen's {2},{T} beside Holdout Settlement's tapped creature):
			// gorge's census for the kicked total is incomplete and only its
			// budgeted prefix search prices the play, so the units census above
			// is no proof (CR 601.2g; tail-42 pair 49, Caligo Skin-Witch)
			aff = s.filtScript(k, MakeAction(AkCast, TCode(o), 0, 1), p, false)
		}
	}
	// A Squad/Replicate offer is gorge's replicate/squad option (mode word,
	// not the kicked mode the multikicker/offspring offer rides), so it needs
	// no gorgeOffersKicked cache read: the real affordability gate above is
	// the same one legal_walk_hand.go's offer uses.
	gorgeOK := cd.RepeatedKind == kop.RepeatSquad || cd.RepeatedKind == kop.RepeatReplicate || s.gorgeOffersKicked(p, o)
	if aff && s.kickedChainFeasible(k, c, p) && gorgeOK && (!k.sacMana || s.kickWitness(k, p, c, o, &kc)) && (k.gen == nil || !k.gen.anyKickCost || s.kickerCostOffer(k, p, c, o)) &&
		!s.dhWithholdsCast(k, p, o, &kc) && // a hand cast the Diamonds alone fund: gorge's planner prices no plan (census_incomplete, mana_discardhand.go)
		s.witnessKeepsTargets(k, p, c, o, &kc) { // a kicked total only a target-consuming sacrifice funds (Hurloon Battle Hymn's {W} from a Prism that is its only target): gorge's prefix search finds no script (cast_tgtkeep.go)
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
	if !(s.spellTargetsFeasibleFrom(k, c, p, o) || s.kickedWaivedFeasible(k, c, p)) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastableFrom(k, c, p, o, preDelve, plainBase, false, false, av) {
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

// kickedTotalCast reports whether action a is a mode-1 cast whose total is the
// card's cost plus kickerPayment: kickedArmOffer's kicker and multikicker
// cards, and the cards that keep their additional cost in CardDef.Kicker as
// the kicked option (Buyback, Offspring, Entwine; kw_offspring.go). The
// any-colour tap extra's owed pips read it (tapAnyColPips): Evincar's
// Justice with buyback owes {B}{B} beside the {5} the extras fund.
func (s *State) kickedTotalCast(k *prep, a Action) bool {
	if s.kickedArmOffer(k, a) {
		return true
	}
	if a.Kind() != AkCast || a.Mode()&3 != 1 {
		return false
	}
	return offspringOffer(k, s.objs[Obj(a.Source())].card)
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
	if paid, hid := s.kickedPotentialSansDeferred(k, p, c, o, kc, d); hid {
		return paid // a sorcery-speed cast the Taps-deferred sources alone fund is no play (kickedPotentialSansDeferred below)
	}
	if s.contribPays(k, p, c, o, true) {
		return false // the potential planner refuses a credit-bearing cast, and "kicked" is no altCastModes row (cost:credit)
	}
	if s.waterbendOwnPart(k, p, c, o) != 0 {
		return false // ... and a waterbend one (potentialModeCastPlan, "shape:contribution")
	}
	// kc's own {X} contributes nothing here: gorge's offer gate (legal_walk_hand
	// buybackCost/offerCastable) prices the composed {X} cost at X = 0
	// conservatively -- "a spell offering itself is withheld only when even X=0
	// is unpayable" (offerCostFor) -- and the seat announces X at the cast, so
	// the potential census paying the X-free total is exactly gorge's offer.
	// (The "largest X the sources fund" read is the lowered witness's pricing,
	// cast.go's kicked arm, not the offer gate.) affords ignores an {X} already.
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
	if s.sacRepeatAnyLive(k, p) {
		// A dedicated-token sacrifice-any source's census is unrelaxable in
		// gorge (paymentPlanRelaxedAlternatives: a repeatable cost that
		// removes no once-only part), so its planner prices the payment from
		// its census only when the printed sources and one activation pay it,
		// and otherwise through its budgeted prefix search
		// (potentialCensusFallback, potential_script.go) — the per-candidate
		// repeats the census records (legal.go sacAnyRepeatUnits) are no
		// proof of it, and the search they stand in for can exhaust its node
		// budget (Rude Awakening's entwined {6}{G}{G} beside four Prism
		// tokens: the search branches five colours per activation and dies
		// inside potentialScriptNodes, so gorge drops the offer). The kernel
		// asks the same search the walk's plays take (mana_tapanother.go
		// tapXScript) once the once-admission census (the repeats held out)
		// cannot price the total.
		return s.kickSacRepeatPays(k, p, c, o, d, kc)
	}
	if !potUnitsAvail(&px, &pu, int(s.pl[SeatIx(p)].life)) {
		if open {
			return false // no bounded census beside an unbounded fold
		}
		return true // the record is incomplete: keep the over-bound answer
	}
	return px.affords(kc, pipMaskOf(kc), 0) && potUnitsPainAffords(&pu, int(s.pl[SeatIx(p)].life), kc)
}

// sacRepeatAnyLive is sacRepeatLive (mana_tapanother.go) narrowed to an
// any-colour production: a live sacrifice-any mana source whose cost
// sacrifices dedicated tokens and whose production is any colour (Diamond
// Kaleidoscope's "Sacrifice a Prism token: Add one mana of any color"). Its
// census gorge cannot relax, so its repeats beyond the potential bound's one
// admission are the budgeted prefix search's proof, never a census one.
func (s *State) sacRepeatAnyLive(k *prep, p uint8) bool {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		ab := k.tapXAbOf(s.cardAt(b.first()))
		if kop.SacAnyManaAb(ab) && ab.ManaColors == kop.ColAny &&
			k.t.Filters[ab.Cost.Filter].Flags&kop.FToken != 0 {
			return true
		}
	}
	return false
}

// kickSacRepeatPays is the kicked cast's potential-pool answer beside a live
// dedicated-token sacrifice-any source (sacRepeatAnyLive). The once-admission
// census (sacAnyRepeatUnits held out) prices the total when the printed
// sources and one activation pay it: gorge's planner prices the payment from
// its census, whose prefix search's first leaf funds that one activation.
// Otherwise gorge's budgeted prefix search decides, and the kernel asks the
// same search the walk's plays take (tapXScriptOffered): a final drop whose
// line latched a rule gap is declared, the walk arms' shape.
func (s *State) kickSacRepeatPays(k *prep, p uint8, c CardID, o Obj, d *payDesc, kc *kop.ManaCost) bool {
	var pf, pc avail
	var pcu [maxPotUnits]potUnit
	nd := *d
	nd.noSacRepeat = true
	s.availPotentialFor(k, p, &pf, &nd, false, &pcu)
	life := int(s.pl[SeatIx(p)].life)
	if potUnitsAvail(&pc, &pcu, life) && pc.affords(kc, pipMaskOf(kc), 0) &&
		potUnitsPainAffords(&pcu, life, kc) {
		return true
	}
	offered, stall := s.tapXScriptOffered(k, MakeAction(AkCast, TCode(o), 0, 1), p)
	if !offered {
		s.capStall(stall)
		return false
	}
	return true
}

// kickedPotentialPainOK: the potential census's units afford kc within the
// payer's summed pain budget (potUnitsPainAffords). An incomplete or
// unbounded record keeps the caller's answer (true).
func (s *State) kickedPotentialPainOK(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost) bool {
	spell := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
	var pa avail
	var pu [maxPotUnits]potUnit
	if s.availPotentialFor(k, p, &pa, &spell, false, &pu) {
		return true
	}
	return potUnitsPainAffords(&pu, int(s.pl[SeatIx(p)].life), kc)
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

// capturesKickCount is a card whose self-ETB trigger carries its cast's
// payment count in pendingTrig.defSeat from the moment it is queued: a Squad
// card, or a pure Multikicker one (Replicate's trigger is a SpellCast one).
// The permanent may be sacrificed with the trigger on the stack, and gorge
// still reads the count off the trigger's last known information (CR 603.10a).
func capturesKickCount(c *kop.CardDef) bool {
	return c.RepeatedKind == kop.RepeatSquad || c.HasMultikicker && c.RepeatedKind != kop.RepeatReplicate
}

// kickedPotentialSansDeferred prices a sorcery-speed kicked cast's potential
// arm without the Taps-deferred sources (Scald beside an Island, CR 603.2,
// 605.3a). gorge's planner never lists such a source, so a kicked total only
// its mana pays is a census gap: the play is census_incomplete and only a
// prefix search could fund it, but the source's tap queues the watcher's
// trigger above the play and a sorcery-speed cast needs an empty stack
// (CR 307.1, 602.5d); the multikicked play finds no script (witness_not_offered
// at every node) and the autopay surface drops it. An instant or flash cast
// waits for no stack, so its potential arm keeps the deferred units. hid
// reports that the sources were hidden, paid the arm's answer without them.
func (s *State) kickedPotentialSansDeferred(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost, d *payDesc) (paid, hid bool) {
	if k.gen == nil || !k.gen.anyTapsDefer || !s.subs[kop.TrigTaps].any() || k.cards[c].instant || s.castFlash(k, p, c) {
		return false, false
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		if sl := b.first(); s.perms[sl].flags&pfTapped == 0 && s.hasManaAbility(k, sl) && s.tapsDeferred(k, p, sl) {
			m = m.with(sl)
		}
	}
	if !m.any() {
		return false, false
	}
	t := s.fork()
	defer s.unfork()
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		t.perms[sl].flags |= pfTapped
		t.touchSl(sl)
	}
	return t.kickedPotentialPays(k, p, c, o, kc, d), true
}
