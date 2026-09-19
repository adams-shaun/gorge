package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseTypeHost is a scripted-ask double for the mid-resolution ChooseType
// ask (task ct1): TypeChoices serves the configured typeChoices list and Ask
// records every posed decision and reports suspended, so the test can drive
// the engine's two passes (ask, then answer through Ctx.ChosenType) the way
// rules' resumeResolution does.
type chooseTypeHost struct {
	fakeHost
	asks      []*decision.Decision
	suspended bool
}

func (h *chooseTypeHost) TypeChoices(_ state.PlayerID, _ string) []decision.Option {
	return h.typeChoices
}

func (h *chooseTypeHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	h.suspended = true
	return true
}

func (h *chooseTypeHost) Suspended() bool { return h.suspended }

// chooseTypeAsks collects the Choose events a resolution emitted.
func chooseTypeAsks(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "type" {
			out = append(out, e)
		}
	}
	return out
}

// TestChooseTypeAsksAndSuspendsMidResolution pins the ask itself (task ct1):
// a resolution-time SP$ ChooseType poses a real KChoose over the host's
// TypeChoices list to the Defined$ player, records no Choose event, and
// suspends (no Choose event before the answer).
func TestChooseTypeAsksAndSuspendsMidResolution(t *testing.T) {
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	h.typeChoices = []decision.Option{
		{Index: 0, Kind: "type", Label: "Elf"},
		{Index: 1, Kind: "type", Label: "Zombie"},
	}
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseType | Defined$ You | Type$ Creature"))
	if len(h.asks) != 1 {
		t.Fatalf("asked %d decisions, want exactly one: %+v", len(h.asks), h.asks)
	}
	d := h.asks[0]
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 ||
		d.ResumeKind != "choosetype" || d.ResumeSA == nil || d.Source != src {
		t.Fatalf("ChooseType ask shape wrong: %+v", d)
	}
	if d.Prompt != "Choose a creature type" {
		t.Fatalf("prompt = %q, want the card-text ask", d.Prompt)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "type" || d.Options[0].Label != "Elf" ||
		d.Options[1].Kind != "type" || d.Options[1].Label != "Zombie" {
		t.Fatalf("option list = %+v, want the host's two \"type\" options in order", d.Options)
	}
	if asks := chooseTypeAsks(&h.fakeHost); len(asks) != 0 {
		t.Fatalf("Choose event(s) emitted before the answer: %+v", asks)
	}
}

// TestChooseTypeReEntryEmitsTheAnsweredTypeOnce pins the resume: the answered
// type re-enters through Ctx.ChosenType, exactly one Choose event carries it,
// events.Apply records it on the object (the shape every downstream
// Card.ChosenType reader reads), and no second ask is posed.
func TestChooseTypeReEntryEmitsTheAnsweredTypeOnce(t *testing.T) {
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	h.typeChoices = []decision.Option{
		{Index: 0, Kind: "type", Label: "Elf"},
		{Index: 1, Kind: "type", Label: "Zombie"},
	}
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	sa := sa(t, "SP$ ChooseType | Defined$ You | Type$ Creature")
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa)
	if len(h.asks) != 1 || len(chooseTypeAsks(&h.fakeHost)) != 0 {
		t.Fatalf("first pass did not suspend on the ask: asks=%d choose=%d",
			len(h.asks), len(chooseTypeAsks(&h.fakeHost)))
	}
	h.suspended = false
	Resolve(h, &Ctx{Source: src, Controller: 0, ChosenType: "Zombie"}, sa)
	asks := chooseTypeAsks(&h.fakeHost)
	if len(asks) != 1 || asks[0].Text != "Zombie" || asks[0].Obj != src {
		t.Fatalf("re-entry Choose events = %+v, want exactly one Choose \"Zombie\" on the source", asks)
	}
	if h.g.Obj(src).ChosenType != "Zombie" {
		t.Fatalf("ChosenType = %q, want the answered Zombie", h.g.Obj(src).ChosenType)
	}
	if len(h.asks) != 1 {
		t.Fatalf("re-entry posed %d asks, want none", len(h.asks)-1)
	}
}

// TestChooseTypeSingleOptionTakesTheFallbackWithoutAsking pins the
// strict-supersets gate: with zero or one offerable type the choice is
// forced (or empty), so no decision is posed and the deterministic fallback
// records the choice exactly as before (R-9 byte-identical contract).
func TestChooseTypeSingleOptionTakesTheFallbackWithoutAsking(t *testing.T) {
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	h.typeChoices = []decision.Option{{Index: 0, Kind: "type", Label: "Elf"}}
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	h.g.AddObject(mkCard(t, "Name:Grunt\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"), 0)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseType | Defined$ You | Type$ Creature"))
	if len(h.asks) != 0 {
		t.Fatalf("a one-option choice posed a decision: %+v", h.asks)
	}
	if h.g.Obj(src).ChosenType != "Goblin" {
		t.Fatalf("fallback not recorded: ChosenType = %q", h.g.Obj(src).ChosenType)
	}
}

// TestChooseTypeChooserFollowsDefined pins the chooser: the first Defined$
// player asks, not the resolving controller, when the SA names one.
func TestChooseTypeChooserFollowsDefined(t *testing.T) {
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	h.typeChoices = []decision.Option{
		{Index: 0, Kind: "type", Label: "Elf"},
		{Index: 1, Kind: "type", Label: "Zombie"},
	}
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseType | Defined$ Opponent | Type$ Creature"))
	if len(h.asks) != 1 || h.asks[0].Player != 1 {
		t.Fatalf("ask = %+v, want one decision owned by seat 1", h.asks)
	}
}

// TestChooseTypeNonCreatureCategoryStaysNotePlusFallback pins the boundary:
// a Type$ category this build cannot enumerate keeps the loud Note and the
// deterministic creature-type fallback, and never poses a list that cannot
// answer the question.
func TestChooseTypeNonCreatureCategoryStaysNotePlusFallback(t *testing.T) {
	h := &chooseTypeHost{}
	h.g = state.NewGame(names(2))
	h.typeChoices = nil
	src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseType | Defined$ You | Type$ Basic Land"))
	if len(h.asks) != 0 {
		t.Fatalf("a non-creature category posed a decision: %+v", h.asks)
	}
	notes := 0
	for _, e := range h.log {
		if e.Kind == events.Note {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("loud Note count = %d, want exactly one", notes)
	}
	if h.g.Obj(src).ChosenType != "Human" {
		t.Fatalf("fallback = %q, want Human (no owned creature types)", h.g.Obj(src).ChosenType)
	}
}
