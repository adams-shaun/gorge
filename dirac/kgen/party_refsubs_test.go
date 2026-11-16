package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRefSubsKeepsPartyRoles pins that a Count$Party cost static names the
// four party roles (CR 700.8) in its reference set. The kernel finds them by
// name (partyRoles; value_party.go), and the matchup builder keeps
// only the subtypes CardRefSubs names: a matchup whose only reader is Sea
// Gate Colossus's own static dropped Cleric, Rogue, Warrior and Wizard, the
// kernel's partySize counted every role outside the table and priced the
// cast {7} where gorge priced it {6} — the cast was never offered.
func TestRefSubsKeepsPartyRoles(t *testing.T) {
	pt := &poolTables{
		subIdx: map[string]int{"Cleric": 1, "Rogue": 2, "Warrior": 3, "Wizard": 4},
		exprs:  []kop.Expr{{}, {Kind: kop.ExParty}},
		conds:  []kop.Cond{{}}, // CondID 0 is always present
	}
	d := &kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatReduceCost, VA: kop.ValueRef{K: kop.VExpr, N: 1}}}}}
	if got := refSubs(pt, d, nil); !slices.Equal(got, []uint8{1, 2, 3, 4}) {
		t.Fatalf("refSubs = %v, want [1 2 3 4] (the four party roles)", got)
	}
}

// TestRefSubsPartyRolesAbsentNamesNone: a pool with no party role at all
// names none — every party member would be a changeling, which the printed
// AllCreatureTypes already covers.
func TestRefSubsPartyRolesAbsentNamesNone(t *testing.T) {
	pt := &poolTables{exprs: []kop.Expr{{}, {Kind: kop.ExParty}}, conds: []kop.Cond{{}}}
	d := &kop.CardDef{Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatReduceCost, VA: kop.ValueRef{K: kop.VExpr, N: 1}}}}}
	if got := refSubs(pt, d, nil); len(got) != 0 {
		t.Fatalf("refSubs = %v, want none (the pool has no party role)", got)
	}
}
