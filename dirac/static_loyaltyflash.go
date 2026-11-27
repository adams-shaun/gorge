package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The loyalty-timing grant (kop.StatLoyaltyFlash): a CastWithFlash static
// whose ValidSA$ is Activated.Loyalty with Caster$ You ("you may activate
// loyalty abilities of <Filter> any time you could cast an instant"). gorge
// rules/loyalty_flash.go loyaltyAtInstantSpeed reads it from a printed static
// (The Wandering Emperor's Card.Self+ThisTurnEntered, Teferi, Master of Time)
// and from an Effect-delivered registration (Teferi, Temporal Archmage's
// emblem, Jace's Machinations), both for the static's controller only. The
// grant changes the timing alone: canActivate keeps CR 606.3's
// once-per-permanent-per-turn limit (loyaltyUsedAtLimit).

// DebugLoyaltyFlashOff is the mutation switch (TestInstantLoyaltyActivation):
// the grant reads false, so loyalty abilities stay sorcery-speed only.
var DebugLoyaltyFlashOff bool

// useLoyaltyFlash arms the read for a table carrying the static on a card or
// an emblem.
var useLoyaltyFlash = newGenUse(func(t *kop.Table) bool {
	return tableHasStatic(t, kop.StatLoyaltyFlash)
})

// loyaltyFlash reports whether a live StatLoyaltyFlash static lets the
// controller of the permanent at sl activate its loyalty abilities at instant
// speed: a battlefield static or an emblem's, controlled by that player
// (Caster$ You; the activator of a loyalty ability is the permanent's
// controller), whose Filter matches sl read with the static's controller as
// "you" and its source as Self.
func (s *State) loyaltyFlash(k *prep, sl Slot) bool {
	if DebugLoyaltyFlashOff || k.gen == nil || !k.gen.uses.has(useLoyaltyFlash) {
		return false
	}
	ctrl := s.perms[sl].ctrl
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatLoyaltyFlash && st.ctrl == ctrl && s.statCond(k, st) &&
			(st.emb == 0 || s.embCaptureOK(k, st.emb, sl)) &&
			s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}
