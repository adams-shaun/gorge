package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
)

// TestChooseEffectsRecordWithoutAsking pins R-9: ChooseType/ChooseNumber on
// an object that already carries the choice emit nothing (the cast-time
// Choose event already recorded it), and on one without a choice they emit a
// Choose event with a deterministic fallback rather than asking.
func TestChooseEffectsRespectARecordedChoice(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	// Put a Goblin the controller owns so effChooseType's fallback is real.
	h.g.AddObject(mkCard(t, "Name:Grunt\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"), 0)
	h.g.Obj(src).ChosenType = "Kithkin"
	h.g.Obj(src).ChosenNumber = 5
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseType | Defined$ You"))
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseNumber | Defined$ You"))
	if h.g.Obj(src).ChosenType != "Kithkin" || h.g.Obj(src).ChosenNumber != 5 {
		t.Fatalf("a recorded choice must not be overwritten: type=%q number=%d",
			h.g.Obj(src).ChosenType, h.g.Obj(src).ChosenNumber)
	}
	for _, e := range h.log {
		if e.Kind == events.Choose {
			t.Fatalf("unexpected Choose event when the choice was already recorded: %+v", e)
		}
	}
}

func TestChooseEffectsRecordTheDeterministicFallback(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	// One owned Goblin means the type fallback is "Goblin", not "Human".
	h.g.AddObject(mkCard(t, "Name:Grunt\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"), 0)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseType | Defined$ You"))
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseNumber | Defined$ You"))
	var sawType, sawNumber []events.Event
	for _, e := range h.log {
		if e.Kind != events.Choose {
			continue
		}
		switch e.Counter {
		case "type":
			sawType = append(sawType, e)
		case "number":
			sawNumber = append(sawNumber, e)
		}
	}
	if len(sawType) != 1 || sawType[0].Text != "Goblin" {
		t.Fatalf("ChooseType fallback = %+v, want one Choose with Text Goblin", sawType)
	}
	if len(sawNumber) != 1 || sawNumber[0].Amount != 0 {
		t.Fatalf("ChooseNumber fallback = %+v, want one Choose with Amount 0", sawNumber)
	}
	if h.g.Obj(src).ChosenType != "Goblin" || h.g.Obj(src).ChosenNumber != 0 {
		t.Fatalf("fallback not recorded: type=%q number=%d", h.g.Obj(src).ChosenType, h.g.Obj(src).ChosenNumber)
	}
}

func TestChooseTypeFallsBackToHumanWhenControllerOwnsNoCreature(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseType | Defined$ You"))
	var last events.Event
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "type" {
			last = e
		}
	}
	if last.Text != "Human" {
		t.Fatalf("type fallback = %q, want Human", last.Text)
	}
}
