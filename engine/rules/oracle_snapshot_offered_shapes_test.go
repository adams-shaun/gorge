package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleSnapshotOffersLandFundedAuraAndTopLibraryCast pins the two shapes
// xproof cycle 11251737 still found diverging against both Forge and XMage at
// a kit with the untapped-source bound (xs_XP_honest-work_static0.0_s2,
// xs_XP_hakoda-selfless-commander_static0.1_s2), both first different at
// `setup`, field `offered`:
//
//   - an Aura spell whose cast needs an Enchant target (CR 303.4a, 601.2c):
//     Honest Work ({U}, Enchant creature an opponent controls) in p0's hand
//     beside an untapped Island, with p1's Wall of Stone a legal target, is a
//     legal play (CR 601.2g-h: the Island funds the {U} during the cast);
//   - a spell castable from a non-hand zone through a MayPlay static
//     (CR 601.2a, 601.3): Hakoda, Selfless Commander ("You may cast Ally
//     spells from the top of your library") with Expedition Envoy ({W} Ally)
//     on top of p0's library, funded by the untapped Plains.
func TestOracleSnapshotOffersLandFundedAuraAndTopLibraryCast(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	// The real find's setup: turn 1, a full library of Islands under Honest
	// Work, Wall of Stone on p1's battlefield.
	const auraSc = `{"name":"xproof start Honest Work static#0.0 seed 2","cr":["601.2g","303.4a"],"why":"the setup offered set is the CR-legal one","setup":{"p0":{"battlefield":["Island"],"hand":["Honest Work"],"library_top":["Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island","Island"],"life":20},"p1":{"battlefield":["Wall of Stone"],"library_top":["Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain","Mountain"],"life":20}},"steps":[]}`
	snap := setupOfferedSnap(t, reg, auraSc)
	if !slices.ContainsFunc(snap.Players[0].Hand, func(n string) bool { return n == "Honest Work" }) {
		t.Fatalf("precondition: Honest Work in p0's hand: %+v", snap.Players[0].Hand)
	}
	if !slices.ContainsFunc(snap.Offered, func(o OracleSnapOffered) bool {
		return o.Source == "p0:Island" && o.Kind == "activate"
	}) {
		t.Fatalf("precondition: p0:Island mana activation expected in offered = %+v", snap.Offered)
	}
	if !slices.ContainsFunc(snap.Offered, func(o OracleSnapOffered) bool {
		return o.Source == "p0:Honest Work" && o.Kind == "cast"
	}) {
		t.Fatalf("setup offered = %+v, want p0:Honest Work|cast (CR 601.2g funds the {U}; CR 303.4a: p1's Wall of Stone is an Enchant target)", snap.Offered)
	}

	// The real find's setup: Hakoda beside an untapped Plains, Expedition
	// Envoy on top of p0's library.
	const topSc = `{"name":"xproof start Hakoda, Selfless Commander static#0.1 seed 2","cr":["601.2a","601.3","702.31"],"why":"the setup offered set is the CR-legal one","setup":{"p0":{"battlefield":["Hakoda, Selfless Commander","Plains"],"library_top":["Expedition Envoy","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains"],"life":20},"p1":{"library_top":["Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains"],"life":20}},"steps":[]}`
	snap = setupOfferedSnap(t, reg, topSc)
	if !slices.ContainsFunc(snap.Offered, func(o OracleSnapOffered) bool {
		return o.Source == "p0:Expedition Envoy" && o.Kind == "cast"
	}) {
		t.Fatalf("setup offered = %+v, want p0:Expedition Envoy|cast (Hakoda's MayPlay lets the top-of-library Ally be cast; CR 601.2g funds the {W} with the untapped Plains)", snap.Offered)
	}
	if !slices.ContainsFunc(snap.Offered, func(o OracleSnapOffered) bool {
		return o.Source == "p0:Plains" && o.Kind == "activate"
	}) {
		t.Fatalf("precondition: p0:Plains mana activation expected in offered = %+v", snap.Offered)
	}
}
