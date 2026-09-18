package effects

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// saWithSubs is sa() plus the SVar bodies a SubAbility$ chain resolves
// through (cards.Link, the same mechanism the compiled corpus uses —
// Ctx.SVars alone does not link a SubAbility$ chain).
func saWithSubs(t *testing.T, line string, svars ...string) *cards.SA {
	t.Helper()
	src := "Name:T\nTypes:Sorcery\nA:" + line
	for _, sv := range svars {
		src += "\nSVar:" + sv
	}
	src += "\nOracle:x\n"
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	return c.Faces[0].Abilities[0]
}

// The bare private look's pacing gate (lookack, task
// fb-20260917T232325Z-35cfca4b): effReveal's two bare arms — NoReveal$ True
// (Mishra's Bauble) and the mandatory Look$ True arm (Gitaxian Probe) — used
// to land their Secret Note with NO decision attached, so a client's
// auto-passing priority streamed the line past before the player could read
// it ("im not sure if other players hand was shown to me... happened way too
// fast"). Both arms now pose a one-option "Continue" KChoose (ResumeKind
// "look_ack") BEFORE the note; the answer re-enters through rules'
// resumeResolution with Ctx.LookAck set together with Ctx.LookAckTarget (the
// decision's ResumeTarget, the index of the Defined$ target that asked), and
// the note lands below the modal. The ask gates only the pacing — there is
// no decline.

// TestBareNoRevealLookPosesTheAckBeforeTheNote pins the NoReveal$ arm's ask
// (Mishra's Bauble's shape): the ack is posed to the looker, names the
// looked-at card and the zone in its prompt, and NO note lands before the
// answer — the chained SubAbility$ (Mishra's slowtrip) must not run before
// the ack either.
func TestBareNoRevealLookPosesTheAckBeforeTheNote(t *testing.T) {
	h, ids := revealBoard(t) // seat 0's library: Bolt on top
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[2]}
	sa := saWithSubs(t, "SP$ PeekAndReveal | Defined$ You | NoReveal$ True | SubAbility$ DBPing",
		"DBPing:DB$ LoseLife | Defined$ You | LifeAmount$ 1")
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatalf("the bare NoReveal$ look posed no ack decision (log %+v)", sh.log)
	}
	d := sh.asked
	if d.Player != 0 {
		t.Fatalf("ack player = %d, want the looker (seat 0)", d.Player)
	}
	if d.Kind != decision.KChoose {
		t.Fatalf("ack kind = %s, want KChoose", d.Kind)
	}
	if d.ResumeKind != "look_ack" {
		t.Fatalf("ResumeKind = %q, want look_ack", d.ResumeKind)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("min/max = %d/%d, want 1/1", d.Min, d.Max)
	}
	if len(d.Options) != 1 || d.Options[0].Kind != "yes" || d.Options[0].Label != "Continue" {
		t.Fatalf("options = %+v, want a single Continue", d.Options)
	}
	if !strings.Contains(d.Prompt, "library") || !strings.Contains(d.Prompt, "Bolt") {
		t.Fatalf("prompt = %q, want it to name the card and the zone", d.Prompt)
	}
	// Ask-first: the look note and the chained sub must not run before the
	// answer.
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a Note landed before the ack: %+v", e)
		}
		if e.Kind == events.LifeChange {
			t.Fatalf("the chained sub ran before the ack: %+v", e)
		}
	}
}

