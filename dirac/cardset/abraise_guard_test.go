package cardset

import (
	"strings"
	"testing"
)

// TestAbRaiseRefusalTwoPickers: two cards whose raise carries a sacrifice pick
// are refused (the build poses one Cost with one filter), where one picker
// beside nothing else is not.
func TestAbRaiseRefusalTwoPickers(t *testing.T) {
	if err := AbRaiseRefusal([]string{"A"}, nil, nil, []string{"A"}, nil); err != nil {
		t.Fatalf("one picker refused: %v", err)
	}
	if err := AbRaiseRefusal([]string{"A", "B"}, nil, nil, []string{"A", "B"}, nil); err == nil {
		t.Fatal("two pick raisers were not refused")
	}
}

// TestAbRaiseRefusalTappedAbility: a generic raise beside an activated ability
// that taps its source is refused, naming both.
func TestAbRaiseRefusalTappedAbility(t *testing.T) {
	err := AbRaiseRefusal([]string{"A"}, nil, nil, nil, []string{"B"})
	if err == nil {
		t.Fatal("a generic raise beside a {T} ability was not refused")
	}
	if got := err.Error(); !strings.Contains(got, "A raises") || !strings.Contains(got, "B has") {
		t.Fatalf("refusal names neither card: %v", err)
	}
}
