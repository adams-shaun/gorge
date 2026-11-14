package oracle

import (
	"strconv"
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

// TestReposeBlockCeilingOnAWideBoard: an attacker under MinMaxBlocker Max$ 1 that
// already has its blocker takes no more, on a board wide enough that searching the
// over-ceiling answers for a completion would run out the budget and turn optimistic.
func TestReposeBlockCeilingOnAWideBoard(t *testing.T) {
	t.Parallel()

	const capped, other = state.ObjID(1), state.ObjID(2)
	d := &decision.Decision{Kind: decision.KBlockers, Player: 1, Min: 0, Max: 24}
	fr := &frame{d: d, dk: DBlockers, multi: true}
	for b := range 24 {
		for a, atk := range []state.ObjID{capped, other} {
			maxb := 0
			if atk == capped {
				maxb = 1
			}
			id := state.ObjID(40 + b)
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "block", Obj: id, Attacker: atk,
				MaxBlockers: maxb, Group: "blocker:" + strconv.Itoa(int(id))})
			fr.acts = append(fr.acts, Pack(ActBlock, Ref(id), Ref(a), 0))
			fr.valid = append(fr.valid, true)
		}
	}
	fr.chosen = []int{0} // the first creature blocks the capped attacker
	(&GorgeSide{}).repose(fr)
	for _, at := range fr.curAt {
		if at >= 0 && d.Options[at].Attacker == capped {
			t.Fatalf("offered a second blocker on the capped attacker: %+v", d.Options[at])
		}
	}
	if want := 23 + 1; len(fr.cur.Options) != want { // the other attacker's blockers and Done
		t.Fatalf("%d options, want %d", len(fr.cur.Options), want)
	}
}