// TestBareNoRevealLookAckAnsweredEmitsExactlyOnce pins the resume: the
// re-entered walk consumes Ctx.LookAck, emits exactly one Secret look Note
// scoped to the looker, poses no second ask, and the chained sub runs after
// it.
func TestBareNoRevealLookAckAnsweredEmitsExactlyOnce(t *testing.T) {
	h, ids := revealBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[2]}
	sa := saWithSubs(t, "SP$ PeekAndReveal | Defined$ You | NoReveal$ True | SubAbility$ DBPing",
		"DBPing:DB$ LoseLife | Defined$ You | LifeAmount$ 1")
	Resolve(sh, ctx, sa) // suspends on the ack
	sh.suspended = false
	sh.asked = nil
	ctx.LookAck = true
	Resolve(sh, ctx, sa) // the resume pass
	if sh.asked != nil {
		t.Fatalf("the resume posed a second ask: %+v", sh.asked)
	}
	notes := secretLookNotes(sh.log)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), sh.log)
	}
	n := notes[0]
	if n.Player != 0 || n.From != state.ZLibrary || !slices.Equal(n.IDs, []state.ObjID{ids[0]}) {
		t.Fatalf("look Note = %+v, want a Secret library look for seat 0 carrying %d", n, ids[0])
	}
	// The chained sub ran AFTER the note, on the resumed walk.
	life := 0
	for _, e := range sh.log {
		if e.Kind == events.LifeChange && e.Player == 0 && e.Amount < 0 {
			life++
		}
	}
	if life != 1 {
		t.Fatalf("%d chained-sub life losses, want exactly 1 after the note", life)
	}
}

// TestBareNoRevealLookAckNoHostEmitsImmediately pins the R-9 fallback: a
// host that cannot ask keeps the pre-gate behaviour — the look emitted
// immediately, no decision, deterministically.
func TestBareNoRevealLookAckNoHostEmitsImmediately(t *testing.T) {
	h, ids := revealBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[2]}
	Resolve(h, ctx, sa(t, "SP$ PeekAndReveal | Defined$ You | NoReveal$ True"))
	notes := secretLookNotes(h.log)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), h.log)
	}
	if notes[0].Player != 0 || !slices.Equal(notes[0].IDs, []state.ObjID{ids[0]}) {
		t.Fatalf("look Note = %+v, want the immediate look for seat 0", notes[0])
	}
}

// TestMandatoryLookAcksTheLooker pins the Look$ True arm's ask (Gitaxian
// Probe's shape): the ack is posed to the LOOKER (the activator, never the
// looked-at player), the prompt names the hand and its cards, and the note
// lands only on the resumed walk.
func TestMandatoryLookAcksTheLooker(t *testing.T) {
	h, hand := revealHandBoard(t, "Bolt", "Bear", "Wrenn", "Snares")
	sh := &suspendHost{fakeHost: *h}
	src := h.g.AddObject(mkCard(t, "Name:Probe\nManaCost:U\nTypes:Sorcery\nOracle:x\n"), 0)
	ctx := &Ctx{Source: src.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	sa := sa(t, "SP$ RevealHand | ValidTgts$ Player | Look$ True")
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatalf("the mandatory Look$ look posed no ack decision (log %+v)", sh.log)
	}
	d := sh.asked
	if d.Player != 0 || d.ResumeKind != "look_ack" || d.Kind != decision.KChoose {
		t.Fatalf("ack = %+v, want a look_ack KChoose for seat 0", d)
	}
	if !strings.Contains(d.Prompt, "hand") || !strings.Contains(d.Prompt, "Bear") {
		t.Fatalf("prompt = %q, want it to name the hand and its cards", d.Prompt)
	}
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a Note landed before the ack: %+v", e)
		}
	}
	sh.suspended = false
	sh.asked = nil
	ctx.LookAck = true
	Resolve(sh, ctx, sa)
	notes := secretLookNotes(sh.log)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), sh.log)
	}
	n := notes[0]
	if n.Player != 0 || n.From != state.ZHand || !slices.Equal(n.IDs, hand) {
		t.Fatalf("look Note = %+v, want a Secret whole-hand look for seat 0", n)
	}
}

