package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiscardBesideSacFiltered: Skoa, Embermage's activated cost, a filtered
// discard beside a counted sacrifice, builds with the sacrifice on Cost.N /
// Cost.Filter and the discard on DiscardSideN / DiscardSideF (CostDiscardFiltered
// clear); the same shape outside an activated ability, and the exile / forage
// beside another part (Tawnos, Solemn Survivor), stay refused.
func TestDiscardBesideSacFiltered(t *testing.T) {
	lc := costPairCtx(true)
	c, err := lc.costHand("Discard<1/Card.namedSkoa, Embermage> Sac<2/Mountain>")
	if err != nil {
		t.Fatalf("Skoa's cost refused: %v", err)
	}
	if c.DiscardF == nil || c.Sac == nil || c.SacN != 2 || c.DiscardN != 1 {
		t.Fatalf("precondition: the lowered cost is not a filtered discard beside Sac<2>: %+v", c)
	}
	if !discardBesideSacFiltered(c) {
		t.Fatal("discardBesideSacFiltered rejects Skoa's cost")
	}
	cb := &cardBuilder{pt: &poolTables{subIdx: map[string]int{"Mountain": 1}, missing: map[string]bool{}, filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	out, err := cb.cost(c)
	if err != nil {
		t.Fatalf("Skoa's cost did not build: %v", err)
	}
	if out.Flags&(kop.CostSac|kop.CostDiscard) != kop.CostSac|kop.CostDiscard || out.Flags&kop.CostDiscardFiltered != 0 {
		t.Errorf("flags %b: want CostSac|CostDiscard without CostDiscardFiltered", out.Flags)
	}
	if out.N != 2 || out.DiscardSideN != 1 || out.DiscardSideF == 0 || out.Filter == out.DiscardSideF {
		t.Errorf("N %d DiscardSideN %d Filter %d DiscardSideF %d: want the sacrifice's 2 and its own filter, the discard's 1 and its own", out.N, out.DiscardSideN, out.Filter, out.DiscardSideF)
	}

	for _, tc := range []struct {
		name string
		lc   *lowerCtx
		cost string
	}{
		{"Tawnos-shaped exile beside a sacrifice", costPairCtx(true), "Sac<2/Artifact.token> ExileFromGrave<1/Artifact;Creature>"},
		{"forage beside a sacrifice", costPairCtx(true), "Sac<1/Creature> Forage"},
		{"discard beside a sacrifice, not an activated cost", costPairCtx(false), "Discard<1/Card.namedSkoa, Embermage> Sac<2/Mountain>"},
	} {
		if _, err := tc.lc.costHand(tc.cost); err == nil || !strings.Contains(err.Error(), "beside another counted part") {
			t.Errorf("%s: err %v, want the counted-part refusal", tc.name, err)
		}
	}
}

// costPairCtx is a lowering context for parsing a bare Cost$ string.
func costPairCtx(act bool) *lowerCtx {
	return &lowerCtx{p: &pool{typeWords: map[string]bool{}}, known: func(string) bool { return true }, actCost: act}
}
