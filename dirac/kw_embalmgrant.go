package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Cursecloth Wrappings' pump (OpGrantEmbalmOwnCost): the target creature card
// in its owner's graveyard gains embalm until end of turn, the embalm cost
// being its mana cost (CR 702.128a). gorge's pump registers the derived
// keyword [Embalm:CardManaCost] on the card, and the granted-keyword offer
// (rules legal_granted.go grantedKeywordLines, cards.GrantedKeywordAbility)
// runs the printed kwEmbalm expander on a scratch face: one graveyard,
// sorcery-speed AB$ CopyPermanent of Self whose cost is the card's mana cost
// plus exiling the card itself, minting a white Zombie copy with no mana cost.
//
// The kernel records the grant as one object bit (mechEmbalmGrant: gorge
// dedups identical derived lines, so a second grant changes nothing), ended
// by a zone change (a new object, CR 400.7) or the cleanup, like a granted
// flashback (arenaobj.go). The body is the granting card's own latent ability
// (kgen lower_embalmgrant.go lowers gorge's expander output with a zero mana
// cost): no card gains an ability index, and the pool carries one body per
// granting card. EmbalmGrantMode is the activation's mode; the entry runs the
// body with the embalmed card as its source, and buildCostPreRaw charges the
// source card's own mana cost in place of the body's.

// EmbalmGrantMode is the AkActivate mode of a granted embalm (exported: the oracle's canonical form, canon/translate.go): the granted
// bit (0x80) with index 63, which no granting card has (the granted index
// space is 0..62), and no target (a granted ability names its granting host).
const EmbalmGrantMode uint8 = 0x80 | 63

// useEmbalmGrant arms the offer for a table that grants embalm.
var useEmbalmGrant = newGenUse(func(t *kop.Table) bool {
	_, _, ok := embalmBody(t)
	return ok
})

// embalmBody finds the latent embalm body a table's OpGrantEmbalmOwnCost
// names: the first granting card's, in card order.
func embalmBody(t *kop.Table) (c CardID, ab uint8, ok bool) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpGrantEmbalmOwnCost {
					return CardID(i), uint8(in.C), true
				}
			}
		}
	}
	return 0, 0, false
}

func init() {
	arenaDetect(mechEmbalmGrant, func(_ *kop.Table, in *kop.Instr) bool { return in.Op == kop.OpGrantEmbalmOwnCost })
}

// prepEmbalmGrant records the body's card and ability index on p.
func prepEmbalmGrant(t *kop.Table, p *prep) {
	p.embalmCard, p.embalmAb, p.anyEmbalmGrant = embalmBody(t)
}

// execGrantEmbalm is OpGrantEmbalmOwnCost: the graveyard card Obj gains the
// granted embalm until end of turn. A card that left the graveyard before the
// resolution gains nothing (gorge pump.go's zone guard).
//
//kmanifest:op OpGrantEmbalmOwnCost
func (s *State) execGrantEmbalm(k *prep, in *kop.Instr) (ok, done bool) {
	if o := s.objRefObj(in.Obj); o != NoObj && s.objs[o].zone == kop.ZoneGraveyard {
		if i := k.embalmIx(); int(i) < len(s.cold.mo) {
			s.cold.mo[i].set(o)
			s.touch(o) // the grant is part of the card's view
		}
	}
	return true, true
}

// embalmIx is mechEmbalmGrant's arena index; arenaNone when the table grants
// no embalm.
func (k *prep) embalmIx() uint8 {
	if k.gen == nil {
		return arenaNone
	}
	return k.gen.ix[mechEmbalmGrant]
}

// embalmGranted reports whether o holds a granted embalm.
func (s *State) embalmGranted(k *prep, o Obj) bool {
	i := k.embalmIx()
	return int(i) < len(s.cold.mo) && s.cold.mo[i].has(o)
}

// embalmClear ends a granted embalm on o (a zone change: a new object). The
// caller marks o changed when the clear ends an actual grant.
func (s *State) embalmClear(k *prep, o Obj) {
	if i := k.embalmIx(); int(i) < len(s.cold.mo) {
		s.cold.mo[i].clr(o)
	}
}

// embalmClearAll ends every granted embalm at the cleanup, marking each card
// whose grant this clear ends before dropping the bitset (encoreClearAll).
func (s *State) embalmClearAll(k *prep) {
	if i := k.embalmIx(); int(i) < len(s.cold.mo) {
		s.touchBits(&s.cold.mo[i])
		s.cold.mo[i] = objBits{}
	}
}

// embalmBodyAb is the latent body's ability when the table grants embalm.
func (k *prep) embalmBodyAb() *kop.Ability {
	return &k.t.Cards[k.embalmCard].Abilities[k.embalmAb]
}

// embalmCostOr is the mana cost of the ability entry e runs: the embalmed
// card's own mana cost when e is the granted embalm's body, else base.
func (s *State) embalmCostOr(k *prep, e *stackEntry, base kop.ManaCost) kop.ManaCost {
	if k.anyEmbalmGrant && e.card == k.embalmCard && e.ab == k.embalmAb {
		return k.t.Cards[s.objs[e.obj].card].Cost
	}
	return base
}

// priorityEmbalmGrant offers p's granted embalms: each creature card in p's
// graveyard holding the grant, in zone order, at sorcery speed, for the
// card's own mana cost (and the body's exile-self cost). It follows the
// graveyard walk's printed abilities (priorityGraveAct), the order gorge's
// grantedKeywordLines gives a derived line after the printed ones.
func (s *State) priorityEmbalmGrant(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if !sorc || !k.gen.uses.has(useEmbalmGrant) {
		return
	}
	pl := &s.pl[p]
	a := k.embalmBodyAb()
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		if !s.embalmGranted(k, o) || k.cards[s.objs[o].card].types&kop.TCreature == 0 || s.castHeldOut(o) {
			continue
		}
		if graveLossOn(k) && s.graveStampedHidden(k, o) {
			continue // the card lost all abilities and the grant's mark carries no timestamp (static_graveabilityloss.go)
		}
		mana := k.t.Cards[s.objs[o].card].Cost
		if k.gen.anyAbReduce {
			mana = s.abReducedGrave(k, a, o, p, mana) // the charge prices this embalm under a graveyard reduction static (cast.go abReducedCharge)
		}
		gd := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
		if !av.affords(&mana, pipMaskOf(&mana), 0) &&
			!(k.restrSrc && s.restrSrcAffords(k, p, gd, &mana, av, NoSlot, 0)) &&
			!s.actPoolAffordsDesc(k, p, &mana, gd, av) {
			continue
		}
		if k.costObj && !s.graveActOK(k, a, p, o) {
			continue
		}
		if s.probeAbilityFrom(k, a, p, s.objs[o].card) {
			d.add(MakeAction(AkActivate, TCode(o), 0, EmbalmGrantMode))
		}
	}
}
