package event

import "testing"

// TestAllReadyFalseBeforeCreation pins the n == 0 guard: a fresh event (no
// EventCreated yet) is not all-ready, so a zero-seat event can never open
// rounds or end as a no-games event.
func TestAllReadyFalseBeforeCreation(t *testing.T) {
	e := New()
	if e.allReady() {
		t.Fatalf("allReady is true on a fresh event with 0 seats, want false")
	}
}
