package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestNameCardChooseFromListRestrictsUniverseAsk(t *testing.T) {
	h := &askHost{}
	h.g = namecardGameWithUniverse(t)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0).ID
	saLine := sa(t, "SP$ NameCard | ChooseFromList$ Wasteland,Bear")
	Resolve(h, &Ctx{Controller: 0, Source: src}, saLine)
	if h.asked == nil {
		t.Fatal("NameCard did not ask")
	}
	if len(h.asked.Options) != 2 || !containsLabel(h.asked.Options, "Wasteland") || !containsLabel(h.asked.Options, "Bear") {
		t.Fatalf("ChooseFromList options = %+v, want exactly Bear and Wasteland", h.asked.Options)
	}
	if containsLabel(h.asked.Options, "Forest") {
		t.Fatalf("ChooseFromList offered unlisted Forest: %+v", h.asked.Options)
	}
}

func TestNameCardAtRandomUsesEngineRandomAndRecords(t *testing.T) {
	h := &askHost{}
	h.g = namecardGameWithUniverse(t)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Controller: 0, Source: src}, sa(t, "SP$ NameCard | AtRandom$ True | ChooseFromList$ Wasteland,Bear"))
	if h.asked != nil {
		t.Fatalf("AtRandom NameCard asked the player: %+v", h.asked)
	}
	if h.n != 1 {
		t.Fatalf("engine Rand calls = %d, want 1", h.n)
	}
	if got := h.g.Obj(src).ChosenName; got != "Bear" {
		t.Fatalf("random ChosenName = %q, want deterministic engine-RNG result Bear", got)
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Choose || h.log[0].Counter != "name" || h.log[0].Text != "Bear" {
		t.Fatalf("name event = %+v, want recorded random Bear choice", h.log)
	}
}

func TestNameCardWithoutUniverseIgnoresNewChoiceModes(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0).ID
	lib := h.g.AddObject(mkCard(t, "Name:Grizzly Bears\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: lib, From: state.ZHand, To: state.ZLibrary, Player: 0})
	Resolve(h, &Ctx{Controller: 0, Source: src}, sa(t, "SP$ NameCard | AtRandom$ True | ChooseFromList$ Bear"))
	if got := h.g.Obj(src).ChosenName; got != "Grizzly Bears" {
		t.Fatalf("legacy no-universe name = %q, want exact library-top fallback", got)
	}
}
