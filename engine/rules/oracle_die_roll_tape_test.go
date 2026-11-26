package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestOracleDieRollTapeDrawFallsBackAfterTapeExhausted pins the tape's
// exhausted behaviour. Engine.Rand serves every random draw, not only dice, so
// after a step's recorded rolls are spent a later draw is an ordinary seeded
// pick and must fall through silently, exactly like the old empty-queue early
// return. Otherwise a card that rolls a die and then makes a random choice
// (Maddening Hex's random opponent) turns a passing game into a harness
// failure.
func TestOracleDieRollTapeDrawFallsBackAfterTapeExhausted(t *testing.T) {
	log := &events.Log{}
	tape := oracleDieRollTape{
		step:  1,
		queue: []oracleDieRoll{{Step: 1, Seat: 0, Sides: 6, Natural: 4}},
		log:   &log,
	}

	// Precondition: the taped roll is served at the log length the roll is
	// recorded at (its DieRollNote has not been appended yet).
	if got, ok := tape.draw(6); !ok || got != 3 {
		t.Fatalf("first draw = (%d, %v), want the taped natural 4 as (3, true)", got, ok)
	}
	if len(tape.fails) != 0 {
		t.Fatalf("first draw failed: %v", tape.fails)
	}

	// A re-execution restores the log to the length the roll was consumed at,
	// so the same record must be served again.
	if got, ok := tape.draw(6); !ok || got != 3 {
		t.Fatalf("re-executed draw = (%d, %v), want the taped natural 4 as (3, true)", got, ok)
	}

	// The roll emitted its note; the step now makes an ordinary random pick at
	// a longer log. That is past the tape: the seeded generator must answer,
	// with no harness failure.
	log.Events = append(log.Events, events.Event{Seq: 1})
	if got, ok := tape.draw(2); ok || got != 0 {
		t.Fatalf("post-tape draw = (%d, %v), want the seeded fallback (0, false)", got, ok)
	}
	if len(tape.fails) != 0 {
		t.Fatalf("post-tape draw reported a harness failure: %v", tape.fails)
	}
}
