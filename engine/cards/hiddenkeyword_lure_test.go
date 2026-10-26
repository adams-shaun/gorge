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
	for _, k := range []string{"All creatures with flying able to block CARDNAME do so.", "MustBeBlockedByAll:Wall:All Walls able to block CARDNAME do so."} {
		if got := HiddenKeywordOf(k); got != HiddenKeywordNone {
			t.Errorf("HiddenKeywordOf(%q) = %d, want None", k, got)
		}
	}
}
