package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestManaEffectAsksForAnyColour exercises the resolution path, not the
// activated-mana path: a Mana SA reached through Resolve must suspend on a
// real WUBRG choice when its host can answer. The source is on the
// battlefield so this cannot pass with a vacuous source-less context.
func TestManaEffectAsksForAnyColour(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Any Source\nTypes:Land\nA:AB$ Mana | Produced$ Any\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	h.askResult = true
	h.suspendAfterAsk = true

	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "AB$ Mana | Produced$ Any"))
	if h.lastAsk == nil {
		t.Fatal("Produced$ Any did not pose a mid-resolution decision")
	}
	if len(h.lastAsk.Options) != 5 {
		t.Fatalf("Any options = %+v, want five colours", h.lastAsk.Options)
	}
	for _, colour := range []string{"W", "U", "B", "R", "G"} {
		found := false
		for _, option := range h.lastAsk.Options {
			if option.Kind == "mana" && option.Label == "Add "+colour {
				found = true
			}
		}
		if !found {
			t.Errorf("Any choice omitted %s: %+v", colour, h.lastAsk.Options)
		}
	}
	for _, event := range h.log {
		if event.Kind == events.ManaAdd {
			t.Fatalf("mana was added before the suspended answer: %+v", event)
		}
	}
}

// TestManaEffectReadsChosenColour proves the direct effect path handles a
// source's recorded as-enters colour rather than falling through to the
// unhandled-Produced$ note. The compared colours differ so a hard-coded
// colourless or first-colour result cannot satisfy the assertion.
func TestManaEffectReadsChosenColour(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Chosen Source\nTypes:Land\nA:AB$ Mana | Produced$ Chosen\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	src.ChosenColor = "G"

	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "AB$ Mana | Produced$ Chosen"))
	var green, colourless int32
	for _, event := range h.log {
		if event.Kind == events.ManaAdd && event.Counter == "G" {
			green += event.Amount
		}
		if event.Kind == events.ManaAdd && event.Counter == "C" {
			colourless += event.Amount
		}
		if event.Kind == events.Note && strings.Contains(event.Text, "unhandled Produced$") {
			t.Fatalf("chosen colour fell through to an unhandled-Produced$ note: %+v", event)
		}
	}
	if green != 1 || colourless != 0 {
		t.Fatalf("Chosen output green=%d colourless=%d, want one green and no colourless", green, colourless)
	}
}
