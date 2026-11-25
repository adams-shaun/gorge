package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// UnearthGrantStats counts the granted unearth activations a StatUnearthGrant
// static offered a card without printed Unearth (activateGen's off-kw arm).
var UnearthGrantStats struct{ Activations atomic.Int64 }

// The granted unearth offer (CR 702.84a): a battlefield static granting
// Unearth to cards in their owner's graveyard (kop.StatUnearthGrant; kgen
// lower_offzone.go offZoneUnearth) is read live, as kw_offkwgrant.go reads a
// retrace grant: no mark is stamped on the card and no State byte records it.
// The offer runs the granting card's latent body -- the printed kwUnearth
// expander's AB$ ChangeZone with Unearth$ True -- with the affected graveyard
// card as its source, so the body's return moves that card and the OpUnearth
// it carries applies the haste and the exile-at-end-step riders the printed
// activation applies (kw_unearth.go).
//
// The action is AkActivate with the granted-mode byte 0x80|62 and the target
// byte naming the granting permanent. 0x80|62 is a reserved granted index no
// granting card has (no ledger grant holds it), beside the pump-granted
// embalm's 0x80|63 (kw_embalmgrant.go): the two cannot share a byte, because
// the pump names no grantor by target 0, which is also the code of a granting
// permanent at object index 0.
//
// The gates follow the graveyard walk's printed abilities
// (priorityGraveAct) and the granted embalm's (priorityEmbalmGrant), the
// order gorge's granted-keyword block gives a derived line after the printed
// ones: sorcery speed (the body's SorcerySpeed$ True), the printed-Unearth
// dedup (gorge printedKeywordCovered: a card the pile already expands offers
// once), the mana affordability funnel and the once-per-turn held-out gate.
// Gorge's remaining granted-keyword gates (an activation limit keyed on the
// line, an activation condition, an SVar gate) have no carrier: no corpus
// card grants Unearth beside one, and the lowering refuses the shapes
// beside which one could (an XAlternative$ cost, an {X}).

// unearthGrantMode is the AkActivate mode of a granted unearth activation:
// the granted byte (0x80) with reserved index 62, the target byte naming the
// granting permanent.
const unearthGrantMode uint8 = 0x80 | 62

// useOffKwGrant arms the offer for a table with a StatUnearthGrant static.
var useOffKwGrant = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatUnearthGrant && a.Static.GrantBody != 0 {
				return true
			}
		}
	}
	return false
})

// offKwBody is one CardManaCost-priced off-kw grant body: the granting card
// and the latent body's ability index. The body bakes the placeholder zero
// mana cost; the offer and the charge read the affected card's printed mana
// cost (prepOffKwGrant). Prep data, never a pinned genPrep field
// (TestNoNewFixedStateFields).
type offKwBody struct {
	card CardID
	ab   uint8
}

// prepOffKwGrant records a table's StatUnearthGrant statics: anyOffKwGrant
// turns the offer on, and every CardManaCost-priced body is recorded for
// buildCostPreRaw's charge. A body's zero baked mana cost marks the shape
// (a plain explicit zero refuses; kgen offZoneUnearth).
func prepOffKwGrant(t *kop.Table, p *prep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatUnearthGrant || a.Static.GrantBody == 0 {
				continue
			}
			p.anyOffKwGrant = true
			body := &t.Cards[i].Abilities[a.Static.GrantBody-1]
			if body.Cost.Mana.Generic == 0 && body.Cost.Mana.Pips == [kop.NumManaKinds]uint8{} && body.Cost.Mana.X == 0 {
				p.anyOffKwCC = true
				if p.offKwCCN < uint8(len(p.offKwCC)) {
					p.offKwCC[p.offKwCCN] = offKwBody{card: CardID(i), ab: a.Static.GrantBody - 1}
					p.offKwCCN++
				}
			}
		}
	}
}

// offKwCostOr is the mana cost the ability entry e runs for: the affected
// card's own printed mana cost when e is a CardManaCost-priced off-kw grant
// body, else base.
func (s *State) offKwCostOr(k *prep, e *stackEntry, base kop.ManaCost) kop.ManaCost {
	if !k.anyOffKwCC {
		return base
	}
	for i := uint8(0); i < k.offKwCCN; i++ {
		if b := &k.offKwCC[i]; b.card == e.card && b.ab == e.ab {
			return k.t.Cards[s.objs[e.obj].card].Cost
		}
	}
	return base
}

