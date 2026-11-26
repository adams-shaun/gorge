package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// priorityHandAct offers the activated abilities that work from the hand
// (ActZone hand: cycling, typecycling and channel, whose cost discards the
// card, CostDiscardSelf), as priorityGraveAct does the graveyard's: gorge
// lists them among the priority options for each hand card in hand order.
//
//go:noinline
func (s *State) priorityHandAct(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		cd := &k.t.Cards[s.objs[o].card]
		for ai := range cd.Abilities {
			a := &cd.Abilities[ai]
			if a.Kind != kop.AbActivated || a.ActZone != kop.ZoneHand {
				continue
			}
			if k.anyCyclingGrant && a.AFlags&kop.AfLatent != 0 && isCyclingGrantBody(cd, ai) {
				continue // the body a StatCyclingGrant static grants, never the grantor's own hand ability (kw_cyclinggrant.go)
			}
			s.handActOffer(k, d, p, sorc, av, o, a, MakeAction(AkActivate, TCode(o), 0, uint8(ai)))
		}
	}
}

// handActOffer is one hand-zone activated ability a of the card o in p's hand
// through priorityHandAct's gates: pa is the action it offers (the card's own
// ability index, or a StatCyclingGrant's granted action, kw_cyclinggrant.go).
//
//go:noinline
func (s *State) handActOffer(k *prep, d *Decision, p uint8, sorc bool, av *avail, o Obj, a *kop.Ability, pa Action) {
	pl := &s.pl[p]
	mana := &a.Cost.Mana
	var red kop.ManaCost
	if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
		red = s.ownReduced(k, a, o, p)
		mana = &red
	}
	if a.Cost.Flags&kop.CostSorcery != 0 && !sorc {
		return
	}
	if s.castHeldOut(o) {
		return // CR 733.2: reversed twice in a row with no progress holds the card's every offer out (gorge castSuppressed), as priorityGraveAct reads it
	}
	if a.Cost.Flags&kop.CostSac != 0 && s.countCostSac(k, p, &a.Cost, NoSlot) < int(a.Cost.N) {
		// a sacrifice part needs its candidates on the battlefield:
		// Edge of Autumn's cycling ("Sacrifice a land") is no option
		// before the first land drop (legal.go's canActivate census)
		return
	}
	if !s.millRevealPayable(k, p, o, &a.Cost) {
		return // cost_millcost.go: CR 701.17b's library (a hand source pays its own reveal)
	}
	if a.Cost.Flags&kop.CostPayLife != 0 && int(pl.life) < lifeCostOwed(&a.Cost) {
		return // CR 119.4: Street Wraith's "Cycling-Pay 2 life" at 1 life
	}
	pd := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
	aff := av.affords(mana, pipMaskOf(mana), 0)
	if !aff && s.rbAny(p) {
		// floating restricted mana the hand ability may spend (CR 106.6):
		// a scripted prefix (Flamebraider's Elemental mana) taps the
		// source into the pool before the activation, and the search's
		// restrOptions walk must list it or no script completes.
		if m := s.restrPoolAct(k, p, &pd); m != 0 {
			t := *av
			s.restrAvailAdd(p, m, &t)
			aff = t.affords(mana, pipMaskOf(mana), 0)
		}
	}
	census := !aff && !(k.restrSrc && s.restrSrcAffords(k, p, pd, mana, av, NoSlot, 0)) && !(mana == &a.Cost.Mana && s.handActFiltAffords(k, p, mana, av))
	script := census && (s.handActScriptPays(k, p, pa) || s.handActFiltPays(k, a, p, pa))
	if census && !script && !s.handActTapXRoute(k, p) {
		return
	}
	if !s.handActCensusPays(k, p, mana, pd, av) {
		return
	}
	if a.AFlags&kop.AfNinjutsu != 0 && !s.ninjutsuOfferOK(k, a, p, o) {
		return // its window and its Return candidate (kw_ninjutsu.go)
	}
	// The feasibility reads the source as the CARD the ability works
	// from (priorityGraveAct's probe, CR 702.16b): a hand ability's
	// source is the green Wasteland Viper in hand, so a Blightbeetle's
	// protection from green bars its bloodrush target and the offer is
	// withheld. abilityTargetsFeasible's bare probe carries no source
	// (its colours 0), so protection from a colour never matched.
	if s.probeAbilityFrom(k, a, p, s.objs[o].card) && (!census || script || s.handActTapXPays(k, p, pa)) {
		d.add(pa)
	}
}

