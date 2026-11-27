package kgen

import "testing"

// TestLIRRefNames pins the two display names that were swapped in the
// refNames table (kgen-lir-refnames-swapped): rTriggeredOwner is the owner
// of the triggering object (kop.RefTriggerObjOwner, lir.go's constant
// comment), and rChosenPlayer is the opponent the source permanent chose as
// it entered (kop.RefChosenPlayer). The lowering and kernel resolution map
// the kinds to the right kop refs; only lRef.String()'s naming was wrong, so
// an -lir dump read the two refs as each other.
func TestLIRRefNames(t *testing.T) {
	if got := (lRef{Kind: rTriggeredOwner}).String(); got != "TriggeredOwner" {
		t.Errorf("rTriggeredOwner printed %q, want %q", got, "TriggeredOwner")
	}
	if got := (lRef{Kind: rChosenPlayer}).String(); got != "ChosenPlayer" {
		t.Errorf("rChosenPlayer printed %q, want %q", got, "ChosenPlayer")
	}
}
