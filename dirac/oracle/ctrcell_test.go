package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The CounterPlayerAddedAll cell lockstep (gorge cr-pc-triggered-counter-map):
// one put action lands one trigger per (putter, recipient) however many kinds
// it places, while CounterTypes$ emits one CounterChange per kind. Abigale,
// Unexpected Fangs and Gift of the Viper put two or three kinds on one target;
// The Great Goblin (your Goblin) and Generous Patron (a creature you don't
// control) listen on either side of the table.
var kctrcellDeckA = []cardset.Entry{
	{Name: "The Great Goblin", Count: 4}, {Name: "Abigale, Eloquent First-Year", Count: 4}, {Name: "Unexpected Fangs", Count: 6},
	{Name: "Raging Goblin", Count: 5}, {Name: "Goblin Piker", Count: 5},
	{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4},
}

var kctrcellDeckB = []cardset.Entry{
	{Name: "Generous Patron", Count: 6}, {Name: "Gift of the Viper", Count: 8}, {Name: "Grizzly Bears", Count: 10},
	{Name: "Forest", Count: 16},
}

var kctrcellMatchups = [][2][]cardset.Entry{{kctrcellDeckA, kctrcellDeckB}, {kctrcellDeckA, kctrcellDeckA}}

// kctrcellCheck counts, on gorge's side, the put actions that landed a second
// kind on one recipient right after the first: the shape the cell merges.
func kctrcellCheck(multi *int) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			if seen == 0 || evs[seen].Kind != events.CounterChange || evs[seen-1].Kind != events.CounterChange {
				continue
			}
			if evs[seen].Obj == evs[seen-1].Obj && evs[seen].Counter != evs[seen-1].Counter && evs[seen].Amount > 0 {
				*multi++
			}
		}
		return nil
	}
}

func TestCtrcellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var multi int
	sonRun(t, "ctrcell", kctrcellMatchups, 300, true, nil, kctrcellCheck(&multi))
	t.Logf("second kinds landed by one put action on one recipient: %d", multi)
	if multi < 20 {
		t.Errorf("second kinds %d (want >= 20): the pool does not exercise the cell", multi)
	}
}

// TestCtrcellOverflowMutation: these games pile keyword counters past a
// permanent's slots, so a reader that ignores the overflow table diverges.
func TestCtrcellOverflowMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrOverProjOff = true
	defer func() { dirac.DebugCtrOverProjOff = false }()
	if n := sonRun(t, "ctrcell", kctrcellMatchups, 64, false, nil, nil); n == 0 {
		t.Errorf("mutation: no seed in 64 diverged")
	}
}

// TestCtrcellMutation: without the merge a later kind fires the listener again.
func TestCtrcellMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrCellOff = true
	defer func() { dirac.DebugCtrCellOff = false }()
	if n := sonRun(t, "ctrcell", kctrcellMatchups, 64, false, nil, nil); n == 0 {
		t.Errorf("mutation: no seed in 64 diverged")
	}
}
