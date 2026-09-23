package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseNumberHost is a scripted-ask double for the mid-resolution
// ChooseNumber ask (task cli-20260923T060000Z-choose-number): Ask records
// every posed decision and reports suspended, so the test can drive the
// engine's two passes (ask, then answer through Ctx.ChosenNumberPick /
// ChosenNumberAnswered) the way rules' resumeResolution does.
type chooseNumberHost struct {
	fakeHost
	asks      []*decision.Decision
	suspended bool
}

func (h *chooseNumberHost) Ask(d *decision.Decision) bool {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asks = append(h.asks, &cp)
	h.suspended = true
	return true
}

func (h *chooseNumberHost) Suspended() bool { return h.suspended }

// chooseNumberEvents collects the Choose "number" events a resolution
// emitted.
func chooseNumberEvents(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "number" {
			out = append(out, e)
		}
	}
	return out
}

// chooseNumberSrc builds a 2-seat game with one land source for seat 0.
func chooseNumberSrc(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	return h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
}

// TestChooseNumberAsksAndSuspendsMidResolution pins the ask itself (task
// cli-20260923T060000Z-choose-number): a resolution-time SP$ ChooseNumber
// poses a real KChoose over the deterministic number list to the Defined$
// player, records no Choose event, and suspends. Without the fix the effect
// recorded 0 and never asked, so the ask-shape assertions fail.
func TestChooseNumberAsksAndSuspendsMidResolution(t *testing.T) {
	h := &chooseNumberHost{}
	h.g = state.NewGame(names(2))
	src := chooseNumberSrc(t, &h.fakeHost)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseNumber | Defined$ You"))
	if len(h.asks) != 1 {
		t.Fatalf("asked %d decisions, want exactly one: %+v", len(h.asks), h.asks)
	}
	d := h.asks[0]
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 ||
		d.ResumeKind != "choosenumber" || d.ResumeSA == nil || d.Source != src {
		t.Fatalf("ChooseNumber ask shape wrong: %+v", d)
	}
	if d.Prompt != "Choose a number" {
		t.Fatalf("prompt = %q, want \"Choose a number\"", d.Prompt)
	}
	// Precondition: the list is non-trivial (so a pick is a real choice) and
	// carries the value on Amount, not only the label.
	if len(d.Options) < 2 {
		t.Fatalf("option list = %+v, want at least two numbers", d.Options)
	}
	for i, o := range d.Options {
		if o.Kind != "number" || o.Amount != i {
			t.Fatalf("option %d = %+v, want Kind number and Amount %d", i, o, i)
		}
	}
	if evs := chooseNumberEvents(&h.fakeHost); len(evs) != 0 {
		t.Fatalf("Choose event(s) emitted before the answer: %+v", evs)
	}
}

// TestChooseNumberReEntryEmitsTheAnsweredNumberOnce pins the resume: the
// answered number re-enters through Ctx.ChosenNumberPick (with the
// ChosenNumberAnswered marker), exactly one Choose event carries its Amount,
// events.Apply records it on the object (the shape every downstream
// o.ChosenNumber reader reads), and no second ask is posed. Without the fix
// the re-entry pass never existed -- the first pass recorded 0 outright.
func TestChooseNumberReEntryEmitsTheAnsweredNumberOnce(t *testing.T) {
	h := &chooseNumberHost{}
	h.g = state.NewGame(names(2))
	src := chooseNumberSrc(t, &h.fakeHost)
	sa := sa(t, "SP$ ChooseNumber | Defined$ You")
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa)
	if len(h.asks) != 1 || len(chooseNumberEvents(&h.fakeHost)) != 0 {
		t.Fatalf("first pass did not suspend on the ask: asks=%d choose=%d",
			len(h.asks), len(chooseNumberEvents(&h.fakeHost)))
	}
	h.suspended = false
	Resolve(h, &Ctx{Source: src, Controller: 0, ChosenNumberAnswered: true, ChosenNumberPick: 4}, sa)
	evs := chooseNumberEvents(&h.fakeHost)
	if len(evs) != 1 || evs[0].Amount != 4 || evs[0].Obj != src {
		t.Fatalf("re-entry Choose events = %+v, want exactly one Choose Amount 4 on the source", evs)
	}
	if h.g.Obj(src).ChosenNumber != 4 {
		t.Fatalf("ChosenNumber = %d, want the answered 4", h.g.Obj(src).ChosenNumber)
	}
	if len(h.asks) != 1 {
		t.Fatalf("re-entry posed %d asks, want none", len(h.asks)-1)
	}
}

