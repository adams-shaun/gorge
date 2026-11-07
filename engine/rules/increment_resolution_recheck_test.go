package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestIncrementRechecksOnResolution pins CR 702.191a's intervening "if"
// (CR 603.4): Increment's condition is checked again as the ability
// resolves. A 1/1 Increment creature's controller casts a 2-mana spell; the
// opponent's SpellCast watcher (APNAP: put on the stack after, so it
// resolves first) puts a +1/+1 counter on it, making it 2/2. When Increment
// resolves, 2 is no longer greater than 2, so it does nothing: one counter,
// not two (the SOS-s104 Ambitious Augmenter + Graduation Day window).
func TestIncrementRechecksOnResolution(t *testing.T) {
	t.Parallel()
	inc := "Name:Inc Sprout\nManaCost:G\nTypes:Creature Plant\nPT:1/1\nK:Increment\nOracle:x\n"
	two := "Name:Two Spell\nManaCost:2\nTypes:Instant\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	watcher := "Name:Rival Grower\nManaCost:G\nTypes:Enchantment\n" +
		"T:Mode$ SpellCast | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ TrigGrow | TriggerDescription$ x\n" +
		"SVar:TrigGrow:DB$ PutCounterAll | ValidCards$ Creature.OppCtrl | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n"

	e, cfg, incID := newFixtureDeckWithOpponentCard(t, 924, inc, two, watcher)
	e.emit(events.Event{Kind: events.MoveZone, Obj: incID, From: state.ZHand, To: state.ZBattlefield})
	putCreature(t, e, 1, watcher)
	e.priorityRound()
	if p, tt := e.Power(incID), e.Toughness(incID); p != 1 || tt != 1 {
		t.Fatalf("precondition: Inc Sprout = %d/%d, want 1/1", p, tt)
	}
	start := len(e.L.Events)

	if got := incrementCast(t, e, incID, two); got != 1 {
		t.Errorf("counters = %d, want 1 (the watcher's counter only; Increment's 2 > 2 fails on resolution)", got)
	}
	// The precondition the assertion rests on: Increment did fire (2 > 1 at
	// cast time) and was removed by the resolution recheck, and the
	// watcher's trigger resolved first.
	fizzled := false
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.Text == "fizzled: intervening-if no longer holds" {
			if o := e.G.Obj(ev.Obj); o != nil && o.Source == incID {
				fizzled = true
			}
		}
	}
	if !fizzled {
		t.Errorf("Increment's trigger never fired and fizzled on its resolution recheck")
	}
	replayCheck(t, e, cfg)
}
