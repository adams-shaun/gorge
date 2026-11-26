package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// AddsKeywords$ Haste on a mana ability (Carnelian Orb of Dragonkind: "If
// that mana is spent on a Dragon creature spell, it gains haste until end of
// turn"; CR 106.6). gorge floats the mana in a source-bearing batch that a
// payment carves before the plain units of its colour, captures the grant at
// the spell's payment when the spell matches the rider's AddsKeywordsValid$
// (pay.emitRestrictedManaSpend), and applies it as the permanent the spell
// became enters (rules/altcast.go applyManaKeywordGrant: a layer-6 grant
// until end of turn).
//
// The kernel taps a source for a cast inside that cast's payment and pools its
// unit as plain mana; gorge spends plain units before the typed one, so a
// unit the payment did not need floats on for a later cast. mana_hastefloat.go
// tallies the floating rider units and records the source in build.obsSl
// when a spell's payment spends one. finishBuild
// matches the spell against the source's kop.Ability.ManaHasteValid and stamps
// the spell entry (spellManaHaste); the permanent it becomes gains haste until
// end of turn as it enters (manaHasteEnter). A copy does not keep the stamp
// (copy_gen.go keeps only spellFirstCast). Guard: genPrep.anyManaHaste.

// spellManaHaste is the spell entry's remVer bit 3 (bits 0-2:
// spellGainedDeathtouch, spellGainedLifelink, spellFirstCast).
const spellManaHaste objWord = 1 << 3

// DebugManaHasteOff drops the rider (mutation test only).
var DebugManaHasteOff bool

// manaHasteRider stamps the spell just pushed when its payment tapped a rider
// source and the spell matches that source's ManaHasteValid (gorge
// RestrictValidMatches with the source's controller as "you"; the filter
// carries no cast-provenance word, kgen lower_manaspent.go).
//
//go:noinline
func (s *State) manaHasteRider(k *prep) {
	b := s.bld()
	if b.obsSl == 0 || b.bfl&bflPushed == 0 || s.sp == 0 || DebugManaHasteOff {
		return
	}
	sl := b.obsSl - 1
	cd := &k.t.Cards[s.cardAt(sl)]
	var f kop.FilterID
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbMana && a.ManaHasteValid != 0 {
			f = a.ManaHasteValid
			break
		}
	}
	e := &s.stack[s.sp-1]
	if f == 0 || e.kind != seSpell || e.obj != b.e.obj {
		return
	}
	fl := k.t.Filters[f]
	fl.Zone = kop.ZoneNone // the spell as it is paid for
	// the pushed spell object, not its printed card: a face-down spell is a
	// colourless typeless-but-creature 2/2 and a bestowed spell an Aura
	// (CR 708.4, 702.103b; gorge state.Object.FaceDownHidden)
	if p := s.perms[sl].ctrl; s.matchCardOne(k, &fl, e.card, p, p, e.obj) {
		e.remVer |= spellManaHaste
		b.e.remVer |= spellManaHaste // a later syncBuild keeps it
	}
}

// manaHasteEnter grants haste until end of turn to the permanent at sl that
// the stamped spell e became.
func (s *State) manaHasteEnter(k *prep, sl Slot, e *stackEntry) {
	if sl == NoSlot || e.remVer&spellManaHaste == 0 {
		return
	}
	s.grantEOT(sl, kop.KwHaste)
	s.promoKw(k, sl, kop.KwHaste)
	s.touchSl(sl)
}
