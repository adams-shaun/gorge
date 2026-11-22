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
			mana := &a.Cost.Mana
			var red kop.ManaCost
			if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
				red = s.ownReduced(k, a, o, p)
				mana = &red
			}
			if a.Cost.Flags&kop.CostSorcery != 0 && !sorc {
				continue
			}
			if a.Cost.Flags&kop.CostSac != 0 && s.countCostSac(k, p, &a.Cost, NoSlot) < int(a.Cost.N) {
				// a sacrifice part needs its candidates on the battlefield:
				// Edge of Autumn's cycling ("Sacrifice a land") is no option
				// before the first land drop (legal.go's canActivate census)
				continue
			}
			if !s.millRevealPayable(k, p, o, &a.Cost) {
				continue // cost_millcost.go: CR 701.17b's library (a hand source pays its own reveal)
			}
			if a.Cost.Flags&kop.CostPayLife != 0 && int(pl.life) < lifeCostOwed(&a.Cost) {
				continue // CR 119.4: Street Wraith's "Cycling-Pay 2 life" at 1 life
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
			census := !aff && !(k.restrSrc && s.restrSrcAffords(k, p, pd, mana, av, NoSlot, 0))
			if census && !s.handActTapXRoute(k, p) {
				continue
			}
			if !s.handActCensusPays(k, p, mana, pd, av) {
				continue
			}
			if mana.X > 0 && s.cfg&cfgGorgeUnpriced != 0 && !s.poolPaysAct(k, p, mana, pd) {
				// gorge's planner declines an {X} activation (PlanCostDetail
				// cost:x; legal.go's battlefield gate): a hand one (X cycling,
				// ) is offered only when the floating pool pays it
				continue
			}
			if a.AFlags&kop.AfNinjutsu != 0 && !s.ninjutsuOfferOK(k, a, p, o) {
				continue // its window and its Return candidate (kw_ninjutsu.go)
			}
			// The feasibility reads the source as the CARD the ability works
			// from (priorityGraveAct's probe, CR 702.16b): a hand ability's
			// source is the green Wasteland Viper in hand, so a Blightbeetle's
			// protection from green bars its bloodrush target and the offer is
			// withheld. abilityTargetsFeasible's bare probe carries no source
			// (its colours 0), so protection from a colour never matched.
			pa := MakeAction(AkActivate, TCode(o), 0, uint8(ai))
			if s.probeAbilityFrom(k, a, p, s.objs[o].card) && (!census || s.handActTapXPays(k, p, pa)) {
				d.add(pa)
			}
		}
	}
}

// handActCensusPays is gorge's hand-zone ability gate (legal_walk_battlefield:
// offerCastable on the floating pool, or autoPayOffer on the census): a hand
// activation is no potential play of its walk, so the any-mana script walk's
// 99 units never make it an offer: it is priced on the gaps' census, where a
// tap-another source (Urza, Lord High Artificer's "Tap an untapped artifact:
// Add {U}") is one relaxed unit, as the autopay planner's witness counts it.
// Urza alone pays no {2}; beside an untapped Swamp he does (Jhessian Zombies'
// typecycling, tail-64 pair 4, decisions 572 and 573). Cold, out of line.
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
	return t.affords(mana, pipMaskOf(mana), 0) || s.poolPaysAct(k, p, mana, pd)
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
