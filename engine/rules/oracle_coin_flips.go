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
	return func(p state.PlayerID) (bool, bool) {
		if len(queue) == 0 {
			r.extraFails = append(r.extraFails, harnessf("seat %d flips a coin past the scenario's %d recorded flips", p, len(flips)).Error())
			return false, false
		}
		f := queue[0]
		queue = queue[1:]
		if f.Seat != int(p) {
			r.extraFails = append(r.extraFails, harnessf("recorded flip (step %d) is seat %d's, but seat %d flips", f.Step, f.Seat, p).Error())
			return false, false
		}
		return f.Win, true
	}
}
