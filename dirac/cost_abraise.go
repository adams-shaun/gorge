package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A StatRaiseAbility static (RaiseCost Type$ Ability; CR 601.2f, 602.2b):
// activated abilities of battlefield permanents matching its filter cost
// Static.A (VA under StDynamic) generic more or owe a Sac<1/<filter>> pick
// (kop.RaiseAbPick, the pick's permanents in Static.Filter2) at the
// activation's cost stage. gorge composes every
// matching static's raise onto the total before the reductions (pay.ApplyTo:
// activate.go's mods applied at the payment); the kernel composes the generic
// into the priced cost the same way (abPlannedCost for the offer's planner
// price, abRaisedCharge at the build charge, abRaised at the offer gate),
// poses the pick as the entry's pick cost (buildPickCost, gorge's sac ask
// covering the raise's part). Every matching static composes; the pick part
// poses the first one's pick filter once per matching static (a second pick card is a different Cost
// than the one a build poses, unmodelled: cardset's raise guard refuses a pool
// holding two cards with a pick; copies of one card compose as one Sac<N/f>).
// gorge's target ask places the raise's pick like any cost part (its probe no
// longer re-folds the static's Sac part), so an offer abRaiseOK admits (N
// candidates) is built.
// A mana-abilities' life raise (RaiseAbMana with StRaiseLife, Thran Portal's
// "Mana abilities of CARDNAME cost an additional 1 life to activate", CR 605.3b,
// 602.2b) is the mana window's price, not this file's: the static is
// self-only (kgen refuses any other ValidCard$) and the fold below prices its
// life at each tap of the source (prepLandGrant's views carry it as the pain
// a Mana Confluence-class tap pays; mana_pain.go). The guard keeps every
// other mana-ability reach out (cardset.AbRaiseRefusal's mana arm, and its
// grant arm for the self-priced one).
// Cold code: every site is behind genPrep.anyAbRaise.

// DebugNoAbRaise drops the raise (TestAbraiseMutation proves the pool
// activates a raised ability at its raised price).
var DebugNoAbRaise bool

// AbRaisedPriced counts the ability pricings a StatRaiseAbility lowered
// (read by TestAbraiseLockstep; nothing in the engine reads it).
var AbRaisedPriced atomic.Int64

// abRaiseStaticApplies is the per-static gate of a StatRaiseAbility static st
// on ability a of the battlefield source sl activated by ctrl: the condition
// and the ValidCard$ filter over the source.
func (s *State) abRaiseStaticApplies(k *prep, st statSrc, a *kop.Ability, sl Slot, ctrl uint8) bool {
	sa := &st.a.Static
	if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatRaiseAbility {
		return false
	}
	if sa.B&kop.RaiseAbMana != 0 && a.Kind != kop.AbMana {
		return false // a mana-abilities-only raise never raises another ability
	}
	return s.statCond(k, st) && s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull)
}

// abRaiseParts walks the raise statics of ability a of the battlefield source
// sl activated by ctrl: the generic raise, the pick filter of the
// first matching pick part and how many matching statics carry one (each owes
// its own Sac<1/f>; the matchup guard keeps every pick raiser one card, so the
// filter is shared: three Brutal Suppressions owe three sacrifices). A
// life raise (StRaiseLife, the mana window's price) prices no generic and no
// pick here.
func (s *State) abRaiseParts(k *prep, a *kop.Ability, sl Slot, ctrl uint8) (g int, pick kop.FilterID, picks int) {
	if DebugNoAbRaise || a.Kind != kop.AbActivated && a.Kind != kop.AbMana || sl == NoSlot {
		return 0, 0, 0
	}
	s.eachStaticG(k, func(st statSrc) bool {
		if !s.abRaiseStaticApplies(k, st, a, sl, ctrl) {
			return true
		}
		sa := &st.a.Static
		if sa.StFlags&kop.StRaiseLife != 0 {
			return true // the life is the mana window's price, not generic
		}
		n := int(sa.A)
		if sa.StFlags&kop.StDynamic != 0 {
			n = s.evalV(k, sa.VA, s.ctxStatic(st))
		}
		if n > 0 {
			g += n
		}
		if sa.B&kop.RaiseAbPick != 0 {
			if picks == 0 {
				pick = sa.Filter2
			}
			picks++
		}
		return true
	})
	return g, pick, picks
}

// manaLifeRaise is the life a card's self StatRaiseAbility mana-ability raises
// charge each activation of its own mana abilities (RaiseAbMana with
// StRaiseLife, Thran Portal's "Mana abilities of CARDNAME cost an additional 1
// life to activate"; CR 305.7, 605.3b, 602.2b). Every non-latent printed mana
// ability of the card beside one is not folded: the fold prices the intrinsic
// views' taps only (static_landtype.go prepLandGrant), so the caller stalls on
// a printed mana ability (StallLandGrant).
func manaLifeRaise(c *kop.CardDef) uint8 {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			return 0
		}
	}
	n := uint8(0)
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatRaiseAbility &&
			a.Static.B&kop.RaiseAbMana != 0 && a.Static.StFlags&kop.StRaiseLife != 0 {
			n += uint8(a.Static.A)
		}
	}
	return n
}

// abRaised is the composition the offer gate and the planner price: c with
// every matching static's generic raise on top (raises before reductions, CR
// 601.2f; the pick part rides the build's own payments, the offer gate
// abRaiseOK prices it).
func (s *State) abRaised(k *prep, a *kop.Ability, sl Slot, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	g, _, picks := s.abRaiseParts(k, a, sl, ctrl)
	if picks == 0 && g == 0 {
		return c
	}
	if g > 0 {
		c.Generic = uint8(min(int(c.Generic)+g, 255))
		AbRaisedPriced.Add(1)
	}
	return c
}

// abRaisedCharge is abRaised for an ability stack entry being charged
// (buildCostPreRaw's seAbility arm): the source is the entry's object.
func (s *State) abRaisedCharge(k *prep, a *kop.Ability, o Obj, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return c
	}
	return s.abRaised(k, a, ob.slot, ctrl, c)
}

// abRaiseOK is the offer gate: p's activation of a owes every matching
// static's pick part, and can pay it (gorge's offer gate prices the
// composed cost: pay.NonManaCastable over the raise's pick).
func (s *State) abRaiseOK(k *prep, sl Slot, a *kop.Ability, p uint8) bool {
	if k.gen == nil || !k.gen.anyAbRaise {
		return true
	}
	_, pick, picks := s.abRaiseParts(k, a, sl, p)
	if picks > 0 && s.countCostSac(k, p, &kop.Cost{Flags: kop.CostSac, N: uint8(picks), Filter: pick}, NoSlot) < picks {
		return false
	}
	return true
}

// abRaisePick is the pick part the entry being built owes, or nil.
func (s *State) abRaisePick(k *prep, e *stackEntry) *kop.Cost {
	a := s.buildAbility(k)
	if a == nil {
		return nil
	}
	_, pick, picks := s.abRaiseParts(k, a, s.srcSlot(), e.ctrl)
	if picks == 0 {
		return nil
	}
	return &kop.Cost{Flags: kop.CostSac, N: uint8(picks), Filter: pick}
}