// TestMultiTargetBareLookTerminatesWithOneNotePerTarget pins the per-target
// cursor (the DigTarget pattern — the r2 review's CRITICAL fix): a walk with
// TWO bare-look targets (the Case the Joint shape, `Defined$ Player`)
// answers one Continue per target, each decision bound to its own
// ResumeTarget, and the chain TERMINATES with exactly one Secret note per
// target, no duplicated earlier note, no re-ask. Consuming the flag at the
// FIRST bare-look target instead (the r1 shape) left the later target's ack
// unanswered, so every resume re-emitted the earlier notes and re-posed the
// later ack — the walk never terminated.
func TestMultiTargetBareLookTerminatesWithOneNotePerTarget(t *testing.T) {
	h, _, _ := lookBoard(t) // seat 1's hand: Bear, Isle, Bolt; seat 0's library: mountains
	bear0 := h.g.AddObject(mkCard(t, "Name:Bear0\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	bear0.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 0, []state.ObjID{bear0.ID})
	sh := h // lookBoard's askHost captures the posed decision and suspends
	ctx := &Ctx{Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}, {Player: 0, IsPlayer: true}}}
	sa := sa(t, "SP$ RevealHand | ValidTgts$ Player | Look$ True")
	// Pass 1: target 0's ack, bound to ResumeTarget 0; no note yet.
	Resolve(sh, ctx, sa)
	if sh.asked == nil || sh.asked.ResumeKind != "look_ack" || sh.asked.ResumeTarget != 0 {
		t.Fatalf("first pass posed %+v, want a look_ack bound to target 0", sh.asked)
	}
	if len(secretLookNotes(sh.log)) != 0 {
		t.Fatalf("a note landed before the first ack: %+v", sh.log)
	}
	sh.asked = nil
	ctx.LookAck, ctx.LookAckTarget = true, 0
	// Pass 2: target 0 emits exactly its note; target 1 poses its own ack,
	// bound to ResumeTarget 1 — the cursor advanced, not consumed.
	Resolve(sh, ctx, sa)
	notes := secretLookNotes(sh.log)
	if len(notes) != 1 || notes[0].Player != 0 || !slices.Equal(notes[0].IDs, h.g.Zone(state.ZHand, 1)) {
		t.Fatalf("notes after pass 2 = %+v, want exactly seat 1's whole hand", notes)
	}
	if sh.asked == nil || sh.asked.ResumeKind != "look_ack" || sh.asked.ResumeTarget != 1 || sh.asked.Player != 0 {
		t.Fatalf("the second bare look posed %+v, want its own look_ack bound to target 1", sh.asked)
	}
	sh.asked = nil
	ctx.LookAck, ctx.LookAckTarget = true, 1
	// Pass 3: terminates — target 0 is skipped (already emitted on pass 2),
	// target 1 emits, no third ask.
	Resolve(sh, ctx, sa)
	if sh.asked != nil {
		t.Fatalf("the walk re-posed an ack after every target was answered: %+v", sh.asked)
	}
	notes = secretLookNotes(sh.log)
	if len(notes) != 2 {
		t.Fatalf("got %d Secret look notes total (%+v), want exactly one per target", len(notes), sh.log)
	}
	seat1, seat0 := h.g.Zone(state.ZHand, 1), h.g.Zone(state.ZHand, 0)
	if !slices.Equal(notes[0].IDs, seat1) || !slices.Equal(notes[1].IDs, seat0) {
		t.Fatalf("look notes %+v, want one over %v and one over %v", notes, seat1, seat0)
	}
}

// TestLookOptionalConsentCoversTheFollowThrough pins the scope boundary: a
// Look$+Optional$ shape keeps the reveal_optional ask ALONE — a player who
// answered "yes — look" has consented to the follow-through, so the resumed
// walk emits the look WITHOUT a second look_ack gate.
func TestLookOptionalConsentCoversTheFollowThrough(t *testing.T) {
	h, _ := revealBoard(t)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	bear.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 0, []state.ObjID{bear.ID})
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0}
	sa := sa(t, "SP$ RevealHand | Defined$ You | Look$ True | Optional$ True")
	Resolve(sh, ctx, sa)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("first pass posed %+v, want the reveal_optional ask", sh.asked)
	}
	sh.suspended = false
	sh.asked = nil
	ctx.RevealOpt = "yes"
	Resolve(sh, ctx, sa)
	if sh.asked != nil {
		t.Fatalf("the consented look posed a second gate: %+v", sh.asked)
	}
	if len(secretLookNotes(sh.log)) != 1 {
		t.Fatalf("the consented look emitted %+v, want exactly one Secret look note", sh.log)
	}
}
