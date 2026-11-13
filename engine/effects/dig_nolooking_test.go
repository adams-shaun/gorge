package effects

// NoLooking$ is Forge's presentation rider, not a blind dig on its own: the
// cards the spell says to look at are named and recorded as a private look,
// while a blind-shaped dig (ExileFaceDown$ or RandomChange$) keeps the
// "a card" labels and records nothing.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// digAskHost is askHost with the log position of the ask: the look Note the
// Dig records must arrive BEFORE the question about the window.
type digAskHost struct {
	fakeHost
	asked    *decision.Decision
	logAtAsk int
}

func (h *digAskHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asked = &cp
	h.logAtAsk = len(h.log)
	return true
}

// digestLookNote returns the one Secret Note in log[0:end] that records a
// private look at a non-empty library window.
func digestLookNote(log []events.Event, end int) (events.Event, bool) {
	for _, ev := range log[:end] {
		if ev.Kind == events.Note && ev.Secret && ev.From == state.ZLibrary && len(ev.IDs) > 0 {
			return ev, true
		}
	}
	return events.Event{}, false
}

// TestDigNoLookingNamesTheLookedCards drives the real corpus carriers whose
// Oracle text says "Look at the top ... cards": a bare NoLooking$ True must
// still name the offered cards and record the chooser's private look before
// the take ask.
func TestDigNoLookingNamesTheLookedCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, cardName := range []string{"Telling Time", "Moment of Truth", "Expressive Iteration"} {
		cardName := cardName
		t.Run(cardName, func(t *testing.T) {
			t.Parallel()
			card, ok := reg.Lookup(cardName)
			if !ok {
				t.Fatalf("%s missing from corpus", cardName)
			}
			face := card.Faces[0]
			if len(face.Abilities) == 0 || face.Abilities[0].API != "Dig" {
				t.Fatalf("%s root ability is not SP$ Dig: %+v", cardName, face.Abilities)
			}
			root := *face.Abilities[0]
			if root.Params["NoLooking"] != "True" {
				t.Fatalf("precondition: %s root Dig carries NoLooking$ %q", cardName, root.Params["NoLooking"])
			}
			// Drive only the main Dig: the card's SubAbility$ is another Dig
			// with its own ask and its own window.
			root.Sub = nil
			h := &digAskHost{}
			h.g = state.NewGame(names(2))
			source := h.g.AddObject(card, 0).ID
			names := []string{"Window Alpha", "Window Beta", "Window Gamma"}
			window := make([]state.ObjID, 0, len(names))
			for _, n := range names {
				window = append(window, h.g.AddObject(mkCard(t,
					"Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0).ID)
			}
			h.g.SetZone(state.ZLibrary, 0, window)
			lib := h.g.Zone(state.ZLibrary, 0)
			if len(lib) != len(window) || lib[0] != window[0] || lib[2] != window[2] {
				t.Fatalf("precondition: window not on top of library: got %v want %v", lib, window)
			}
			Resolve(h, &Ctx{Source: source, Controller: 0}, &root)
			if h.asked == nil || h.asked.Kind != decision.KChoose {
				t.Fatalf("decision = %+v, want the take ask over the three-card window", h.asked)
			}
			if len(h.asked.Options) != len(window) {
				t.Fatalf("options = %+v, want all %d window cards offered", h.asked.Options, len(window))
			}
			for i, opt := range h.asked.Options {
				if opt.Obj != window[i] {
					t.Fatalf("option %d object = %d, want library order %d", i, opt.Obj, window[i])
				}
				if !strings.Contains(opt.Label, names[i]) || opt.Label == "a card" {
					t.Fatalf("option %d label = %q, want the looked-at card %q", i, opt.Label, names[i])
				}
			}
			if !strings.Contains(h.asked.Prompt, "Look at the top 3 card(s)") {
				t.Fatalf("prompt = %q, want the look wording", h.asked.Prompt)
			}
			look, ok := digestLookNote(h.log, h.logAtAsk)
			if !ok {
				t.Fatalf("no Secret look Note precedes the take ask: log %+v", h.log)
			}
			if len(look.IDs) != len(window) || look.IDs[0] != window[0] || look.IDs[2] != window[2] {
				t.Fatalf("look Note ids = %v, want exactly the window %v", look.IDs, window)
			}
		})
	}
}

// TestDigNoLookingBlindCarriersStayBlind pins the other half: the two blind
// shapes still withhold the names and the look. The corpus carriers are
// Become Anonymous (ExileFaceDown$) and Chronicler of Worship (RandomChange$).
func TestDigNoLookingBlindCarriersStayBlind(t *testing.T) {
	cases := []struct {
		name   string
		params string
	}{
		{name: "random change", params: " | NoLooking$ True | RandomChange$ True | SkipReorder$ True"},
		{name: "face-down exile", params: " | NoLooking$ True | ExileFaceDown$ True | DestinationZone$ Exile | ChangeNum$ 1 | SkipReorder$ True"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			line := sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Card"+tc.params)
			h := &digAskHost{}
			h.g = state.NewGame(names(2))
			names := []string{"Blind Alpha", "Blind Beta", "Blind Gamma"}
			window := make([]state.ObjID, 0, len(names))
			for _, n := range names {
				window = append(window, h.g.AddObject(mkCard(t,
					"Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0).ID)
			}
			h.g.SetZone(state.ZLibrary, 0, window)
			lib := h.g.Zone(state.ZLibrary, 0)
			if len(lib) != len(window) || lib[0] != window[0] {
				t.Fatalf("precondition: window not on top of library: got %v want %v", lib, window)
			}
			Resolve(h, &Ctx{Controller: 0}, line)
			if h.asked == nil || h.asked.Kind != decision.KChoose || len(h.asked.Options) != len(window) {
				t.Fatalf("decision = %+v, want the take ask over the three-card window", h.asked)
			}
			for i, opt := range h.asked.Options {
				if opt.Label != "a card" {
					t.Fatalf("blind option %d label = %q, want the withheld %q", i, opt.Label, "a card")
				}
			}
			if strings.Contains(h.asked.Prompt, "Look at") {
				t.Fatalf("blind prompt = %q, want no look wording", h.asked.Prompt)
			}
			if look, ok := digestLookNote(h.log, len(h.log)); ok {
				t.Fatalf("blind dig recorded a private look at %v", look.IDs)
			}
		})
	}
}
