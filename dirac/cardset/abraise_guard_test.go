package cardset

import "testing"

// TestAbRaiseRefusalTwoPickers: two cards whose raise carries a sacrifice pick
// are refused (the build poses one Cost with one filter), where one picker
// beside nothing else is not.
func TestAbRaiseRefusalTwoPickers(t *testing.T) {
	if err := AbRaiseRefusal([]string{"A"}, nil, nil, []string{"A"}); err != nil {
		t.Fatalf("one picker refused: %v", err)
	}
	if err := AbRaiseRefusal([]string{"A", "B"}, nil, nil, []string{"A", "B"}); err == nil {
		t.Fatal("two pick raisers were not refused")
	}
}