// handActFiltAffords: the paid filter sources (Golgari Signet's "{1}, {T}: Add
// {B}{G}") fund a hand activation's mana beside the plain sources, each fee
// taken out of the other mana (CR 605.3a-b, 602.2b): Spectator Seating's {W}
// pays the Signet's {1}, and the Signet's {B}{G} pays Hollow One's cycling
// {2}. gorge prices a hand ability on the payment planner's census, which
// lists such a filter; the plain census leaves every filter out. The window's
// own filter walk (faceReachFilters) charges each fee from the rest, so a
// filter whose fee takes the only mana of a colour the cost needs does not
// pay it (Herd Migration's {1}{G} over a Bountiful Promenade and a Desolate
// Mire). Only the printed cost: a reduced one (Otawara's channel, costs {1}
// less per legendary creature) stays priced by the plain census alone, where
// the filter walk offers a play gorge does not (measured on the Otawara
// repro). Only in the real offer walk (availX 0): the script walks (availX
// nonzero) price a hand activation on the census alone, as the payment's
// prefix search does, so a filter-funded cycling the walk admitted there left
// the play's own payment to the plain planner and stranded the card in hand
// (Wormwood Treefolk's Elven Chorus mana, Minion of Tevesh Szat's Fetid Heath
// fee). Cold, out of line.
//
//go:noinline
func (s *State) handActFiltAffords(k *prep, p uint8, mana *kop.ManaCost, av *avail) bool {
	if s.g.availX != 0 || k.gen == nil || !k.gen.anyFilt || !s.scriptFilt(k, p).any() {
		return false
	}
	return s.faceReachFilters(k, p, av, mana, 0, NoSlot)
}

// handActCensusPays is gorge's hand-zone ability gate (legal_walk_battlefield:
// offerCastable on the floating pool, or autoPayOffer on the census): a hand
// activation is no potential play of its walk, so the any-mana script walk's
// 99 units never make it an offer: it is priced on the gaps' census, where a
// tap-another source (Urza, Lord High Artificer's "Tap an untapped artifact:
// Add {U}") is one relaxed unit, as the autopay planner's witness counts it.
// Urza alone pays no {2}; beside an untapped Swamp he does (Jhessian Zombies'
// typecycling, tail-64 pair 4, decisions 572 and 573). A live Combo Any source
// beside a granted mana ability (Sage of the Maze beside Springleaf Parade's
// tokens) opens gorge's potential walk in every colour, and its census-gap arm
// prices the hand activation by the prefix search (Netherborn Phalanx's
// transmute {1}{B}{B}): the gaps census holds no Combo Any mana, so the walk
// lists the activation and grantCap's search (grantSearch) decides it, as it
// does for a cast. Cold, out of line.
//
//go:noinline
func (s *State) handActCensusPays(k *prep, p uint8, mana *kop.ManaCost, pd payDesc, av *avail) bool {
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 || s.g.availX&availAny == 0 {
		return true
	}
	x := s.g.availX
	s.g.availX = availGaps | availPlain
	var t avail
	s.availFor(k, p, &t)
	s.g.availX = x
	return t.affords(mana, pipMaskOf(mana), 0) || s.poolPaysAct(k, p, mana, pd) || s.sacRepeatPays(k, p, mana, pd) || s.caOpenGrant(k, p)
}

