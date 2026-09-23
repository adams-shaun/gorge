package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseColorHost is a scripted-ask double for the mid-resolution ChooseColor
// ask (task cli-20260923T060000Z-choose-color): Ask records every posed
// decision and reports suspended, so the test can drive the engine's two
// passes (ask, then answer through Ctx.ChosenColor) the way rules'
// resumeResolution does.
type chooseColorHost struct {
	fakeHost
	asks      []*decision.Decision
	suspended bool
}

func (h *chooseColorHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	h.suspended = true
	return true
}

func (h *chooseColorHost) Suspended() bool { return h.suspended }

// chooseColorEvents collects the Choose events a resolution emitted.
func chooseColorEvents(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "color" {
			out = append(out, e)
		}
	}
	return out
}

// chooseColorSrc builds a 2-seat game with one land source for seat 0.
func chooseColorSrc(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	return h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
}

// The wantOptions list every unrestricted ChooseColor ask must offer: the
// fixed WUBRG order with full colour names as Labels, the same list shape
// the cast-time "as this enters" colour ask offers.
var wantWUBRGOptions = []decision.Option{
	{Index: 0, Kind: "color", Label: "White"},
	{Index: 1, Kind: "color", Label: "Blue"},
	{Index: 2, Kind: "color", Label: "Black"},
	{Index: 3, Kind: "color", Label: "Red"},
	{Index: 4, Kind: "color", Label: "Green"},
}

// TestChooseColorAsksAndSuspendsMidResolution pins the ask itself: a
// resolution-time SP$ ChooseColor poses a real KChoose over the fixed WUBRG
// colour list to the Defined$ player, records no Choose event, and suspends
// (no Choose event before the answer). Without the fix the effect recorded
// "W" and never asked, so this test's ask-shape assertions fail.
func TestChooseColorAsksAndSuspendsMidResolution(t *testing.T) {
	h := &chooseColorHost{}
	h.g = state.NewGame(names(2))
	src := chooseColorSrc(t, &h.fakeHost)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseColor | Defined$ You"))
	if len(h.asks) != 1 {
		t.Fatalf("asked %d decisions, want exactly one: %+v", len(h.asks), h.asks)
	}
	d := h.asks[0]
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 ||
		d.ResumeKind != "choosecolor" || d.ResumeSA == nil || d.Source != src {
		t.Fatalf("ChooseColor ask shape wrong: %+v", d)
	}
	if d.Prompt != "Choose a color" {
		t.Fatalf("prompt = %q, want \"Choose a color\"", d.Prompt)
	}
	if len(d.Options) != 5 {
		t.Fatalf("option list = %+v, want the five WUBRG colours", d.Options)
	}
	for i, want := range wantWUBRGOptions {
		if d.Options[i] != want {
			t.Fatalf("option %d = %+v, want %+v (fixed WUBRG order, full names)",
				i, d.Options[i], want)
		}
	}
	if evs := chooseColorEvents(&h.fakeHost); len(evs) != 0 {
		t.Fatalf("Choose event(s) emitted before the answer: %+v", evs)
	}
}

// TestChooseColorReEntryEmitsTheAnsweredLetterOnce pins the resume: the
// answered colour re-enters through Ctx.ChosenColor, exactly one Choose
// event carries its WUBRG letter, events.Apply records it on the object (the
// shape every downstream Card.ChosenColor reader reads), and no second ask
// is posed. Without the fix the re-entry pass never existed -- the first
// pass recorded "W" outright.
func TestChooseColorReEntryEmitsTheAnsweredLetterOnce(t *testing.T) {
	h := &chooseColorHost{}
	h.g = state.NewGame(names(2))
	src := chooseColorSrc(t, &h.fakeHost)
	sa := sa(t, "SP$ ChooseColor | Defined$ You")
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa)
	if len(h.asks) != 1 || len(chooseColorEvents(&h.fakeHost)) != 0 {
		t.Fatalf("first pass did not suspend on the ask: asks=%d choose=%d",
			len(h.asks), len(chooseColorEvents(&h.fakeHost)))
	}
	h.suspended = false
	Resolve(h, &Ctx{Source: src, Controller: 0, ChosenColor: "Black"}, sa)
	evs := chooseColorEvents(&h.fakeHost)
	if len(evs) != 1 || evs[0].Text != "B" || evs[0].Obj != src {
		t.Fatalf("re-entry Choose events = %+v, want exactly one Choose \"B\" on the source", evs)
	}
	if h.g.Obj(src).ChosenColor != "B" {
		t.Fatalf("ChosenColor = %q, want the answered B", h.g.Obj(src).ChosenColor)
	}
	if len(h.asks) != 1 {
		t.Fatalf("re-entry posed %d asks, want none", len(h.asks)-1)
	}
}

