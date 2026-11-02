package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTriggerLimitHoldsOnLookBack pins "this ability triggers only once each
// turn" across the source's own death: Battle-Scarred Survivalist fired on the
// first creature to die, then dies alongside a second. It sees the second die
// by looking back (CR 603.10a), as the permanent it was, and that permanent's
// one trigger is spent; the limit used to read the graveyard card's fresh count.
func TestTriggerLimitHoldsOnLookBack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := combatTriggerBoard(t, reg, []string{"Battle-Scarred Survivalist"}, nil, nil, []string{
		"Name:First Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n",
		"Name:Second Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"})
	survivalist := findBattlefield(t, e, 0, "Battle-Scarred Survivalist", 0)
	first := findBattlefield(t, e, 1, "First Bear", 0)
	second := findBattlefield(t, e, 1, "Second Bear", 0)

	pushes := func() int {
		n := 0
		for i := range e.pendingTriggers {
			if e.pendingTriggers[i].Source == survivalist {
				n++
			}
		}
		return n
	}
	e.emit(events.Event{Kind: events.Damage, Obj: first, Amount: 2})
	e.checkStateBased()
	if got := pushes(); got != 1 {
		t.Fatalf("the first death queued %d triggers, want 1", got)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: survivalist, Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: second, Amount: 2})
	e.checkStateBased()
	if got := pushes(); got != 1 {
		t.Fatalf("after the second death %d triggers are queued, want the first's 1", got)
	}
}
