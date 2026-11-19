package events

import "testing"

func TestReverseTurnOrderKindString(t *testing.T) {
	if got, want := ReverseTurnOrder.String(), "reverse_turn_order"; got != want {
		t.Fatalf("ReverseTurnOrder.String() = %q, want %q", got, want)
	}
}
