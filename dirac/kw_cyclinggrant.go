package dirac

import (
	"slices"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CyclingGrantStats counts the granted cycling activations a StatCyclingGrant
// static offered a hand card (activateGen's hand arm).
var CyclingGrantStats struct{ Activations atomic.Int64 }

// The granted cycling offer (CR 702.29a, 702.29e): a battlefield static
// granting Cycling or TypeCycling to cards in a hand (kop.StatCyclingGrant;
// kgen lower_offzone.go offZoneCycling) is read live, as kw_unearthgrant.go
// reads a graveyard unearth grant: no mark is stamped on the card and no
// State byte records it. The offer runs the granting card's latent body -- the
// printed cycling expansion's AB$ with ActivationZone$ Hand -- with the
// affected hand card as its source, so the body's own discard-this cost
// discards that card (CostDiscardSelf; discardSelfCost fires the cycled
// triggers off the body's AfCycling flag).
//
// The action is AkActivate with unearthGrantMode (the granted byte 0x80 with
// the reserved index 62, the target byte naming the granting permanent). The
// graveyard unearth offer and this hand one share the byte: the activation
// reads the affected card's zone first, and no card is offered both.
//
// The offer is the hand's OWNER's, whoever controls the static ("each Sliver
// card in each player's hand", Homing Sliver); the filter is read from the
// static controller's side (a YouOwn grant matches the controller's own
// cards). The gates are the printed hand walk's (handActOffer). The statics
// are read in perm.ts order (eachStaticTS): gorge attributes an identical line
// two sources grant to the oldest one (KeywordGrantSource). The printed dedup
// is gorge's printedKeywordCovered: a card whose printed face already carries
// the SAME line (an equal cost and an equal body here) offers it once, and two
// grants of the same line offer once.
//
// Not modelled (no corpus carrier lowers with it): an activation limit or
// condition on the granted line, and a ninjutsu grant (Satoru Umezawa keeps
// its refusal).

// useCyclingGrant arms the offer for a table with a StatCyclingGrant static.
var useCyclingGrant = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCyclingGrant && a.Static.GrantBody != 0 {
				return true
			}
		}
	}
	return false
})

// prepCyclingGrant records whether a table has a StatCyclingGrant static
// (prep data, never a pinned genPrep field: TestNoNewFixedStateFields).
func prepCyclingGrant(t *kop.Table, p *prep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCyclingGrant && a.Static.GrantBody != 0 {
				p.anyCyclingGrant = true
				return
			}
		}
	}
}

// cyclingGrantBody is the latent body the StatCyclingGrant static on card c
// names, or -1 when c carries none (the lowering refuses a second one).
func cyclingGrantBody(k *prep, c CardID) int {
	abs := &k.t.Cards[c].Abilities
	for i := range *abs {
		a := &(*abs)[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCyclingGrant && a.Static.GrantBody != 0 {
			return int(a.Static.GrantBody) - 1
		}
	}
	return -1
}

// isCyclingGrantBody reports whether ability ai of card cd is the latent body
// a StatCyclingGrant static of the same card names: not an ability the card
// itself has in hand, so the printed hand walk skips it.
func isCyclingGrantBody(cd *kop.CardDef, ai int) bool {
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCyclingGrant && int(a.Static.GrantBody) == ai+1 {
			return true
		}
	}
	return false
}

// sameCyclingBody reports whether two cycling bodies are one line: an equal
// cost and an equal program (a plain cycling and a typecycling of the same
// cost differ by their search).
func sameCyclingBody(a, b *kop.Ability) bool {
	return a.Cost == b.Cost && slices.EqualFunc(a.Prog, b.Prog, func(x, y kop.Instr) bool { return x == y })
}

// priorityCyclingGrant offers p's granted cycling activations: each card in
// p's hand a StatCyclingGrant static covers, in hand order, one per distinct
// granted line the card does not already print.
func (s *State) priorityCyclingGrant(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if !k.anyCyclingGrant || !k.gen.uses.has(useCyclingGrant) {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		card := s.objs[o].card
		cd := &k.t.Cards[card]
		var seen [4]*kop.Ability
		nseen := 0
		for ai := range cd.Abilities {
			if a := &cd.Abilities[ai]; a.Kind == kop.AbActivated && a.ActZone == kop.ZoneHand && a.AFlags&kop.AfCycling != 0 && a.AFlags&kop.AfLatent == 0 && nseen < len(seen) {
				seen[nseen] = a // a printed line: gorge's printedKeywordCovered
				nseen++
			}
		}
		s.eachStaticTS(k, sclsOffKwG, func(st statSrc) bool {
			a := st.a
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCyclingGrant || a.Static.GrantBody == 0 {
				return true
			}
			if a.Static.Zone != kop.ZoneAny && a.Static.Zone != kop.ZoneHand {
				return true
			}
			if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, card, s.objs[o].owner(), st.ctrl, o, nil) {
				return true
			}
			body := &k.t.Cards[s.cardAt(st.sl)].Abilities[a.Static.GrantBody-1]
			for j := 0; j < nseen; j++ {
				if sameCyclingBody(seen[j], body) {
					return true
				}
			}
			if nseen < len(seen) {
				seen[nseen] = body
				nseen++
			}
			s.handActOffer(k, d, p, sorc, av, o, body, MakeAction(AkActivate, TCode(o), TCode(s.perms[st.sl].obj), unearthGrantMode))
			return true
		})
	}
}
