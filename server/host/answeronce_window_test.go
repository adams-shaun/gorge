package host

import "testing"

// TestAnswerOnceWaitsPastTheStaleAskWindow pins answerOnce's Seq guard. A
// submit now claims the seat's slot before its send, so a first-200 Pending
// poll cannot re-serve the just-answered decision; the guard is retained
// because the poll still races the submit, and the helper must never return
// the answered decision as if it were the next ask.
func TestAnswerOnceWaitsPastTheStaleAskWindow(t *testing.T) {
	t.Parallel()
	r, id := startHumanTable(t)
	d1 := waitPending(t, r, id, 1, 0) // pre-answer: waitPending is fine here
	if err := r.SubmitIntent(id, 1, 0, legalIntent(d1)); err != nil {
		t.Fatalf("SubmitIntent: %v", err)
	}
	// Called with no delay after the submit, answerOnce's first Pending
	// lands inside the stale window (the parked await has not yet woken to
	// clear its slot).
	d2 := answerOnce(t, r, id, d1.Seq)
	if d2.Seq <= d1.Seq {
		t.Fatalf("answerOnce returned seq %d after answering seq %d: it re-served the stale ask", d2.Seq, d1.Seq)
	}
}
