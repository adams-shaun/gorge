package rules

import "github.com/adams-shaun/gorge/engine/state"

// oracleCoinFlip is one recorded coin flip of the live game a lowered
// scenario replays: the step whose window it fell in, the flipper's seat,
// the result, and how many RNG draws it took there. Draws 0 is a flip a
// FlipCoinMod static (Edgar) forced, which draws nothing and so never asks
// the hook; 2 is a doubler (Krark's Thumb) flip, whose kept side is Win.
// Absent, Draws is 1.
type oracleCoinFlip struct {
	Step  int  `json:"step"`
	Seat  int  `json:"seat"`
	Win   bool `json:"win"`
	Draws *int `json:"draws,omitempty"`
}

// forcedFlips builds the run's CoinFlipHook from the scenario's recorded
// flips: each random flip takes the next drawn record's result, in order.
// A flip whose seat disagrees with its record, or one past the last record,
// is a harness failure and falls back to the seeded RNG. No records, no
// hook: the run flips exactly as before.
//
// The cursor is a function of the event log, not a bare counter: a
// resolution that asks after a flip is re-executed from its checkpoint
// (resolve.Kernel's resubmit), which restores the log and the RNG but not
// this closure. Every flip emits its result Note before the next flip asks
// the hook, so in one run the hook is asked at strictly increasing log
// lengths; taken holds the log length at each consumption, and a call at a
// length some consumption already reached is a re-execution of that flip,
// which rewinds the cursor to it and replays the same record.
func (r *oracleRun) forcedFlips(flips []oracleCoinFlip) func(state.PlayerID) (bool, bool) {
	var queue []oracleCoinFlip
	for _, f := range flips {
		if f.Draws == nil || *f.Draws > 0 {
			queue = append(queue, f)
		}
	}
	if len(queue) == 0 {
		return nil
	}
	var taken []int // the log length at each consumed record, ascending
	failedAt := -1  // the last log length a harness failure was reported at
	fail := func(at int, err error) (bool, bool) {
		if at > failedAt { // a re-executed failure reports once
			failedAt = at
			r.extraFails = append(r.extraFails, err.Error())
		}
		return false, false
	}
	return func(p state.PlayerID) (bool, bool) {
		at := len(r.e.L.Events)
		for len(taken) > 0 && taken[len(taken)-1] >= at {
			taken = taken[:len(taken)-1]
		}
		i := len(taken)
		if i >= len(queue) {
			return fail(at, harnessf("seat %d flips a coin past the scenario's %d recorded flips", p, len(flips)))
		}
		f := queue[i]
		if f.Seat != int(p) {
			return fail(at, harnessf("recorded flip (step %d) is seat %d's, but seat %d flips", f.Step, f.Seat, p))
		}
		taken = append(taken, at)
		return f.Win, true
	}
}
