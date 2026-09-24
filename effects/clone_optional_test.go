package effects

// The Optional$ True election leaves of DB$ Clone (ticket
// api-clone-trigger-copy, Sarkhan Soul Aflame's shape): the no-host stand-in
// and the Ctx answer-field contract the rules-side resume arm depends on.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// cloneOptionalSA is Sarkhan Soul Aflame's compiled clone body verbatim.
const cloneOptionalSA = "DB$ Clone | Defined$ TriggeredCardLKICopy | NewName$ Sarkhan, Soul Aflame | AddTypes$ Legendary | Duration$ UntilEndOfTurn | Optional$ True"

// cloneOptionalFixture puts a Sarkhan-like permanent (the become operand,
// the SA's default CloneTarget$ Self) and a Dragon-like body (the copy
// source) on seat 0's battlefield and returns (host, sarkID, dragonID). The
// ctx the caller passes carries the Dragon in Remembered, exactly what the
// rules engine binds for a ChangesZone trigger's TriggeredCardLKICopy.
func cloneOptionalFixture(t *testing.T) (*fakeHost, state.ObjID, state.ObjID) {
	t.Helper()
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	sark := mkCard(t, "Name:Sarkhan, Soul Aflame\nManaCost:1 U R\nTypes:Legendary Creature Human Shaman\nPT:2/4\nOracle:x\n")
	dragon := mkCard(t, "Name:Dragon Hatchling\nManaCost:1 R\nTypes:Creature Dragon\nPT:0/1\nK:Flying\nOracle:x\n")
	sarkID := h.g.AddObject(sark, 0).ID
	dragonID := h.g.AddObject(dragon, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{sarkID, dragonID})
	// SetZone moves the zone LIST only; the object's cached Zone field is
	// what effClone's become-object check reads (the attach_test convention).
	h.g.Obj(sarkID).Zone = state.ZBattlefield
	h.g.Obj(dragonID).Zone = state.ZBattlefield
	return h, sarkID, dragonID
}

// TestCloneOptionalNoHostTakesTheCopy pins the R-9 no-host stand-in: a host
// that cannot ask resolves the "you may" copy deterministically as take,
// with the one degradation Note and the same layer-1 copy the pre-election
// build performed.
func TestCloneOptionalNoHostTakesTheCopy(t *testing.T) {
	h, sark, dragon := cloneOptionalFixture(t)
	ctx := &Ctx{Controller: 0, Source: sark, Remembered: []state.Target{{Obj: dragon}}}
	Resolve(h, ctx, sa(t, cloneOptionalSA))
	f := h.g.Obj(sark).Face()
	if f == nil || f.Name != "Sarkhan, Soul Aflame" {
		t.Fatalf("no-host copy name %v, want the overridden Sarkhan, Soul Aflame", f)
	}
	if h.g.Obj(sark).CopyFace == nil {
		t.Fatal("no-host run recorded no copy basis")
	}
	took := false
	for _, e := range h.log {
		if e.Kind == events.Note && e.Text == "Clone Optional$ resolved as take (no engine host to ask)" {
			took = true
		}
	}
	if !took {
		t.Fatal("no-host take ran without its degradation Note")
	}
}

