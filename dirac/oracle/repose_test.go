package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestReposeMenaceOffersEitherBlockerFirst pins the multi-pick filter on a
// menace attacker (gorge folds menace into Option.MinBlockers = 2) blocked
// by either of two creatures: each blocker is a legal FIRST pick, because
// the other can join it. The completability search used to extend only
// past the last pick's index, so the second-listed blocker (Perilous Myr
// against Boggart Brute, kfuzz vanilla game 57) was never offered.
func TestReposeMenaceOffersEitherBlockerFirst(t *testing.T) {
	t.Parallel()

	const attacker, lions, myr = state.ObjID(1), state.ObjID(41), state.ObjID(42)
	d := &decision.Decision{Kind: decision.KBlockers, Player: 1, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "block", Obj: lions, Attacker: attacker, MinBlockers: 2, Group: "blocker:41"},
			{Index: 1, Kind: "block", Obj: myr, Attacker: attacker, MinBlockers: 2, Group: "blocker:42"},
		}}
	fr := &frame{d: d, dk: DBlockers, multi: true,
		acts:  []Canon{Pack(ActBlock, 1, 0, 0), Pack(ActBlock, 2, 0, 0)},
		valid: []bool{true, true}}
	g := &GorgeSide{}
	g.repose(fr)
	want := []Canon{Pack(ActBlock, 1, 0, 0), Pack(ActBlock, 2, 0, 0), Pack(ActDone, RefNone, RefNone, 0)}
	if len(fr.cur.Options) != len(want) {
		t.Fatalf("first pick options %v, want %v", fr.cur.Options, want)
	}
	have := map[Canon]bool{}
	for _, a := range fr.cur.Options {
		have[a] = true
	}
	for _, a := range want {
		if !have[a] {
			t.Fatalf("first pick options %v miss %v", fr.cur.Options, a)
		}
	}
	// After the second-listed blocker, the first-listed one completes the
	// answer; Done is not yet valid (a lone blocker of a menace attacker).
	fr.chosen = []int{1}
	g.repose(fr)
	if len(fr.cur.Options) != 1 || fr.cur.Options[0] != Pack(ActBlock, 1, 0, 0) {
		t.Fatalf("after the Myr pick: options %v, want only the Lions block", fr.cur.Options)
	}
}