// TestChooseNumberZeroAnswerIsAnsweredNotFallback pins the ambiguity the
// ChosenNumberAnswered marker exists for: ZERO is a legal pick, so an
// answered 0 must take the ANSWERED path (consume and clear the marker,
// emit exactly the fallback's event), never be mistaken for "never asked".
// The precondition asserts the marker is set and the pick is 0 before the
// resolution, and the postcondition asserts the marker is cleared -- a bare
// o.ChosenNumber guard could not tell this case from the no-choice case.
func TestChooseNumberZeroAnswerIsAnsweredNotFallback(t *testing.T) {
	h := &chooseNumberHost{}
	h.g = state.NewGame(names(2))
	src := chooseNumberSrc(t, &h.fakeHost)
	ctx := &Ctx{Source: src, Controller: 0, ChosenNumberAnswered: true, ChosenNumberPick: 0}
	Resolve(h, ctx, sa(t, "SP$ ChooseNumber | Defined$ You"))
	if len(h.asks) != 0 {
		t.Fatalf("an ANSWERED 0 posed %d asks, want none: %+v", len(h.asks), h.asks)
	}
	if ctx.ChosenNumberAnswered {
		t.Fatalf("the answered marker was not consumed by the answered path")
	}
	evs := chooseNumberEvents(&h.fakeHost)
	if len(evs) != 1 || evs[0].Amount != 0 || evs[0].Obj != src {
		t.Fatalf("answered-0 Choose events = %+v, want exactly one Choose Amount 0", evs)
	}
}

// TestChooseNumberNoHostDegradesToZero pins R-9: on a host that cannot ask
// (the plain fakeHost), the ask falls through to the deterministic fallback
// 0 with no extra Note, preserving the old stand-in's event byte-for-byte.
func TestChooseNumberNoHostDegradesToZero(t *testing.T) {
	h := newHost(t, 2)
	src := chooseNumberSrc(t, h)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "DB$ ChooseNumber | Defined$ You"))
	if h.g.Obj(src).ChosenNumber != 0 {
		t.Fatalf("no-host fallback = %d, want the deterministic 0", h.g.Obj(src).ChosenNumber)
	}
	evs := chooseNumberEvents(h)
	if len(evs) != 1 || evs[0].Amount != 0 {
		t.Fatalf("no-host Choose events = %+v, want exactly one Amount 0", evs)
	}
	for _, e := range h.log {
		if e.Kind == events.Note {
			t.Fatalf("unexpected Note on the no-host degradation: %+v", e)
		}
	}
}

// TestChooseNumberFreshAskAfterAnEarlierChoiceOnTheSource pins the
// stale-source-state rule the sibling colour ask settled: a ChosenNumber
// already on the object is an EARLIER choice's answer, not THIS ask's own, so
// a fresh resolution-time ChooseNumber still poses its ask. The precondition
// emits a real Choose "number" event (events.Apply folds it onto
// o.ChosenNumber) and asserts the field is set before the resolution runs.
// Without the fix the unconditional o.ChosenNumber guard returned before the
// ask, so the ask-count assertion fails.
func TestChooseNumberFreshAskAfterAnEarlierChoiceOnTheSource(t *testing.T) {
	h := &chooseNumberHost{}
	h.g = state.NewGame(names(2))
	src := chooseNumberSrc(t, &h.fakeHost)
	h.Emit(events.Event{Kind: events.Choose, Obj: src, Counter: "number", Amount: 5})
	if got := h.g.Obj(src).ChosenNumber; got != 5 {
		t.Fatalf("precondition: the earlier choice did not record: %d", got)
	}
	before := len(h.log)
	Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, "SP$ ChooseNumber | Defined$ You"))
	if len(h.asks) != 1 {
		t.Fatalf("a fresh ask after an earlier choice posed %d asks, want one: %+v", len(h.asks), h.asks)
	}
	if d := h.asks[0]; d.ResumeKind != "choosenumber" || len(d.Options) < 2 {
		t.Fatalf("fresh ask shape wrong: %+v", d)
	}
	if evs := chooseNumberEvents(&fakeHost{log: h.log[before:]}); len(evs) != 0 {
		t.Fatalf("Choose event(s) emitted before the answer: %+v", evs)
	}
}

// TestChooseNumberEntryBodyStaysTheNoOp pins the preserved half: the as-enters
// ENTRY-choice body (Ctx.ETBNumberRecorded -- rules' replCtx flags the
// K:ETBReplacement ChooseNumber repl's body) never poses an ask, even when
// the recorded entry answer is 0 (the value a bare o.ChosenNumber guard could
// not distinguish from unset). The flag is consumed so a nested ChooseNumber
// poses its own ask.
func TestChooseNumberEntryBodyStaysTheNoOp(t *testing.T) {
	// Recorded entry answer 0: the ambiguous case the flag exists for.
	h := &chooseNumberHost{}
	h.g = state.NewGame(names(2))
	src := chooseNumberSrc(t, &h.fakeHost)
	h.Emit(events.Event{Kind: events.Choose, Obj: src, Counter: "number", Amount: 0})
	ctx := &Ctx{Source: src, Controller: 0, ETBNumberRecorded: true}
	before := len(h.log)
	Resolve(h, ctx, sa(t, "DB$ ChooseNumber"))
	if len(h.asks) != 0 {
		t.Fatalf("the entry body posed %d asks, want none: %+v", len(h.asks), h.asks)
	}
	if len(h.log) != before {
		t.Fatalf("the entry body emitted %d event(s), want none", len(h.log)-before)
	}
	if ctx.ETBNumberRecorded {
		t.Fatalf("the entry-body flag was not consumed")
	}
	// A fresh resolution after the entry body still asks (the flag is gone).
	Resolve(h, ctx, sa(t, "SP$ ChooseNumber | Defined$ You"))
	if len(h.asks) != 1 {
		t.Fatalf("a resolution-time ask after the entry body posed %d asks, want one", len(h.asks))
	}
}
