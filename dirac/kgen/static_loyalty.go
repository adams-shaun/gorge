package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The bodies of the loyalty statics (static_mode.go).
func init() {
	staticMode("NumLoyaltyAct", (*staticCtx).modeNumLoyaltyAct)
}

// modeNumLoyaltyAct lowers a printed S:Mode$ NumLoyaltyAct static: the
// ValidCard$ selector is the permanent whose loyalty-ability limit the static
// raises, and Twice$ True is the one grant the kernel reads (gorge
// rules/legal_loyalty.go loyaltyAbilityLimit: any matching Twice$ raises the
// base 1 to 2, never compounds). Additional$ is gorge's other parameter but
// no printed carrier in the corpus uses it here (the two Effect-delivered
// cards are a different route), so it stays refused rather than silently
// ignored. The kernel reads the static live: dirac/static_loyaltyact.go.
func (sc *staticCtx) modeNumLoyaltyAct() ([]lStatic, bool, error) {
	sp := sc.sp
	if v, ok := sp.get("Twice"); !ok || !isTrue(v) {
		return nil, true, refuse("NumLoyaltyAct without Twice$ True (%q)", v)
	}
	if v, ok := sp.get("Additional"); ok {
		return nil, true, refuse("NumLoyaltyAct Additional$ %q", v)
	}
	if v, ok := sp.get("OnlySourceAbs"); ok {
		return nil, true, refuse("NumLoyaltyAct OnlySourceAbs$ %q on a printed static", v)
	}
	f, err := sc.readF("ValidCard")
	if err != nil {
		return nil, true, err
	}
	if f == nil {
		return nil, true, refuse("NumLoyaltyAct without ValidCard$")
	}
	sc.st.F = f
	return nil, false, nil
}

// isTrue is params.is with the case and surrounding spaces Forge scripts
// occasionally spell "true" / " True " with (isLoyaltyMarked reads the
// Planeswalker$ marker the same way).
func isTrue(v string) bool { return strings.EqualFold(strings.TrimSpace(v), "True") }

// numLoyaltyActStatic builds the kernel static the mode body lowered: the
// ValidCard$ filter as the matched permanent and the shared Condition gate.
// It lives outside (*cardBuilder).static because that function is at its
// codeshape ceiling.
func (cb *cardBuilder) numLoyaltyActStatic(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	st.Kind = kop.StatNumLoyaltyAct // static_loyaltyact.go
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	if st.Filter, err = cb.filter(s.F, kop.ZoneBattlefield); err != nil {
		return ka, nil, err
	}
	return ka, nil, nil
}
