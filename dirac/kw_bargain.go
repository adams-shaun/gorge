package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Bargain (CR 702.166, Wilds of Eldraine), copied from gorge (rules
// optional_sacrifice.go's optSacBargain row, legal_walk_hand.go's hand walk,
// cast_init.go's modeFlags):
//
//   - the bargained cast is gorge's "bargained" cast mode, offered beside
//     the plain cast when the caster controls an artifact, an enchantment or
//     a token. A card never has Casualty and Bargain together (kgen refuses
//     it), so the cast reuses AkCastCasualty and the card's CardDef picks the
//     row. CardDef.Bargain is table data, not State.
//   - the election reuses the casualty build state (b.cas and the pkSac
//     pick, so it settles at payment with the other sacrifices) but poses
//     ChGenBargain, the answer kind the oracle maps to gorge's "bargain".
//   - the paid flag rides the kicked provenance: the cast sets the stack
//     entry's sfKicked, enterGen copies it to mechKicked, and ExKicked
//     (Count$Bargained/Count$Bargain) and F2Kicked (Card.Self+bargained)
//     read it. A copy is never cast, so copySpell clears it (CR 707.10).
//   - a Bargain card never queues Casualty's copy trigger.
//   - the paid flag is set at payment, so the target ask reads the election itself
//     (bargainElected): a part that happens only if it was bargained has its targets
//     chosen only then (CR 702.166d).
//
// DebugBargainNoFlag is the mutation hook: it suppresses the provenance bit
// so TestBargainLockstep proves the flag is what the readers see.
var DebugBargainNoFlag bool

// bargainCandAt reports whether sl may be sacrificed for Bargain: an
// artifact, an enchantment or a token (CR 702.166a). gorge's row spells the
// spec "Artifact,Enchantment,Card.token" (rules optional_sacrifice.go,
// ), so a non-artifact token (a creature token) is a candidate too.
func (s *State) bargainCandAt(k *prep, sl Slot) bool {
	return s.typesAt(k, sl)&(kop.TArtifact|kop.TEnchantment) != 0 || s.objs[s.perms[sl].obj].of&ofToken != 0
}

// bargainCands lists p's Bargain candidates in battlefield order, excluding
// a permanent already committed to another sacrifice cost (the shared
// picked test).
func (s *State) bargainCands(k *prep, p uint8, d *Decision) {
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.bargainCandAt(k, sl) && !s.picked(s.perms[sl].obj) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
}

// bargainOffer: p may cast o with bargain (an artifact, an enchantment or a
// token to sacrifice). Like casualty, the mode prices the plain cast, except
// for a StBargained cost static (a Spell.Bargain ReduceCost: Hamlet Glutton),
// which prices the bargained cast only (cost_reducespell.go).
func (s *State) bargainOffer(k *prep, p uint8, c CardID) bool {
	if !k.t.Cards[c].Bargain {
		return false
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if s.bargainCandAt(k, m.first()) {
			return true
		}
	}
	return false
}

// bargainOptions poses the sacrifice election; false = no candidate remains
// (gorge: "bargain no longer payable; casting without bargain", the cast
// degrading to the plain one).
func (s *State) bargainOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	d.N = 0
	s.bargainCands(k, e.ctrl, d)
	if d.N == 0 {
		s.bld().cas = 0
		return false
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChGenBargain, e.ctrl
	return true
}

// bargainElected: the entry being built is a bargained cast whose sacrifice is yet to
// be paid.
func (s *State) bargainElected(k *prep, e *stackEntry) bool {
	return e.kind == seSpell && k.t.Cards[e.card].Bargain && s.bld().cas != 0 && s.bld().cas < casConspire
}

// FeatBargainCard reports whether o's card has Bargain (the oracle's cast
// mode choice; kw_bargain.go).
func (s *State) FeatBargainCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Bargain
}

// offerBargainPlay offers the bargained cast of o through a permission
// (Johann, Apprentice Sorcerer's library-top cast): gorge's permission walk
// prices the bargained mode apart from the plain one, from the same base, so
// a card the bargain discount alone makes payable is still offered. The
// action is the hand's AkCastCasualty, its mode the permission's
// (beginCasualtyCast).
func (s *State) offerBargainPlay(k *prep, d *Decision, p uint8, o Obj, c CardID, mode uint8, anyMana bool, av *avail) {
	if !k.bargain || mode != castMayPlay || anyMana || !s.bargainOffer(k, p, c) {
		return
	}
	cost := s.bargainedPrice(k, p, c, o, k.cards[c].cost)
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
		return
	}
	if !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) ||
		!s.altAddCastableFrom(k, c, p, o, &cost, &k.cards[c].cost, true, false, av) || s.castHeldOut(o) {
		return
	}
	if s.bargainOffers(k, p, o) {
		d.add(MakeAction(AkCastCasualty, TCode(o), 0, mode))
	}
}
