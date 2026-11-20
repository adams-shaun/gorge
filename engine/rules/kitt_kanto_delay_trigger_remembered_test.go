package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Kitt Kanto, Mayhem Diva's pump names `Creature.ControlledBy
// DelayTriggerRemembered`: the creature must belong to the player the
// ImmediateTrigger captured (`RememberObjects$ TriggeredPlayer`, the player
// whose beginning of combat it is), not to every player. The script is inline
// (the 2026-11-20 corpus spelling; the seat's .cards still carries the older
// ActivePlayerCtrl one).
const kittKantoSrc = "Name:Kitt Probe\nManaCost:1 R G W\nTypes:Legendary Creature Cat Bard\nPT:3/3\n" +
	"T:Mode$ Phase | Phase$ BeginCombat | Execute$ TrigImmediateTrig | TriggerZones$ Battlefield | TriggerDescription$ At the beginning of combat on each player's turn, you may tap two untapped creatures you control. When you do, target creature that player controls gets +2/+2 and gains trample until end of turn. Goad that creature.\n" +
	"SVar:TrigImmediateTrig:AB$ ImmediateTrigger | Cost$ tapXType<2/Creature> | RememberObjects$ TriggeredPlayer | Execute$ TrigPump | TriggerDescription$ When you do, target creature that player controls gets +2/+2 and gains trample until end of turn. Goad that creature.\n" +
	"SVar:TrigPump:DB$ Pump | ValidTgts$ Creature.ControlledBy DelayTriggerRemembered | TgtPrompt$ Select target creature that player controls | NumAtt$ +2 | NumDef$ +2 | KW$ Trample | SubAbility$ DBGoad\n" +
	"SVar:DBGoad:DB$ Goad | Defined$ Targeted\nOracle:x\n"

const kittBearSrc = "Name:Bear%s\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// kittKantoOffer builds a three-seat table with Kitt Kanto and a spare
// creature (the two creatures the cost taps) on seat 0 and one Bear each on
// seats 1 and 2, makes `active` the turn's player, drives the beginning-of-
// combat trigger through the tap cost, and returns the objects the "when you
// do" pump offers as targets plus each seat's Bear.
func kittKantoOffer(t *testing.T, active state.PlayerID) (offered []state.ObjID, bears [3]state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	e.Advance()
	onBoardReady(t, e, 0, kittKantoSrc)
	onBoardReady(t, e, 0, "Name:Bear0\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bears[0] = findByName(e, "Bear0", 0)
	for p := state.PlayerID(1); p <= 2; p++ {
		name := string(rune('0' + p))
		bears[p] = onBoardReady(t, e, p, "Name:Bear"+name+"\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	}
	e.G.Active = active
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat, Player: active})
	e.putTriggersOnStack()
	for range 24 {
		e.Advance()
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision before the pump's target ask")
		}
		switch {
		case d.Kind == decision.KTarget:
			for _, o := range d.Options {
				offered = append(offered, o.Obj)
			}
			return offered, bears
		case d.Kind == decision.KChoose && d.Min == 2 && d.Max == 2 && len(d.Options) >= 2:
			submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
		default:
			submitChoices(t, e, 0)
		}
	}
	t.Fatal("the pump's target ask never came")
	return nil, bears
}

func TestKittKantoDelayTriggerRemembered(t *testing.T) {
	for _, active := range []state.PlayerID{1, 2} {
		offered, bears := kittKantoOffer(t, active)
		// Precondition: three distinct creatures under three different
		// controllers, so a filter that names every player's creatures would
		// offer all of them.
		if bears[0] == 0 || bears[1] == 0 || bears[2] == 0 || bears[1] == bears[2] {
			t.Fatalf("active %d: fixture bears missing: %v", active, bears)
		}
		if !slices.Contains(offered, bears[active]) {
			t.Fatalf("active %d: the active player's Bear is not offered: %v (bears %v)", active, offered, bears)
		}
		for p, b := range bears {
			if state.PlayerID(p) != active && slices.Contains(offered, b) {
				t.Errorf("active %d: seat %d's Bear %d is offered although its controller is not the remembered player: %v", active, p, b, offered)
			}
		}
	}
}
