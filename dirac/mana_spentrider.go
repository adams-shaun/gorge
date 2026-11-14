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
// The kernel taps a source for a cast inside that cast's payment and spends
// the pool at once, so the rider's mana is spent exactly when the payment
// tapped the source: the planner's tap sites record it in build.obsSl beside
// the Tf2ManaFromSelf reader (noteTapSrc, mana_spendtrigger.go). finishBuild
// matches the spell against the source's kop.Ability.ManaHasteValid and stamps
// the spell entry (spellManaHaste); the permanent it becomes gains haste until
// end of turn as it enters (manaHasteEnter). A copy does not keep the stamp
// (copy_gen.go keeps only spellFirstCast). Guard: genPrep.anyManaHaste.

// spellManaHaste is the spell entry's remVer bit 3 (bits 0-2:
// spellGainedDeathtouch, spellGainedLifelink, spellFirstCast).
const spellManaHaste objWord = 1 << 3

// DebugManaHasteOff drops the rider (mutation test only).
var DebugManaHasteOff bool

// noteHasteSrc records that the cast being built tapped the rider source at
// sl. Every lowered rider is the same grant (kgen manaSpentRiders), so a
// second rider source keeps the first; a Tf2ManaFromSelf reader's source
// beside it stalls loudly, as two readers do.
func (s *State) noteHasteSrc(k *prep, sl Slot) {
	b := s.bld()
	if b.obsSl == 0 {
		b.obsSl = sl + 1
		return
	}
	if b.obsSl != sl+1 && k.cards[s.cardAt(b.obsSl-1)].castSrc&(csManaHaste|csObserved) != csManaHaste {
		s.capFull(StallObsSrc)
	}
}

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