// TestChooseColorFallbackRespectsTheExclusion pins the deterministic
// no-host degradation reading the SA's own colour restriction: on a host
// that cannot ask (the plain fakeHost), Exclude$ White removes White from
// the option list, so the fallback records the next colour "U" -- NOT the
// first-WUBRG "W" the pre-fix stand-in always recorded.
func TestChooseColorFallbackRespectsTheExclusion(t *testing.T) {
	h := newHost(t, 2)
	src := chooseColorSrc(t, h)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseColor | Defined$ You | Exclude$ White"))
	if h.g.Obj(src).ChosenColor != "U" {
		t.Fatalf("excluded-White fallback = %q, want U", h.g.Obj(src).ChosenColor)
	}
	evs := chooseColorEvents(h)
	if len(evs) != 1 || evs[0].Text != "U" {
		t.Fatalf("excluded-White Choose events = %+v, want exactly one \"U\"", evs)
	}
}

// TestChooseColorChoicesRestrictsTheAskAndForcesASingleOffer pins the
// Choices$ restriction: with two named colours the ask offers exactly those
// two in Choices$-filtered WUBRG order; with one named colour the choice is
// forced and the fallback records it without posing an ask (the
// effChooseType strict-supersets convention).
func TestChooseColorChoicesRestrictsTheAskAndForcesASingleOffer(t *testing.T) {
	// Two named colours: the ask is real and restricted.
	h := &chooseColorHost{}
	h.g = state.NewGame(names(2))
	src := chooseColorSrc(t, &h.fakeHost)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseColor | Defined$ You | Choices$ black,red"))
	if len(h.asks) != 1 || len(h.asks[0].Options) != 2 ||
		h.asks[0].Options[0].Label != "Black" || h.asks[0].Options[1].Label != "Red" {
		t.Fatalf("Choices$ ask = %+v options %+v, want one ask over Black/Red", h.asks, h.asks[0].Options)
	}
	if evs := chooseColorEvents(&h.fakeHost); len(evs) != 0 {
		t.Fatalf("Choose event(s) emitted before the answer: %+v", evs)
	}
	// One named colour: forced, fallback records it, no ask.
	h2 := &chooseColorHost{}
	h2.g = state.NewGame(names(2))
	src2 := chooseColorSrc(t, &h2.fakeHost)
	Resolve(h2, &Ctx{Source: src2, Controller: 0}, sa(t, "SP$ ChooseColor | Defined$ You | Choices$ Red"))
	if len(h2.asks) != 0 {
		t.Fatalf("single-offer Choices$ posed %d asks, want none", len(h2.asks))
	}
	if h2.g.Obj(src2).ChosenColor != "R" {
		t.Fatalf("single-offer fallback = %q, want R", h2.g.Obj(src2).ChosenColor)
	}
}

// TestChooseColorRandomStaysTheSilentDeterministicFallback pins Random$:
// the card text makes the choice a die roll, never a player's pick, so no
// ask is posed and the deterministic first-WUBRG "W" stands in exactly as
// the pre-fix behaviour did (no Note, no ask, one Choose).
func TestChooseColorRandomStaysTheSilentDeterministicFallback(t *testing.T) {
	h := newHost(t, 2)
	src := chooseColorSrc(t, h)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseColor | Random$ True"))
	if h.g.Obj(src).ChosenColor != "W" {
		t.Fatalf("Random$ fallback = %q, want the deterministic W", h.g.Obj(src).ChosenColor)
	}
	if evs := chooseColorEvents(h); len(evs) != 1 || evs[0].Text != "W" {
		t.Fatalf("Random$ Choose events = %+v, want exactly one \"W\"", evs)
	}
	for _, e := range h.log {
		if e.Kind == events.Note {
			t.Fatalf("unexpected Note on a Random$ carrier: %+v", e)
		}
	}
}

// TestChooseColorUnaskableShapesNoteAndFallBack pins the loud convention the
// unsupported ChooseType category carries: a mid-resolution carrier whose
// option list this build cannot build (TwoColors$) keeps the deterministic
// first-WUBRG "W" AND emits the one Note naming the parameter, so the
// degradation is never silent.
func TestChooseColorUnaskableShapesNoteAndFallBack(t *testing.T) {
	h := newHost(t, 2)
	src := chooseColorSrc(t, h)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseColor | Defined$ You | TwoColors$ True"))
	if h.g.Obj(src).ChosenColor != "W" {
		t.Fatalf("TwoColors$ fallback = %q, want the deterministic W", h.g.Obj(src).ChosenColor)
	}
	notes := 0
	for _, e := range h.log {
		if e.Kind == events.Note {
			notes++
			if !strings.Contains(e.Text, "TwoColors$") {
				t.Fatalf("Note = %q, want it to name the unaskable parameter", e.Text)
			}
		}
	}
	if notes != 1 {
		t.Fatalf("TwoColors$ emitted %d Notes, want exactly one", notes)
	}
	if evs := chooseColorEvents(h); len(evs) != 1 || evs[0].Text != "W" {
		t.Fatalf("TwoColors$ Choose events = %+v, want exactly one \"W\"", evs)
	}
}
