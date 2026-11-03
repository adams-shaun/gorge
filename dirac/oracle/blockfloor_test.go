package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBlockFloorsMatchable: with a menace attacker blocked by one creature,
// a further pair is completable only while some other undeclared creature can
// still join that attacker (CR 509.1a). A flier that holds the menace
// attacker's only second blocker cannot be spent on a ground attacker.
func TestBlockFloorsMatchable(t *testing.T) {
	const menace, ground state.ObjID = 100, 101
	const first, flier, walker state.ObjID = 1, 2, 3
	d := &decision.Decision{Kind: decision.KBlockers}
	pairs := []struct{ b, a state.ObjID }{
		{first, menace}, {flier, menace}, {flier, ground}, {walker, ground},
	}
	for i, p := range pairs {
		o := decision.Option{Index: i, Obj: p.b, Attacker: p.a}
		if p.a == menace {
			o.MinBlockers = 2
		}
		d.Options = append(d.Options, o)
	}
	fr := &frame{d: d, valid: []bool{true, true, true, true}}
	for _, c := range []struct {
		chosen []int
		want   bool
	}{
		{nil, true},
		{[]int{0}, true},     // first on the menace attacker: the flier can join
		{[]int{0, 1}, true},  // both declared
		{[]int{0, 2}, false}, // the flier is spent on the ground attacker
		{[]int{0, 3}, true},  // the walker blocks the ground attacker, the flier stays free
		{[]int{3}, true},     // the menace attacker is not blocked yet
	} {
		if got := blockFloorsMatchable(fr, c.chosen); got != c.want {
			t.Errorf("chosen %v: matchable %v, want %v", c.chosen, got, c.want)
		}
	}
}
