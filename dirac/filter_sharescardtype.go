package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// sharedCardTypes are the card types sharesCardTypeWith intersects (CR 205.2;
// gorge's cardTypeWords): never a supertype (Basic, Legendary), a subtype the
// kernel keeps as a type bit (Aura, Equipment, Vehicle) or the token mark.
const sharedCardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
	kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred

// objCardTypes is o's card types where it now is: a battlefield permanent's
// current types (a land animated into a creature has both), anything else
// its card's printed types (gorge sharesCardTypeWith reads the layer-4 set on
// the battlefield and the printed face elsewhere, a graveyard card included).
func (s *State) objCardTypes(k *prep, o Obj) kop.TypeMask {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		return s.typesGen(k, ob.slot) & sharedCardTypes
	}
	return k.t.Cards[ob.card].Types & sharedCardTypes
}

// fwSharesCardType is kop.FwSharesCardType: the candidate shares a card type
// with the first object the referent names (Braids, Arisen Nightmare's
// Permanent.sharesCardTypeWith RememberedCard). A candidate with no card type,
// or an unbound referent, matches nothing (gorge's fail-closed reading).
//
//kmanifest:fword FwSharesCardType
func (s *State) fwSharesCardType(c *fctx, l *kop.FLit) bool {
	var ct kop.TypeMask
	switch {
	case c.perm():
		ct = s.typesGen(c.k, c.sl) & sharedCardTypes
	case c.c != NoCard:
		ct = c.k.t.Cards[c.c].Types & sharedCardTypes
	}
	if ct == 0 {
		return false
	}
	for _, r := range s.fwReferentObjs(c, l.R) {
		if s.objCardTypes(c.k, r)&ct != 0 {
			return true
		}
	}
	return false
}
