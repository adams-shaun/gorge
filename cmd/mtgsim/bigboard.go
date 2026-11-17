package main

import (
	"fmt"
	"io"

	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
)

// reportBigBoard prints playOne's line for a game the object cap (or the
// runaway-on-a-large-board trip) ended: degenerate VOLUME -- a token
// doubler such as Scute Swarm copying itself on every landfall -- that
// rules.IsVolumeReason names, recorded as a bigboard stall the way
// testkit/bench records it, never as a LIVELOCK failure. The line carries no
// "LIVELOCK" marker, so the hunt's failure grep passes over it.
//
// A stalled game has no end to compare against, but its recorded prefix is
// still a deterministic chain: under -verify the intents the engine
// finished before the aborted Submit are replayed (the aborted one's
// events are not, it never reached a decision) and every replayed event is
// compared against the live log, so a state write outside events.Apply on
// the way to the stall still fails the run. Returns true: the stall is not a
// failure; a prefix that does not replay is.
func reportBigBoard(out io.Writer, seed uint64, seats, n int, lle *rules.LivelockError, e *rules.Engine, cfg rules.Config, verify bool) bool {
	fmt.Fprintf(out, "seed %d: %d seats, %6d intents, %6d events, %3d turns -- BIGBOARD STALL (volume, not a loop): %s, %d object(s), cap %d\n",
		seed, seats, n, len(e.L.Events), e.G.Turn, lle.Reason, lle.Count, lle.Cap)
	if !verify {
		return true
	}
	done := len(e.L.Intents) - 1 // the last intent is the Submit the cap aborted
	if done < 0 {
		done = 0
	}
	head, err := replayPrefix(e, cfg, done)
	return printReplayOutcome(out, err, head)
}

// replayPrefix replays the first n recorded intents of e's log and returns
// the replayed chain head. A panic out of the replay (the cap tripping
// again would be one) is an error: the prefix ends before the aborted
// Submit, so nothing in it may trip.
func replayPrefix(e *rules.Engine, cfg rules.Config, n int) (head string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("replay of the %d-intent prefix panicked: %v", n, r)
		}
	}()
	re, err := replay.ReplayTo(e.L, cfg, n)
	if re != nil {
		head = re.L.Head()
	}
	return head, err
}
