package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// revealBoard builds a 2-seat game with a source object for seat 0, an
// instant and a land on seat 0's library (instant on top), and returns the
// host plus the object ids (instant first, then the land, then the source).
func revealBoard(t *testing.T) (*fakeHost, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	bolt := mkCard(t, "Name:Bolt\nTypes:Instant\nOracle:3 damage\n")
	mtn := mkCard(t, "Name:Mountain\nTypes:Land\nOracle:x\n")
	fix := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	var ids []state.ObjID
	ids = append(ids, h.g.AddObject(bolt, 0).ID)
	ids = append(ids, h.g.AddObject(mtn, 0).ID)
	ids = append(ids, h.g.AddObject(fix, 0).ID)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{ids[0], ids[1]})
	for _, id := range ids[:2] {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	return h, ids
}

// TestRevealOptionalPeekPosesTheYesNoAsk pins the ask itself: a
// RevealOptional$ peek poses a KChoose yes/no to the peeking player with
// ResumeKind "reveal_optional" and SUSPENDS, so a chained SubAbility$ does
// not run before the answer (task fb-20260914T033246Z-3f1cc033, defect 1).
func TestRevealOptionalPeekPosesTheYesNoAsk(t *testing.T) {
	h, ids := revealBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: 3}
	sa := sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("a RevealOptional$ peek posed no decision")
	}
	if sh.asked.Player != 0 {
		t.Fatalf("decider = seat %d, want the peeking player 0", sh.asked.Player)
	}
	if sh.asked.Kind != decision.KChoose {
		t.Fatalf("kind = %s, want KChoose", sh.asked.Kind)
	}
	if sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("ResumeKind = %q, want reveal_optional", sh.asked.ResumeKind)
	}
	if len(sh.asked.Options) != 2 || sh.asked.Options[0].Kind != "yes" || sh.asked.Options[1].Kind != "no" {
		t.Fatalf("options = %+v, want yes then no", sh.asked.Options)
	}
	// The ask carries WHAT would be revealed (round-2 finding 1): the
	// peeking player is deciding over a card only they can see and the
	// library is not projected to their seat, so the name goes into the
	// prompt and the yes option's label, and the card rides the option's
	// Obj — the same private channel the hidden-library "search" options
	// use.
	if sh.asked.Options[0].Label != "Yes — reveal Bolt" || sh.asked.Options[0].Obj != ids[0] {
		t.Fatalf("yes option = %+v, want label naming the top card and its Obj", sh.asked.Options[0])
	}
	if sh.asked.Prompt == "" || !strings.Contains(sh.asked.Prompt, "Bolt") {
		t.Fatalf("prompt = %q, want it to name the top card", sh.asked.Prompt)
	}
	// Suspended means the chain stopped: the sub-ability never ran on the
	// first pass.
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a Note was emitted before the answer: %+v", e)
		}
	}
}

// TestRevealOptionalAnsweredYesRevealsAndRemembers pins the resume: "yes"
// emits the Note (no text of its own — view.Describe renders the ids) and
// appends the revealed cards to Ctx.Remembered for the chained condition
// gate, WITHOUT corrupting the aliased stack-object Remembered slice the
// resume rebuilt the Ctx from.
func TestRevealOptionalAnsweredYesRevealsAndRemembers(t *testing.T) {
	h, ids := revealBoard(t)
	sh := &suspendHost{fakeHost: *h}
	// Deliberate spare capacity: an in-place append would write ids[0]'s
	// Target past len into the shared backing array — the corruption this
	// pins against.
	remembered := make([]state.Target, 1, 8)
	remembered[0] = state.Target{Obj: ids[2]} // the source (a creature)
	orig := append([]state.Target(nil), remembered...)
	backing := remembered[:cap(remembered)] // the full backing array, len included
	ctx := &Ctx{Controller: 0, Source: ids[2], Remembered: remembered}
	sa := sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	Resolve(sh, ctx, sa) // suspends on the ask
	sh.suspended = false
	sh.asked = nil
	ctx.RevealOpt = "yes"
	Resolve(sh, ctx, sa) // the resume pass

	var note *events.Event
	for i := range sh.log {
		e := sh.log[i]
		if e.Kind == events.Note && len(e.IDs) == 1 && e.IDs[0] == ids[0] {
			cp := e
			note = &cp
		}
	}
	if note == nil {
		t.Fatalf("no reveal Note carrying the revealed card: %+v", sh.log)
	}
	if note.Text != "" {
		t.Fatalf("reveal Note Text = %q, want \"\" (Describe renders the ids)", note.Text)
	}
	if note.Secret {
		t.Fatal("a reveal must be public")
	}
	if len(ctx.Remembered) != 2 || ctx.Remembered[1].Obj != ids[0] {
		t.Fatalf("Remembered = %+v, want the original entry plus the revealed %d", ctx.Remembered, ids[0])
	}
	// The aliasing guard: nothing was written past the original length.
	if remembered[0] != orig[0] {
		t.Fatalf("the original Remembered entry was mutated: %+v", remembered[0])
	}
	for i, t2 := range backing[len(orig):] {
		if t2 != (state.Target{}) {
			t.Fatalf("the backing array was written past len at +%d: %+v", i+1, t2)
		}
	}
}

