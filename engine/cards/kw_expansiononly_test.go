package cards

import "testing"

// TestKeywordExpansionOnly pins the answer on one head of each kind: a head
// whose rules meaning is entirely its expansion lines (Soulshift) and a head
// the rules read by name (Flanking, trigger_blocks.go). Both preconditions --
// that either head is a registered expander at all -- are asserted, so the
// comparison is against a registry that actually holds both heads.
func TestKeywordExpansionOnly(t *testing.T) {
	if !KeywordExpandable("Soulshift") {
		t.Fatal("Soulshift is not a registered expander; test premise gone")
	}
	if !KeywordExpandable("Flanking") {
		t.Fatal("Flanking is not a registered expander; test premise gone")
	}
	if !KeywordExpansionOnly("Soulshift") {
		t.Error("KeywordExpansionOnly(Soulshift) = false, want true")
	}
	if KeywordExpansionOnly("Flanking") {
		t.Error("KeywordExpansionOnly(Flanking) = true, want false")
	}
}
