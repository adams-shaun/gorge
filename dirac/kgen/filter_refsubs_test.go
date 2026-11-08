package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterRefSubsWalksModesPickV pins that a card-level dynamic charm count
// contributes the subtypes its filter names to the card's reference set. The
// matchup builder keeps only those subtypes (pooldata.go subSet), so a count
// filter it never sees is compacted to NoSubtype and counts zero (Bumi, King
// of Three Trials; CR 603.3c).
func TestFilterRefSubsWalksModesPickV(t *testing.T) {
	pt := &poolTables{
		filters: []kop.Filter{{}, {Sub: 2}}, // subtype id 1, as "Lesson"
		exprs:   []kop.Expr{{}, {Kind: kop.ExCountGraveyard, F: 1}},
	}
	d := &kop.CardDef{ModesPickV: kop.ValueRef{K: kop.VExpr, N: 1}}
	if got := refSubs(pt, d, nil); !slices.Equal(got, []uint8{1}) {
		t.Fatalf("refSubs = %v, want [1] (the count filter's subtype)", got)
	}
}

// TestFilterRefSubsKeepsHistoricSaga: PredHistoric's PredN is the Saga
// subtype index+1 (CR 700.6), and the kernel reads it by index. The matchup
// keeps only the subtypes CardRefSubs names, so a historic filter must name
// its Saga or the matchup remap zeroes PredN and the Saga stops being
// historic.
func TestFilterRefSubsKeepsHistoricSaga(t *testing.T) {
	pt := &poolTables{
		filters: []kop.Filter{{}, {Pred: kop.PredHistoric, PredN: 49}}, // Saga = pool subtype 48
		exprs:   []kop.Expr{{}, {Kind: kop.ExCountGraveyard, F: 1}},
	}
	d := &kop.CardDef{ModesPickV: kop.ValueRef{K: kop.VExpr, N: 1}}
	if got := refSubs(pt, d, nil); !slices.Equal(got, []uint8{48}) {
		t.Fatalf("refSubs = %v, want [48] (the Saga subtype PredN names)", got)
	}
}
