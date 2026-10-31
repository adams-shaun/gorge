package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const creaturesDieOnce = "Name:Mourning Bell\nManaCost:1\nTypes:Artifact\n" +
	"T:Mode$ ChangesZoneAll | ValidCards$ Creature | Origin$ Battlefield | Destination$ Graveyard | TriggerZones$ Battlefield | Execute$ BellDraw | TriggerDescription$ When creatures die, draw.\n" +
	"SVar:BellDraw:DB$ Draw | NumCards$ 1\nOracle:x\n"

// passToNonPriorityAsk passes priority until a decision other than priority is pending.
func passToNonPriorityAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind != decision.KPriority {
			return d
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
					t.Fatalf("pass: %v", err)
				}
				break
			}
		}
	}
	t.Fatal("no ask reached")
	return nil
}

// CR 101.4's own example: the active player chooses, then each other player in
// turn order, and only then are all the chosen creatures sacrificed, together.
func TestEachPlayerSacrificesChoosesThenMovesTogether(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	two := []*cards.Card{lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Hill Giant")}
	spell := append([]*cards.Card{lookup(t, reg, "Innocent Blood")}, two...)
	e := corpusEngineThree(t, reg, spell, two, two)
	for p := state.PlayerID(0); p < 3; p++ {
		moveByName(t, e, p, "Grizzly Bears", state.ZBattlefield)
		moveByName(t, e, p, "Hill Giant", state.ZBattlefield)
	}
	bell := onBoard(t, e, 0, creaturesDieOnce)
	moveByName(t, e, 0, "Innocent Blood", state.ZHand)
	addMana(t, e, 0, "B")
	castNamed(t, e, "Innocent Blood")

	mark := len(e.L.Events)
	for want := state.PlayerID(0); want < 3; want++ {
		d := passToNonPriorityAsk(t, e)
		if d.Kind != decision.KChoose || d.Player != want {
			t.Fatalf("ask %d: %v of seat %d, want the sacrifice of seat %d", want, d.Kind, d.Player, want)
		}
		if len(d.Options) != 2 {
			t.Fatalf("seat %d chooses among %d creatures, want 2", want, len(d.Options))
		}
		for _, ev := range e.L.Events[mark:] {
			if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
				t.Fatalf("obj %d left the battlefield before seat %d chose", ev.Obj, want)
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
			t.Fatalf("seat %d answers: %v", want, err)
		}
	}

	moved := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
			moved++
		} else if moved > 0 && moved < 3 && ev.Kind == events.DecisionAsk {
			t.Fatalf("an ask fell among the sacrifices after %d moved", moved)
		}
	}
	if moved != 3 {
		t.Fatalf("%d creatures left the battlefield, want 3", moved)
	}
	if got := queuedPhaseTriggers(e, bell) + triggerPushesOf(e, bell, mark); got != 1 {
		t.Fatalf("the batch trigger fired %d times, want 1 for one simultaneous sacrifice", got)
	}
}

func triggerPushesOf(e *Engine, id state.ObjID, mark int) int {
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.TriggerPush && ev.Obj == id {
			n++
		}
	}
	return n
}

// The held choices survive the tape's re-execution and replay byte for byte.
func TestEachPlayerSacrificesReplays(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	two := []*cards.Card{lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Hill Giant")}
	e, cfg := corpusEngineCfg(t, reg, append([]*cards.Card{lookup(t, reg, "Innocent Blood")}, two...), two)
	for p := state.PlayerID(0); p < 2; p++ {
		moveByName(t, e, p, "Grizzly Bears", state.ZBattlefield)
		moveByName(t, e, p, "Hill Giant", state.ZBattlefield)
	}
	moveByName(t, e, 0, "Innocent Blood", state.ZHand)
	addMana(t, e, 0, "B")
	castNamed(t, e, "Innocent Blood")
	for range 2 {
		d := passToNonPriorityAsk(t, e)
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[1].Index}}); err != nil {
			t.Fatalf("answer: %v", err)
		}
	}
	for p := state.PlayerID(0); p < 2; p++ {
		if got := len(e.G.Zone(state.ZBattlefield, p)); got != 1 {
			t.Fatalf("seat %d has %d permanents left, want 1", p, got)
		}
	}
	replayCheck(t, e, cfg)
}
