package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTypesallCMCBound: a constant cmcLE/cmcLT beside an all-of type
// conjunction lowers to one combined predicate (PredTypesAllCMCLE); GE, EQ
// and a value bound beside the conjunction still refuse (one Pred slot).
func TestTypesallCMCBound(t *testing.T) {
	const types = tArtifact | tCreature
	const predN = int16(kop.TArtifact | kop.TCreature)
	base := func() lClause {
		return lClause{Types: types, TypesAll: types, CmpStat: cmpCMC}
	}
	for _, tc := range []struct {
		name string
		op   uint8
		n    int
		want int
	}{
		{"cmcLE3", opLE, 3, 3},
		{"cmcLT4", opLT, 4, 3},
	} {
		c := base()
		c.CmpOp, c.CmpN = tc.op, tc.n
		kf, err := (&cardBuilder{}).clause(c, kop.ZoneGraveyard)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if kf.Pred != kop.PredTypesAllCMCLE || kf.PredN != predN || kf.PredV != kop.Const(tc.want) ||
			kf.Types != kop.TArtifact|kop.TCreature {
			t.Errorf("%s emitted %+v, want PredTypesAllCMCLE PredN=%#x PredV=Const(%d) Types=%#x",
				tc.name, kf, predN, tc.want, types)
		}
	}
	for _, tc := range []struct {
		name string
		op   uint8
		n    int
	}{
		{"cmcGE2", opGE, 2},
		{"cmcEQ2", opEQ, 2},
	} {
		c := base()
		c.CmpOp, c.CmpN = tc.op, tc.n
		if _, err := (&cardBuilder{}).clause(c, kop.ZoneGraveyard); err == nil || !strings.Contains(err.Error(), "one Pred slot") {
			t.Errorf("%s: %v; want the one-Pred-slot refusal", tc.name, err)
		}
	}
	// a value bound beside the conjunction (Vision Quest's cmcLEX) still refuses
	c := base()
	c.CmpOp, c.CmpV = opLE, &lValue{Kind: vConst, N: 3}
	if _, err := (&cardBuilder{}).clause(c, kop.ZoneGraveyard); err == nil || !strings.Contains(err.Error(), "one Pred slot") {
		t.Errorf("cmcLEX: %v; want the one-Pred-slot refusal", err)
	}

	// the carrier: Resistance Is Futile lowers and builds with no refusal
	seen := 0
	for _, b := range lowerPoolBuilt(t, []string{"Resistance Is Futile"}) {
		if b.name != "Resistance Is Futile" {
			continue
		}
		seen++
		for _, r := range b.refusals {
			t.Errorf("Resistance Is Futile: refused: %s", r.Reason)
		}
	}
	if seen != 1 {
		t.Fatalf("built %d Resistance Is Futile cards, want 1", seen)
	}
}
