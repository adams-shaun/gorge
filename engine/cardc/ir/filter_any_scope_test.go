package ir

import "testing"

// TestValidateFilterAnyAlternativesOwnScope: an FAny alternative with its own
// Subject or Zones is a root of its own; an FAll part is not.
func TestValidateFilterAnyAlternativesOwnScope(t *testing.T) {
	perm := &Filter{Subject: SubjPermanent, Zones: MakeZoneSet(ZoneBattlefield), Op: FPred, Pred: PCardType, Word: "Creature"}
	player := &Filter{Subject: SubjPlayer, Op: FPred, Pred: PAny}
	anyTarget := &Filter{Subject: SubjObject, Op: FAny, Args: []*Filter{perm, player}}
	if err := ValidateFilter(anyTarget); err != nil {
		t.Errorf("any target: ValidateFilter = %v, want nil", err)
	}
	// Inside the alternative the alternative's scope holds.
	bad := &Filter{Subject: SubjObject, Op: FAny, Args: []*Filter{
		{Subject: SubjPlayer, Op: FAll, Args: []*Filter{{Subject: SubjPermanent, Op: FPred, Pred: PToken}}},
	}}
	if err := ValidateFilter(bad); err == nil {
		t.Error("a part whose subject differs from its alternative's must not validate")
	}
	all := &Filter{Subject: SubjObject, Op: FAll, Args: []*Filter{perm, player}}
	if err := ValidateFilter(all); err == nil {
		t.Error("FAll parts share the root's scope; mixed subjects must not validate")
	}
}
