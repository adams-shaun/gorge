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
			if !aff && !(k.restrSrc && s.restrSrcAffords(k, p, pd, mana, av, NoSlot, 0)) {
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
			if s.abilityTargetsFeasible(k, a, p, NoSlot, 0xff) {
				d.add(MakeAction(AkActivate, TCode(o), 0, uint8(ai)))
			}
		}
	}
}

// discardSelfCost pays CostDiscardSelf: the source card, still in its
// owner's hand, is discarded (gorge's Discard<1/CARDNAME> cost:
// events.DiscardCostCycling, a discard that fires Discarded triggers).
//
//go:noinline
func (s *State) discardSelfCost(k *prep, e *stackEntry) {
	if o := e.obj; o != NoObj && s.objs[o].zone == kop.ZoneHand {
		if k.t.Cards[e.card].Abilities[e.ab].AFlags&kop.AfCycling != 0 {
			if !s.parkCost(k, o, parkCycle, e.x) {
				s.cycleDiscard(k, o, e.x)
			}
			return
		}
		if !s.parkCost(k, o, parkDiscard, 0) {
			s.discardObj(k, o)
		}
	}
}
