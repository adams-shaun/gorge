package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// A stated-quality search may fail to find (701.23b); a quantity-only
// search may not (701.23d). The Evolving Wilds control cannot catch this.
func TestCR701QuantityOnlyTutorMustFindAvailableCard(t *testing.T) {
	requireCR601Audit(t, "CR 701.23d: quantity-only search permits failing to find")
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Vampiric Tutor")
	_, d := castSearchSpell(t, e, "Vampiric Tutor")
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" || len(d.Options) == 0 {
		t.Fatalf("CR 701.23d: real Vampiric Tutor must reach a nonempty search, got %+v", d)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("CR 701.23d: quantity-only search with %d available cards offers %d..%d; must find one, not fail to find", len(d.Options), d.Min, d.Max)
	}
}