// sacRepeatPays: a live sacrifice-any source whose cost sacrifices dedicated
// tokens (Diamond Kaleidoscope's "Sacrifice a Prism token: Add one mana of any
// color", sacRepeatLive) activates once per candidate in the planner's witness
// (gorge's autoPayOffer; CR 605.3b), so the gaps census's one relaxed unit
// per source undercounts it. The potential census prices the repeats, one
// unit per candidate (sacAnyRepeatUnits), as it does for a battlefield
// activation (Bone Sabres' equip, Jungle Patrol's Wood), and a hand activation
// the repeats fund is offered too (Titanoth Rex's cycling beside two Prism
// tokens, tail-12-c000 pair 93). Cold, out of line.
//
//go:noinline
func (s *State) sacRepeatPays(k *prep, p uint8, mana *kop.ManaCost, pd payDesc) bool {
	if !s.sacRepeatLive(k, p) {
		return false
	}
	var pot avail
	s.availPotentialFor(k, p, &pot, &pd, false)
	return pot.affords(mana, pipMaskOf(mana), 0)
}

// handActScriptPays is the planner witness's half of gorge's hand-zone gate
// (autoPayOffer, CR 602.2b): the plain census holds no unit for a mana
// ability whose own cost is mana (Crossroads Candleguide's "{2}: Add one mana
// of any color"), but the planner's witness taps it, so the activation's
// filter script prices what the census cannot (actScriptPays, the battlefield
// activation's route). Cold, out of line.
//
//go:noinline
func (s *State) handActScriptPays(k *prep, p uint8, pa Action) bool {
	if k.gen == nil || s.cfg&cfgGorgeLowering == 0 || s.g.availX&(availNoActScript|availWalk) != 0 || s.pool != nil && s.pool.inFilt != 0 {
		return false
	}
	if !s.scriptFilt(k, p).any() {
		return false
	}
	x := s.g.availX
	s.g.availX = x | availNoHandFund
	ok := s.filtScript(k, pa, p, false)
	s.g.availX = x
	return ok
}

// discardSelfCost pays CostDiscardSelf: the source card, still in its
// owner's hand, is discarded (gorge's Discard<1/CARDNAME> cost:
// events.DiscardCostCycling, a discard that fires Discarded triggers).
//
//go:noinline
func (s *State) discardSelfCost(k *prep, e *stackEntry) {
	if o := e.obj; o != NoObj && s.objs[o].zone == kop.ZoneHand {
		if k.t.Cards[e.card].Abilities[e.ab].AFlags&kop.AfCycling != 0 {
			if !s.parkCost(k, o, parkCycle, uint8(e.x)) {
				s.cycleDiscard(k, o, uint8(e.x))
			}
			return
		}
		if !s.parkCost(k, o, parkDiscard, 0) {
			s.discardObj(k, o)
		}
	}
}

// handActFiltPays prices a hand activation the printed census cannot pay on a
// board whose filters include a net-negative one (Llanowar Envoy's repeatable
// "{1}{G}: Add one mana of any color"): gorge's potential plan chains the
// filter as often as its fee is funded (Seething Landscape's Cycling {U}{B}{R}
// off a Swamp and two Envoy activations; CR 602.2b, 605.3a-b, 601.2g), which
// the census counts once and actScriptPays declines wholesale (a battlefield
// activation keeps that answer). The filter search is the payment itself, so
// its verdict is exact; only the real offer walk asks (an outer walk's census
// stays the plain one) and never from inside a search. Cold, out of line.
//
//go:noinline
func (s *State) handActFiltPays(k *prep, a *kop.Ability, p uint8, pa Action) bool {
	if k.gen == nil || !plainPricedCost(a) || s.g.availX != 0 || s.cfg&cfgGorgeAutopay == 0 || s.pool != nil && s.pool.inFilt != 0 {
		return false
	}
	f := s.scriptFilt(k, p)
	if !f.any() || s.filtNetNegative(k, f) == 0 {
		return false
	}
	av := s.g.availX
	s.g.availX = av | availNoHandFund
	ok := s.filtScript(k, pa, p, false)
	s.g.availX = av
	return ok
}
