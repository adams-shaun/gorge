package kgen

import "testing"

// A ward of discarding a card or paying {N} is refused by name, with its
// reason, not lowered as a plain discard ward.
func TestWardDiscardOrPayRefused(t *testing.T) {
	const titania = "Titania, Rugged Rumbler"
	kp := kfhBuild(t, []string{titania}, nil)
	kp.refusedWith(t, titania, "ward discard-or-pay cost")
}
