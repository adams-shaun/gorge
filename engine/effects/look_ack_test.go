package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The bare private look's pacing gate: effReveal's two bare arms — NoReveal$ True
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

// TestBareNoRevealLookAckNoHostEmitsImmediately pins the no-host fallback: a
// host that cannot ask emits the look emitted
// immediately, with no decision, deterministically.
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
