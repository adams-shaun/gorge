package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Emiel the Blessed's "whenever another creature you control enters, you may
// pay {G/W}. If you do, put a +1/+1 counter on it" names the creature that
// entered. If that creature is blinked while the trigger waits on the stack it
// is a new object (CR 400.7), so the stale trigger puts nothing on it. The
// returned creature's own entry fires a second trigger, which does count it:
// the blinked board ends with ONE counter, the stale trigger's being the extra
// one a wrong referent would add.
func TestPutCounterTriggeredLKICopyNewIncarnation(t *testing.T) {
	emiel := card(t, "Name:Emiel Tester\nManaCost:2 W W\nTypes:Creature Unicorn\nPT:4/4\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | Execute$ TrigPutCounter | TriggerZones$ Battlefield | TriggerDescription$ x\n"+
		"SVar:TrigPutCounter:AB$ PutCounter | Cost$ 0 | Defined$ TriggeredCardLKICopy | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n")
	bear := card(t, "Name:Bear Tester\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	for _, blinked := range []bool{false, true} {
		e := New(Config{Seed: 717, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
		onBoardReadyCard(t, e, 0, emiel)
		id := enterCreature(t, e, 0, bear)
		e.putTriggersOnStack()
		if len(e.G.Stack) != 1 {
			t.Fatalf("blinked=%v: stack %v after the entry, want Emiel's trigger", blinked, e.G.Stack)
		}
		wantStack := 1
		if blinked {
			wantStack = 2
			inc := e.G.Obj(id).Incarnation
			flicker(e, id)
			if e.G.Obj(id).Incarnation == inc || e.G.Obj(id).Zone != state.ZBattlefield {
				t.Fatalf("the blink left %+v, want a later incarnation on the battlefield", e.G.Obj(id))
			}
		}
		e.putTriggersOnStack()
		if len(e.G.Stack) != wantStack {
			t.Fatalf("blinked=%v: stack %v, want %d Emiel triggers", blinked, e.G.Stack, wantStack)
		}
		e.askPriority(e.G.Active)
		for range 4 {
			if len(e.G.Stack) == 0 {
				break
			}
			if d := passUntilNonPriority(t, e, 8); d != nil {
				chooseKindOption(t, e, d, "trigger_cost_pay")
			}
		}
		if len(e.G.Stack) != 0 {
			t.Fatalf("blinked=%v: stack %v never drained", blinked, e.G.Stack)
		}
		if got := counterCount(e.G.Obj(id), "P1P1"); got != 1 {
			t.Fatalf("blinked=%v: %d +1/+1 counters, want 1", blinked, got)
		}
	}
}
