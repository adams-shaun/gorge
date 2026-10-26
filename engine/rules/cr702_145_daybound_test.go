package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const plainDFCSrc = "Name:Plain Front\nTypes:Creature\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n\nALTERNATE\n\nName:Plain Back\nTypes:Creature\nPT:2/2\nOracle:x\n"

// transformedFlips counts CR 701.27 transforms (FlipFace, Text "Transformed")
// of id from log index from.
func transformedFlips(e *Engine, id state.ObjID, from int) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.FlipFace && ev.Obj == id && ev.Text == "Transformed" {
			n++
		}
	}
	return n
}

// triggerPushes counts TriggerPush events whose source permanent is id.
func triggerPushes(e *Engine, id state.ObjID, from int) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.TriggerPush && ev.Obj == id {
			n++
		}
	}
	return n
}

func faceName(t *testing.T, e *Engine, id state.ObjID) string {
	t.Helper()
	f := e.G.Obj(id).Face()
	if f == nil {
		t.Fatalf("object %d has no face", id)
	}
	return f.Name
}

func TestCR702_145cDayboundTransformsAsItBecomesNight(t *testing.T) {
	e, cfg := dayNightGame(t)
	cathar := searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	driveToStep(t, e, 2, 1, state.StepUpkeep)
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("precondition: %q, want night", e.G.DayNight)
	}
	if got := faceName(t, e, cathar); got != "Moonrage Brute" {
		t.Fatalf("face = %q, want Moonrage Brute (CR 702.145c)", got)
	}
	if n := transformedFlips(e, cathar, 0); n != 1 {
		t.Fatalf("transforms = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

func TestCR702_145fNightboundTransformsAsItBecomesDay(t *testing.T) {
	e, cfg := dayNightGame(t)
	cathar := searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	driveToStep(t, e, 3, 0, state.StepMain1)
	castVanilla(t, e, "Ping", "R")
	castVanilla(t, e, "Ping", "R")
	mark := len(e.L.Events)
	passUntilTurn(t, e, 4)
	if e.G.DayNight != state.DayNightDay {
		t.Fatalf("precondition: %q, want day", e.G.DayNight)
	}
	if got := faceName(t, e, cathar); got != "Brutal Cathar" {
		t.Fatalf("face = %q, want Brutal Cathar (CR 702.145f)", got)
	}
	if n := transformedFlips(e, cathar, 0); n != 2 {
		t.Fatalf("transforms = %d, want 2 (night, then day)", n)
	}
	if n := triggerPushes(e, cathar, mark); n != 1 {
		t.Fatalf("transform triggers = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

func TestCR702_145bDayboundEntersTransformedAtNight(t *testing.T) {
	e, cfg := dayNightGame(t)
	searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	driveToStep(t, e, 3, 0, state.StepMain1)
	if e.G.DayNight != state.DayNightNight {
		t.Fatalf("precondition: %q, want night", e.G.DayNight)
	}
	mark := len(e.L.Events)
	id := searchMoveByName(t, e, "Graveyard Trespasser", state.ZBattlefield)
	e.checkStateBased()
	if got := faceName(t, e, id); got != "Graveyard Glutton" {
		t.Fatalf("face = %q, want Graveyard Glutton", got)
	}
	if n := transformedFlips(e, id, 0); n != 0 {
		t.Fatalf("transforms = %d, want 0 (entering transformed is no transform)", n)
	}
	if n := triggerPushes(e, id, mark); n != 1 {
		t.Fatalf("enters triggers = %d, want 1 (the Glutton's own)", n)
	}
	replayCheck(t, e, cfg)
}

func TestCR702_145bDayboundCantBeTransformedByOtherEffects(t *testing.T) {
	e, _ := dayNightGame(t)
	cathar := searchMoveByName(t, e, "Brutal Cathar", state.ZBattlefield)
	plain := onBoard(t, e, 0, plainDFCSrc)
	src := onBoard(t, e, 0, "Name:Transformer\nTypes:Artifact\nSVar:Flip:DB$ SetState | ValidTgts$ Creature | Mode$ Transform\nOracle:x\n")
	face := e.G.Obj(src).Face()
	for _, id := range []state.ObjID{cathar, plain} {
		effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, SVars: face.SVars,
			Targets: []state.Target{{Obj: id}}, TargetsOffered: true},
			cards.ResolveSVar(face.SVars, "Flip"))
	}
	if got := faceName(t, e, cathar); got != "Brutal Cathar" {
		t.Fatalf("daybound face = %q, want Brutal Cathar (CR 702.145b)", got)
	}
	if got := faceName(t, e, plain); got != "Plain Back" {
		t.Fatalf("plain DFC face = %q, want Plain Back (precondition: the effect works)", got)
	}
}

func TestDayboundNightboundAreRegistered(t *testing.T) {
	sup := effects.Supported()
	for _, k := range []string{"kw:Daybound", "kw:Nightbound"} {
		if !sup[k] {
			t.Errorf("%s is not a supported primitive", k)
		}
	}
	reg := searchTestRegistry(t)
	for _, name := range []string{"Brutal Cathar", "Graveyard Trespasser"} {
		for _, p := range searchCorpusCard(t, reg, name).Primitives() {
			if !sup[p] {
				t.Errorf("%s: primitive %s unsupported", name, p)
			}
		}
	}
}
