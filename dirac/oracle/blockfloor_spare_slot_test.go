package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1a with Brave the Sands: a declared blocker's spare slot (its group
// cap less its chosen pairs) can still meet a menace attacker's floor, and the
// match must not reuse a pair the answer already declared — one blocker blocks
// an attacker at most once. Seasoned Dungeoneer already blocks the menace
// Skeleton Token; while Palisade Giant blocks Frostwalla it still holds the
// slot that completes the token, which the emulation once denied by building
// its free set from blockers in no chosen pair (decision 414 of the pair-36
// recording behind TestMBCPalisadeGiantReprosLockstep).
func TestBlockFloorsSpareSlotMatching(t *testing.T) {
	const token, elephant, frostwalla state.ObjID = 100, 101, 102
	const dungeoneer, giant, walker state.ObjID = 1, 2, 3
	d := &decision.Decision{Kind: decision.KBlockers, GroupLimits: map[string]int{"brave": 2}}
	pairs := []struct{ b, a state.ObjID }{
		{dungeoneer, token}, {giant, token},
		{giant, frostwalla}, {dungeoneer, elephant}, {walker, frostwalla},
	}
	for i, p := range pairs {
		// AttackMust routes the kernel to blockerOptionsReq, whose search
		// counts a declared blocker's spare slot; the plain blocksCompletable
		// path does not.
		o := decision.Option{Index: i, Obj: p.b, Attacker: p.a, Group: "brave", AttackMust: p.a == token}
		if p.a == token {
			o.MinBlockers = 2
		}
		d.Options = append(d.Options, o)
	}
	fr := &frame{d: d, valid: []bool{true, true, true, true, true}}
	for _, c := range []struct {
		chosen []int
		want   bool
	}{
		{[]int{0}, true},          // dungeoneer on the token, giant's two slots open
		{[]int{0, 2}, true},       // giant on frostwalla keeps the slot that completes the token
		{[]int{0, 4}, true},       // the walker on frostwalla leaves giant's slots open
		{[]int{0, 1, 2}, true},    // the token's floor is met by dungeoneer and giant
		{[]int{0, 1, 2, 3}, true}, // giant's spare covers the elephant
	} {
		if got := blockFloorsMatchable(fr, c.chosen); got != c.want {
			t.Errorf("chosen %v: matchable %v, want %v", c.chosen, got, c.want)
		}
	}
}

// The same board with a single-slot giant (no group cap): once it is spent on
// frostwalla the token's missing blocker has no holder with a free pair —
// dungeoneer's spare slot may not re-block the token (CR 509.1a: one blocker
// blocks an attacker at most once) and the walker has no pair on it.
func TestBlockFloorsSpareSlotExhausted(t *testing.T) {
	const token, frostwalla state.ObjID = 100, 102
	const dungeoneer, giant, walker state.ObjID = 1, 2, 3
	d := &decision.Decision{Kind: decision.KBlockers, GroupLimits: map[string]int{"brave": 2}}
	pairs := []struct{ b, a state.ObjID }{
		{dungeoneer, token}, {giant, token},
		{giant, frostwalla}, {walker, frostwalla},
	}
	for i, p := range pairs {
		o := decision.Option{Index: i, Obj: p.b, Attacker: p.a, Group: "brave", AttackMust: p.a == token}
		if p.a == token {
			o.MinBlockers = 2
		}
		if p.b == giant {
			o.Group = "" // no Brave the Sands slot for this creature
		}
		d.Options = append(d.Options, o)
	}
	fr := &frame{d: d, valid: []bool{true, true, true, true}}
	for _, c := range []struct {
		chosen []int
		want   bool
	}{
		{[]int{0}, true},     // dungeoneer on the token, giant free to join
		{[]int{0, 1}, true},  // the token's floor is met
		{[]int{0, 2}, false}, // giant spent on frostwalla: no free pair on the token left
		{[]int{0, 3}, true},  // the walker is spent, giant still free
	} {
		if got := blockFloorsMatchable(fr, c.chosen); got != c.want {
			t.Errorf("chosen %v: matchable %v, want %v", c.chosen, got, c.want)
		}
	}
}