// TestRevealOptionalAnsweredNoRevealsNothing pins the decline: no Note and
// RememberRevealed$ finds nothing, so a chained gate does not fire.
func TestRevealOptionalAnsweredNoRevealsNothing(t *testing.T) {
	h, ids := revealBoard(t)
	sh := &suspendHost{fakeHost: *h}
	remembered := []state.Target{{Obj: ids[2]}}
	ctx := &Ctx{Controller: 0, Source: ids[2], Remembered: remembered}
	sa := sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	Resolve(sh, ctx, sa)
	sh.suspended = false
	ctx.RevealOpt = "no"
	Resolve(sh, ctx, sa)
	for _, e := range sh.log {
		if e.Kind == events.Note && len(e.IDs) > 0 {
			t.Fatalf("a declined reveal emitted a Note: %+v", e)
		}
	}
	if len(ctx.Remembered) != 1 {
		t.Fatalf("Remembered = %+v, want the declined reveal to remember nothing new", ctx.Remembered)
	}
}

// TestRevealOptionalNoHostKeepsTheMandatoryReveal pins the R-9 fallback: a
// host that cannot ask keeps the pre-ask behaviour — the mandatory reveal,
// RememberRevealed$ fired — deterministically.
func TestRevealOptionalNoHostKeepsTheMandatoryReveal(t *testing.T) {
	h, ids := revealBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[2], Remembered: []state.Target{{Obj: ids[2]}}}
	sa := sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	Resolve(h, ctx, sa)
	found := false
	for _, e := range h.log {
		if e.Kind == events.Note && len(e.IDs) == 1 && e.IDs[0] == ids[0] {
			found = true
		}
	}
	if !found {
		t.Fatal("the no-host fallback did not reveal")
	}
	if len(ctx.Remembered) != 2 || ctx.Remembered[1].Obj != ids[0] {
		t.Fatalf("Remembered = %+v, want the fallback reveal remembered", ctx.Remembered)
	}
}

// TestRevealOptionalIsOnlyThePeekShape pins the scope boundary:
// RevealOptional$ on the non-peek shapes poses no ask (mandatory reveal,
// today's behaviour), and a plain PeekAndReveal without RevealOptional$
// never asks either.
func TestRevealOptionalIsOnlyThePeekShape(t *testing.T) {
	h, ids := revealBoard(t)
	// Reveal$/RevealHand look at the HAND, not the library: give seat 0 a
	// hand card to reveal (hand-of-bear, id 4).
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 0, []state.ObjID{bear.ID})
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[2]}
	Resolve(sh, ctx, sa(t, "SP$ Reveal | Defined$ You | NumCards$ 1 | RevealOptional$ True"))
	if sh.asked != nil {
		t.Fatal("Reveal$ posed a RevealOptional$ ask — out of scope shape")
	}
	found := false
	for _, e := range sh.log {
		if e.Kind == events.Note && len(e.IDs) == 1 && e.IDs[0] == bear.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("Reveal$ did not reveal")
	}

	sh2 := &suspendHost{fakeHost: *h}
	Resolve(sh2, &Ctx{Controller: 0, Source: ids[2]},
		sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1"))
	if sh2.asked != nil {
		t.Fatal("a PeekAndReveal without RevealOptional$ posed an ask")
	}
}
