package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The loyalty-timing grant: a Mode$ CastWithFlash static whose ValidSA$ is
// Activated.Loyalty ("you may activate loyalty abilities of <ValidCard$> any
// time you could cast an instant"; gorge rules/loyalty_flash.go
// loyaltyAtInstantSpeed). It is a timing grant for activations, never a
// spell-flash grant, so it lowers to its own kind (kop.StatLoyaltyFlash; kernel
// static_loyaltyflash.go), printed (The Wandering Emperor, Teferi, Master of
// Time) or Effect-delivered (lower_effectemblem.go: Teferi, Temporal
// Archmage's emblem, Jace's Machinations).
func init() {
	registerStaticBuild("LoyaltyFlash", cbLoyaltyFlashStatic)
}

// loyaltyFlash lowers modeCastWithFlash's Activated.Loyalty arm. Caster$ You
// is the one caster both routes read (gorge actorMatches on a printed static,
// effects.LoyaltyFlashParamsReadable on an Effect's); any other caster, an
// EffectZone$ (the grant is read from the battlefield or an emblem only) or a
// missing ValidCard$ refuses. A ValidCard$ word the filter lowerer cannot
// read refuses through readF.
func (sc *staticCtx) loyaltyFlash() ([]lStatic, bool, error) {
	sp := sc.sp
	if c := sp.str("Caster"); c != "You" {
		return nil, true, refuse("CastWithFlash Activated.Loyalty Caster$ %q", c)
	}
	if ez, ok := sp.get("EffectZone"); ok {
		return nil, true, refuse("CastWithFlash Activated.Loyalty EffectZone$ %q", ez)
	}
	f, err := sc.readF("ValidCard")
	if err != nil {
		return nil, true, err
	}
	if f == nil {
		return nil, true, refuse("CastWithFlash Activated.Loyalty without ValidCard$")
	}
	sc.st.Kind = "LoyaltyFlash"
	sc.st.F = f
	return nil, false, nil
}

// cbLoyaltyFlashStatic builds the kernel static: the planeswalkers whose
// loyalty abilities the grant times, read on the battlefield.
func cbLoyaltyFlashStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	return staticFilterArm(kop.StatLoyaltyFlash)(cb, s, st)
}
