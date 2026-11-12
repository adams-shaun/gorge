package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CR 801.5b pin on the REAL Fact or Fiction card at three seats: "An
// opponent separates those cards into two piles" is an opponent OF THE
// CASTER'S CHOICE, so before the split the caster is asked which opponent
// separates. At two seats the same cast poses no such ask (the first live
// opponent is the only candidate -- fofTestEngine's byte-identical pin); the
// ask exists only at three or more seats.

// TestFactOrFictionThreeSeatsCasterPicksTheSeparator casts Fact or Fiction
// at three seats and drives the whole resolution: the first pending decision
// is the twopiles_opponent KChoose posed to the caster over the two live
// opponents in seat order, the caster's answer seats the split ask, and the
// chosen pile still lands in the caster's hand with the other in the
// graveyard, replayed byte-identically.
func TestFactOrFictionThreeSeatsCasterPicksTheSeparator(t *testing.T) {
	t.Parallel()
	e, cfg, revealIDs, fofID := fofTestEngineN(t, 3)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_opponent" {
		t.Fatalf("after the cast: %+v, want the CR 801.5b opponent KChoose", d)
	}
	if d.Player != 0 {
		t.Fatalf("opponent ask player = %d, want the caster 0", d.Player)
	}
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 ||
		d.Options[0].Kind != "player" || d.Options[0].Player != 1 ||
		d.Options[1].Kind != "player" || d.Options[1].Player != 2 {
		t.Fatalf("opponent ask = %d..%d options %+v, want 1..1 over players 1 and 2 in seat order",
			d.Min, d.Max, d.Options)
	}
	// The caster picks seat 2 to separate.
	submitChoices(t, e, d.Options[1].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_split" {
		t.Fatalf("after the opponent pick: %+v, want the TwoPiles split KChoose", d)
	}
	if d.Player != 2 {
		t.Fatalf("split ask player = %d, want the chosen separator 2", d.Player)
	}
	if d.Min != 0 || d.Max != 5 || len(d.Options) != 5 {
		t.Fatalf("split bounds/options = %d..%d, %d options, want 0..5 over the five revealed",
			d.Min, d.Max, len(d.Options))
	}
	// Pile A = the first and third revealed cards.
	submitChoices(t, e, d.Options[0].Index, d.Options[2].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_pick" {
		t.Fatalf("after the split: %+v, want the TwoPiles pile pick", d)
	}
	if d.Player != 0 {
		t.Fatalf("pick ask player = %d, want the chooser (Defined$ You) 0", d.Player)
	}
	submitChoices(t, e, d.Options[0].Index) // the caster takes pile A
	for _, id := range []state.ObjID{revealIDs[0], revealIDs[2]} {
		if z := e.G.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("chosen-pile card %d zone = %s, want hand", id, z)
		}
	}
	for _, id := range []state.ObjID{revealIDs[1], revealIDs[3], revealIDs[4]} {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("unchosen-pile card %d zone = %s, want graveyard", id, z)
		}
	}
	if z := e.G.Obj(fofID).Zone; z != state.ZGraveyard {
		t.Fatalf("Fact or Fiction zone = %s, want the graveyard after resolving", z)
	}
	replayCheck(t, e, cfg)
}
