package dirac

import "testing"

// TestWithRootExhaustReadsTheSearchRoot: a prefix search's fork that marks
// p's Exhaust activation (an Exhaust mana ability it activated) prices the
// play with the root's fact, through nested forks, and gets its own back; a
// State that is no fork reads its own fact.
func TestWithRootExhaustReadsTheSearchRoot(t *testing.T) {
	a := NewConfig(testTable(), nil, nil, 1, Config{})
	if a.turnExhaustBit(0) {
		t.Fatal("precondition: the root holds no Exhaust activation")
	}
	outer := a.fork()
	outer.setTurnExhaust(0)
	inner := outer.fork()
	if !inner.turnExhaustBit(0) {
		t.Fatal("precondition: the nested fork copies the outer fork's Exhaust mark")
	}
	saved := inner.withRootExhaust(0)
	if inner.turnExhaustBit(0) {
		t.Error("the nested fork read its own Exhaust mark, not the root's")
	}
	inner.g.turnF = saved
	if !inner.turnExhaustBit(0) {
		t.Error("the fork's own Exhaust mark was not restored")
	}
	outer.unfork()
	a.unfork()

	a.setTurnExhaust(0)
	saved = a.withRootExhaust(0)
	if !a.turnExhaustBit(0) {
		t.Error("a State that is no fork must read its own Exhaust mark")
	}
	a.g.turnF = saved
}
