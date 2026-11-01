package draftlab

import (
	"strings"
	"testing"
)

// TestReplayTapeVerifyingLabMustMatchRecordedCaps pins the watchdog-cap
// coupling ReplayTape's doc states: the re-drive runs under the VERIFYING
// lab's own MaxTurns/MaxIntents, never caps carried by the tape, so a lab
// built with larger caps than the recording plays past the recorded end and
// fails "tape exhausted", while a lab built with the recorded caps replays
// to the identical head.
func TestReplayTapeVerifyingLabMustMatchRecordedCaps(t *testing.T) {
	rec := testLabOpts(t, Options{RecordTape: true, MaxTurns: 3})
	es := testEntrants(t, rec, 2)
	res, err := rec.PlayGame(es[0], es[1], 5)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if res.Stall != "turns" {
		t.Fatalf("stall %q, want turns (MaxTurns 3)", res.Stall)
	}
	if res.Tape == nil || len(res.Tape.Intents) == 0 {
		t.Fatal("the stalled game recorded no tape; the mismatch assertions would be vacuous")
	}
	if got := rec.maxTurns(); got != 3 {
		t.Fatalf("recording lab effective MaxTurns %d, want 3", got)
	}

	// A default-caps lab: its effective MaxTurns (150) is larger than the
	// recording's 3, so the re-drive plays past the recorded stall point
	// and runs out of tape.
	loose := testLabOpts(t, Options{})
	if loose.maxTurns() == rec.maxTurns() {
		t.Fatalf("the two labs' effective MaxTurns do not differ (%d), so the mismatch case would be vacuous", loose.maxTurns())
	}
	_, err = loose.ReplayTape(es[0].Deck, es[1].Deck, 5, res.Tape)
	if err == nil || !strings.Contains(err.Error(), "tape exhausted") {
		t.Errorf("a larger-caps lab re-driving a stalled tape: error %v, want \"tape exhausted\"", err)
	}

	// A lab built with the recorded caps reproduces the recorded head.
	same := testLabOpts(t, Options{MaxTurns: 3})
	head, err := same.ReplayTape(es[0].Deck, es[1].Deck, 5, res.Tape)
	if err != nil {
		t.Fatalf("ReplayTape with matching caps: %v", err)
	}
	if head != res.Head {
		t.Fatalf("matching-caps replay head %s, recorded %s", head, res.Head)
	}
}
