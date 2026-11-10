package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPTCompareEqGt: a battlefield power == a constant (Amalia Benavides
// Aguirre's powerEQ20) lowers to PredPowerGEV with PredN 3, and a battlefield
// toughness > a value (Hulkling, Burgeoning Bruiser's toughnessGTZ) to
// PredToughnessLEV with PredN 3 and the bound raised by one; the toughness EQ
// a value (out of scope) still refuses. Both cards lower with no refusal.
func TestPTCompareEqGt(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	kf, err := cb.clause(lClause{CmpStat: cmpPower, CmpOp: opEQ, CmpN: 20}, kop.ZoneBattlefield)
	if err != nil || kf.Pred != kop.PredPowerGEV || kf.PredN != 3 || kf.PredV != kop.Const(20) {
		t.Errorf("powerEQ20: got %+v, %v; want PredPowerGEV PredN 3 PredV 20", kf, err)
	}
	if _, err := cb.clause(lClause{CmpStat: cmpPower, CmpOp: opEQ, CmpN: 20}, kop.ZoneGraveyard); err == nil {
		t.Errorf("powerEQ20 off the battlefield: want a refusal")
	}
	y := &lValue{Kind: vX}
	gt, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opGT, CmpX: "Z", CmpV: y}, kop.ZoneBattlefield)
	ge, _ := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opLE, CmpX: "Z", CmpV: y}, kop.ZoneBattlefield)
	if err != nil || gt.Pred != kop.PredToughnessLEV || gt.PredN != 3 || gt.PredV == ge.PredV {
		t.Errorf("toughnessGTZ: got %+v, %v; want PredToughnessLEV PredN 3 with the bound raised by one", gt, err)
	}
	if none, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opGT, CmpX: "Z", CmpV: y}, kop.ZoneNone); err != nil || none != gt {
		t.Errorf("toughnessGTZ with no zone (Hulkling's RefCount over the triggered creature): got %+v, %v; want %+v", none, err, gt)
	}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opGT, CmpX: "Z", CmpV: y}, kop.ZoneGraveyard); err == nil {
		t.Errorf("toughnessGTZ off the battlefield: want a refusal")
	}
	if _, err := cb.clause(lClause{CmpStat: cmpToughness, CmpOp: opEQ, CmpX: "Z", CmpV: y}, kop.ZoneBattlefield); err == nil {
		t.Errorf("toughnessEQZ: want a refusal")
	}
	got := lowerNamed(t, []string{"Amalia Benavides Aguirre", "Hulkling, Burgeoning Bruiser"})
	if len(got) != 2 {
		t.Fatalf("lowered %d cards, want 2", len(got))
	}
	for _, c := range got {
		for _, r := range lowerRefusals(c) {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
}