// TestCloneOptionalAnswerFieldIsConsumedAndCleared pins the fx42 scoping the
// rules resume arm relies on: a re-entry with the answered decline performs
// no copy and clears both fields.
// TestCloneOptionalAcceptedAnswerEmitsTheUnreadRiderNote is the
// findings-r2 MAJOR regression: the accepted may-copy re-entry (cloneDone
// set, a real host that answered yes) must still emit the combined
// unread-rider Note. The first pass returns at the Ask before the note, so
// without the fix the diagnostic never fires for any carrier that asks
// (Kimahri / Vesuvan Doppelganger / Lazav's AddSVars$ family, measured 7
// corpus lines). The decline path returns before the note, so it must not
// emit one.
func TestCloneOptionalAcceptedAnswerEmitsTheUnreadRiderNote(t *testing.T) {
	const sAVarsSA = "DB$ Clone | Defined$ TriggeredCardLKICopy | NewName$ Kimahri, Valiant Guardian | GainThisAbility$ True | Optional$ True | AddSVars$ RonsoCounter,RonsoTap"
	want := "Clone does not read: AddSVars$ RonsoCounter,RonsoTap"
	// Accepted answer.
	h, sark, dragon := cloneOptionalFixture(t)
	ctx := &Ctx{Controller: 0, Source: sark, Remembered: []state.Target{{Obj: dragon}},
		Clone: "yes", CloneDone: true}
	Resolve(h, ctx, sa(t, sAVarsSA))
	if f := h.g.Obj(sark).Face(); f == nil || f.Name != "Kimahri, Valiant Guardian" {
		t.Fatalf("accepted copy precondition failed: name %v (the copy must have happened for the note run to be about a real copy)", f)
	}
	if !hasNote(h, want) {
		t.Fatalf("accepted-answer clone dropped the unread-rider diagnostic (want %q); notes seen: %v", want, cloneNoteTexts(h))
	}
	// Decline: no copy, and no unread diagnostic either.
	h2, sark2, dragon2 := cloneOptionalFixture(t)
	ctx2 := &Ctx{Controller: 0, Source: sark2, Remembered: []state.Target{{Obj: dragon2}},
		Clone: "no", CloneDone: true}
	Resolve(h2, ctx2, sa(t, sAVarsSA))
	if h2.g.Obj(sark2).CopyFace != nil {
		t.Fatal("declined clone precondition failed: the decline must not copy")
	}
	if hasNote(h2, want) {
		t.Fatal("declined clone emitted the unread-rider diagnostic")
	}
}

// TestCloneOptionalAcceptedAnswerRegistersDuration verifies an answered
// election keeps the event-driven UntilFacedown lifetime without warning.
func TestCloneOptionalAcceptedAnswerRegistersDuration(t *testing.T) {
	const facedownSA = "DB$ Clone | Defined$ TriggeredCardLKICopy | Optional$ True | Duration$ UntilFacedown"
	h, sark, dragon := cloneOptionalFixture(t)
	ctx := &Ctx{Controller: 0, Source: sark, Remembered: []state.Target{{Obj: dragon}},
		Clone: "yes", CloneDone: true}
	Resolve(h, ctx, sa(t, facedownSA))
	if h.g.Obj(sark).CopyFace == nil {
		t.Fatal("accepted copy precondition failed: no copy basis")
	}
	for _, note := range cloneNoteTexts(h) {
		if strings.Contains(note, "UntilFacedown") {
			t.Fatalf("supported duration emitted a warning: %s", note)
		}
	}
}

func hasNote(h *fakeHost, text string) bool {
	for _, e := range h.log {
		if e.Kind == events.Note && e.Text == text {
			return true
		}
	}
	return false
}

func cloneNoteTexts(h *fakeHost) []string {
	var out []string
	for _, e := range h.log {
		if e.Kind == events.Note {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestCloneOptionalAnswerFieldIsConsumedAndCleared(t *testing.T) {
	h, sark, dragon := cloneOptionalFixture(t)
	ctx := &Ctx{Controller: 0, Source: sark, Remembered: []state.Target{{Obj: dragon}},
		Clone: "no", CloneDone: true}
	Resolve(h, ctx, sa(t, cloneOptionalSA))
	if ctx.Clone != "" || ctx.CloneDone {
		t.Fatal("re-entry left Ctx.Clone/CloneDone set: the answer field must be consumed and cleared")
	}
	if h.g.Obj(sark).CopyFace != nil {
		t.Fatal("the answered decline still cloned")
	}
	// The accepted answer performs the copy.
	h2, sark2, dragon2 := cloneOptionalFixture(t)
	ctx2 := &Ctx{Controller: 0, Source: sark2, Remembered: []state.Target{{Obj: dragon2}},
		Clone: "yes", CloneDone: true}
	Resolve(h2, ctx2, sa(t, cloneOptionalSA))
	if ctx2.Clone != "" || ctx2.CloneDone {
		t.Fatal("accepted re-entry left Ctx.Clone/CloneDone set")
	}
	if f := h2.g.Obj(sark2).Face(); f == nil || f.Name != "Sarkhan, Soul Aflame" {
		t.Fatalf("accepted copy name %v, want the overridden name", f)
	}
}