// offKwGrantBody is the latent body the StatUnearthGrant static on card c
// names, or -1 when c carries none. A card with two grants keeps the first:
// the lowering refuses a card whose face carries a second Unearth static
// (kgen offZoneUnearth), so one is all a lowered card has.
func offKwGrantBody(k *prep, c CardID) int {
	abs := &k.t.Cards[c].Abilities
	for i := range *abs {
		a := &(*abs)[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatUnearthGrant && a.Static.GrantBody != 0 {
			return int(a.Static.GrantBody) - 1
		}
	}
	return -1
}

// offKwUnearthStatic is the first StatUnearthGrant static in force that p
// controls whose filter matches card c owned by owner in the graveyard, with
// its source slot: the granting permanent the activation names. The card is
// read as a card in its zone: the static's Zone is its AffectedZone$
// (ZoneAny for All).
func (s *State) offKwUnearthStatic(k *prep, p, owner uint8, c CardID, o Obj) (*kop.Ability, Slot) {
	if k.gen == nil {
		return nil, NoSlot
	}
	var got *kop.Ability
	var gotSl Slot
	s.eachStaticC(k, sclsOffKwG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatUnearthGrant || a.Static.GrantBody == 0 || st.ctrl != p {
			return true
		}
		if a.Static.Zone != kop.ZoneAny && a.Static.Zone != kop.ZoneGraveyard {
			return true
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, owner, p, o, nil) {
			return true
		}
		got, gotSl = a, st.sl
		return false
	})
	return got, gotSl
}

// priorityUnearthGrant offers p's granted unearths: each card in p's
// graveyard a StatUnearthGrant static covers, in zone order, at sorcery
// speed (the body's SorcerySpeed$ True), for the body's cost -- the affected
// card's own mana cost for a CardManaCost grant.
func (s *State) priorityUnearthGrant(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if !sorc || !k.anyOffKwGrant || !k.gen.uses.has(useOffKwGrant) {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		card := s.objs[o].card
		cd := &k.t.Cards[card]
		if cd.HasUnearth {
			continue // the printed offer exists (gorge printedKeywordCovered): the derived line offers nothing
		}
		a, sl := s.offKwUnearthStatic(k, p, s.objs[o].owner(), card, o)
		if a == nil || s.castHeldOut(o) {
			continue
		}
		body := &k.t.Cards[s.cardAt(sl)].Abilities[a.Static.GrantBody-1]
		mana := &body.Cost.Mana
		var mc kop.ManaCost
		if body.Cost.Mana.Generic == 0 && body.Cost.Mana.Pips == [kop.NumManaKinds]uint8{} && body.Cost.Mana.X == 0 {
			mc = cd.Cost // the CardManaCost shape: the affected card's own mana cost
			mana = &mc
		}
		var red kop.ManaCost
		if k.gen.anyOwnReduce && body.OwnReduce != (kop.ValueRef{}) {
			red = s.ownReduced(k, body, o, p)
			mana = &red
		}
		if k.gen.anyAbReduce {
			red = s.abReducedGrave(k, body, o, p, *mana) // a graveyard-zone reduction static (cost_reduceability.go)
			mana = &red
		}
		gd := payDesc{class: pdAct, card: card, src: NoSlot}
		if !av.affords(mana, pipMaskOf(mana), 0) &&
			!(k.restrSrc && s.restrSrcAffords(k, p, gd, mana, av, NoSlot, 0)) &&
			!s.actPoolAffordsDesc(k, p, mana, gd, av) {
			continue
		}
		if k.costObj && !s.graveActOK(k, body, p, o) {
			continue
		}
		if body.Cost.Flags&kop.CostPayEnergy != 0 && !s.energyPayable(p, &body.Cost) {
			continue
		}
		if f := body.Cost.Flags; f&(kop.CostSac|kop.CostTapX|kop.CostPayLife) != 0 &&
			!s.pickCostPayable(k, p, &body.Cost, o) {
			continue
		}
		if s.probeAbilityFrom(k, body, p, card) {
			d.add(MakeAction(AkActivate, TCode(o), TCode(s.perms[sl].obj), unearthGrantMode))
		}
	}
}
