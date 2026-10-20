package host

import "testing"

// TestOnlyAWaitingHumanSubmitIsReparked pins the review finding that a refused
// answer must be re-parked only when a registry submit is waiting for the
// verdict. A caretaker or plain Decide answer has a parking but no ack
// channel; re-parking it would make the same timed-out caretaker answer the
// same refused decision forever, spinning the match and hanging Close.
func TestOnlyAWaitingHumanSubmitIsReparked(t *testing.T) {
	plain := &parkedDecision{hs: &parking{}}
	if plain.waitingHumanSubmit() {
		t.Fatal("a caretaker/plain answer with no waiting submit was treated as re-parkable")
	}
	waiting := &parkedDecision{hs: &parking{ack: make(chan error, 1)}}
	if !waiting.waitingHumanSubmit() {
		t.Fatal("a registry submit waiting for its verdict was not treated as re-parkable")
	}
	bot := &parkedDecision{}
	if bot.waitingHumanSubmit() {
		t.Fatal("a bot parking was treated as a waiting human submit")
	}
}
