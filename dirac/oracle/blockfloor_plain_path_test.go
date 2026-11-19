package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1a with Brave the Sands on the plain block path: the kernel's plain
// offer (blocksCompletable, no must-block attacker or lure) counts one slot
// per blocker in no chosen pair, so a declared blocker's spare slot does not
// complete a menace floor there. Only the requirement path (blockerOptionsReq,
// reached through a must-block attacker/duty or a lure) counts it.
func TestBlockFloorsPlainPathSpareSlot(t *testing.T) {
	const token, elephant, frostwalla state.ObjID = 100, 101, 102
	const dungeoneer, giant, walker state.ObjID = 1, 2, 3
	d := &decision.Decision{Kind: decision.KBlockers, GroupLimits: map[string]int{"brave": 2}}
	pairs := []struct{ b, a state.ObjID }{
		{dungeoneer, token}, {giant, token},
		{giant, frostwalla}, {dungeoneer, elephant}, {walker, frostwalla},
	}
	for i, p := range pairs {
		o := decision.Option{Index: i, Obj: p.b, Attacker: p.a, Group: "brave"}
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
		{nil, true},
		{[]int{0}, true},     // dungeoneer on the token, giant still free to join
		{[]int{0, 2}, false}, // giant spent on frostwalla: no undeclared blocker has a token pair
		{[]int{0, 1}, true},  // the token's floor is met
		{[]int{0, 4}, true},  // the walker on frostwalla leaves giant free
	} {
		if got := blockFloorsMatchable(fr, c.chosen); got != c.want {
			t.Errorf("chosen %v: matchable %v, want %v", c.chosen, got, c.want)
		}
	}
}

// CR 509.1a with Min$ 3: one blocker blocks one attacker at most once, so a
// blocker's two spare slots may not cover two missing blockers of the same
// attacker. The matching is a flow with one unit per (attacker, blocker)
// pair; a second blocker with a pair on the attacker completes the floor.
func TestBlockFloorsRequirementRepeatPair(t *testing.T) {
	const token, frostwalla state.ObjID = 100, 102
	const dungeoneer, giant, walker state.ObjID = 1, 2, 3
	build := func(walkerToken bool) *frame {
		d := &decision.Decision{Kind: decision.KBlockers, GroupLimits: map[string]int{"brave": 2}}
		pairs := []struct{ b, a state.ObjID }{
			{dungeoneer, token}, {giant, token}, {giant, frostwalla}, {walker, frostwalla},
		}
		if walkerToken {
			pairs = append(pairs, struct{ b, a state.ObjID }{walker, token})
		}
		valid := make([]bool, len(pairs))
		for i, p := range pairs {
			valid[i] = true
			// AttackMust routes the offer to the requirement search, whose
			// spare slots this matching feeds.
			o := decision.Option{Index: i, Obj: p.b, Attacker: p.a, Group: "brave", AttackMust: p.a == token}
			if p.a == token {
				o.MinBlockers = 3
			}
			d.Options = append(d.Options, o)
		}
		return &frame{d: d, valid: valid}
	}
	// The token has one blocker and needs three. The giant's two spare slots
	// are one blocker, so alone they cannot field two more: only one attacker
	// pair exists and it may be used once.
	if got := blockFloorsMatchable(build(false), []int{0}); got {
		t.Error("giant's two spare slots matched two deficits of the same attacker: matchable, want false")
	}
	// A second blocker with a pair on the token completes the floor.
	if got := blockFloorsMatchable(build(true), []int{0}); !got {
		t.Error("giant and walker cover the token's two missing blockers: not matchable, want true")
	}
}
