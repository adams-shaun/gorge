package cards

import "testing"

func TestLureSentenceCanonicalisesToAllMustBlock(t *testing.T) {
	t.Parallel()
	const sentence = "All creatures able to block CARDNAME do so."
	if got := CanonicalKeywordLine(sentence); got != "AllMustBlock" {
		t.Fatalf("CanonicalKeywordLine = %q, want AllMustBlock", got)
	}
	for _, k := range []string{"AllMustBlock", "HIDDEN " + sentence, sentence} {
		if got := HiddenKeywordOf(k); got != HiddenKeywordAllMustBlock {
			t.Errorf("HiddenKeywordOf(%q) = %d, want AllMustBlock", k, got)
		}
	}
	if got := HiddenKeywordOf("CARDNAME must be blocked if able."); got != HiddenKeywordMustBlock {
		t.Errorf("must-be-blocked sentence = %d, want MustBlock", got)
	}
	// The filtered printed form is its own code (the filter rides the line),
	// never the unfiltered Lure; the bare filtered sentence stays unknown.
	if got := HiddenKeywordOf("MustBeBlockedByAll:Creature.Wall:All Walls able to block CARDNAME do so."); got != HiddenKeywordMustBeBlockedByAll {
		t.Errorf("filtered lure = %d, want MustBeBlockedByAll", got)
	}
	if got := HiddenKeywordOf("All creatures with flying able to block CARDNAME do so."); got != HiddenKeywordNone {
		t.Errorf("filtered sentence = %d, want None", got)
	}
}
