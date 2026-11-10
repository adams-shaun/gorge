package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestShareLandTypeKeepsBasicLandTypes: a search flagged SrShareLandType
// compares the found cards' printed land types in the kernel
// (zone_sharelandtype.go), and the matchup keeps only the subtypes a card's
// refSubs names, so the flag must name the five basic land types or the second
// pick finds no shared type and the search ends after one card (Myriad
// Landscape; CR 701.23, 205.3i).
func TestShareLandTypeKeepsBasicLandTypes(t *testing.T) {
	pt := &poolTables{
		filters: []kop.Filter{{}},
		exprs:   []kop.Expr{{}},
		conds:   []kop.Cond{{}},
		subIdx:  map[string]int{"Saga": 2, "Plains": 5, "Island": 7, "Swamp": 11, "Mountain": 13, "Forest": 17},
	}
	card := func(b kop.ValueRef) *kop.CardDef {
		return &kop.CardDef{Abilities: []kop.Ability{{Prog: []kop.Instr{{Op: kop.OpSearchEx, B: b}}}}}
	}
	flagged := refSubs(pt, card(kop.Const(kop.SrShareLandType)), nil)
	plain := refSubs(pt, card(kop.Const(0)), nil)
	if slices.Equal(flagged, plain) {
		t.Fatalf("precondition: flagged and unflagged searches both give %v", flagged)
	}
	if want := []uint8{5, 7, 11, 13, 17}; !slices.Equal(flagged, want) {
		t.Fatalf("flagged refSubs = %v, want %v (the five basic land types, not Saga)", flagged, want)
	}
	if len(plain) != 0 {
		t.Fatalf("unflagged refSubs = %v, want none", plain)
	}
}
