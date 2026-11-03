package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// xGrowerSrc grows when another creature enters: "you may pay {X}. When you
// do, put X +1/+1 counters on it" (the reflexive-trigger idiom).
const xGrowerSrc = "Name:Patient Grower\nManaCost:1 G\nTypes:Creature Elf\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | TriggerZones$ Battlefield | Execute$ TrigAskGrow | TriggerDescription$ Whenever another creature you control enters, you may pay {X}. When you do, put X +1/+1 counters on this creature.\n" +
	"SVar:TrigAskGrow:AB$ ImmediateTrigger | Cost$ X | Execute$ TrigGrowBy\n" +
	"SVar:TrigGrowBy:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ X\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// TestTriggerPaidXZeroBindsOverTheCausingSpellsX pins that a "you may pay {X}"
// trigger paid X = 0 puts no counters even when the creature that triggered it
// was cast with a positive X: CR 107.3m binds a trigger's X to the causing
// permanent's X only for an enters-the-battlefield ability of that permanent,
// and this trigger's X is the one its controller just announced (CR 107.3).
func TestTriggerPaidXZeroBindsOverTheCausingSpellsX(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Shanna, Purifying Blade")
	grower := onBoard(t, e, 0, xGrowerSrc)
	for i := 0; i < 3; i++ {
		xFoldLand(t, e, "Test Forest "+string(rune('A'+i)), "G")
	}
	entrant := e.G.AddObject(card(t, "Name:Big Entrant\nManaCost:X G\nTypes:Creature Hydra\nPT:0/0\nOracle:x\n"), 0)
	entrant.Zone = state.ZLibrary
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	e.G.SetZone(state.ZLibrary, 0, append(ids, entrant.ID))
	entrant.X = 2
	e.emit(events.Event{Kind: events.MoveZone, Obj: entrant.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()

	sawAsk := false
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		pick := 0
		if d.Kind == decision.KPriority {
			if len(e.G.Zone(state.ZStack, 0)) == 0 && len(e.G.Zone(state.ZStack, 1)) == 0 {
				break
			}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pick = o.Index
				}
			}
		} else if len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" {
			sawAsk = true // option 0 is X = 0
		}
		submitChoices(t, e, pick)
	}
	if !sawAsk {
		t.Fatal("the X announcement ask never appeared")
	}
	if n := e.G.Obj(grower).Counter("P1P1"); n != 0 {
		t.Fatalf("a trigger paid X = 0 put %d +1/+1 counters, want 0", n)
	}
}
