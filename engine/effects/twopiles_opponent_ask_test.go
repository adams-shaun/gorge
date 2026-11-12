package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the CR 801.5b opponent ask in the TwoPiles resolution: at
// three or more seats a Separator$/Chooser$ spec naming the literal
// "Opponent" is the spell controller's CHOICE of opponent, so the caster is
// asked which one before the split (engine/effects/twopiles.go
// twoPilesAskOpponent). At two seats no ask is posed -- the first live
// opponent is the only candidate -- so the two-seat goldens and the
// existing TestTwoPiles* pins stay byte-identical.

// twopilesOppTapeHost serves the mid-resolution asks a TwoPiles resolution
// poses (the CR 801.5b opponent KChoose, the split KChoose and the pile
// pick KChoose) through the ask seam, so an effects-level test can drive
// each answer without an engine. Every other decision falls through to
// fakeHost's deterministic stand-in. served records each ask the seam
// actually answered, in order -- the assertion the ask sequence rides on.
type twopilesOppTapeHost struct {
	*fakeHost
	oppIdx   int   // the opponent-ask option index answered
	splitIdx []int // the split's pile-A picks
	pickIdx  int   // 0 = pile-a, 1 = pile-b
	served   []*decision.Decision
}

func (h *twopilesOppTapeHost) AskCount() uint64 { return uint64(h.askCount) }

func (h *twopilesOppTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.Kind != decision.KChoose || len(d.Options) == 0 {
		return decision.Intent{}, false
	}
	switch d.Options[0].Kind {
	case "player":
		h.served = append(h.served, d)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{h.oppIdx}}, true
	case "twopiles":
		h.served = append(h.served, d)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: append([]int(nil), h.splitIdx...)}, true
	case "pile-a", "pile-b":
		h.served = append(h.served, d)
		return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{h.pickIdx}}, true
	default:
		return decision.Intent{}, false
	}
}

// twopilesOppBoard is twopilesBoard at THREE seats: five cards on seat 0's
// library (ids[0] on top), a source object, and two live opponents for the
// CR 801.5b ask to offer. Returns the host and the six object ids.
func twopilesOppBoard(t *testing.T) (*twopilesOppTapeHost, []state.ObjID) {
	t.Helper()
	h := &twopilesOppTapeHost{fakeHost: newHost(t, 3)}
	var ids []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Source"} {
		c := mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n")
		ids = append(ids, h.g.AddObject(c, 0).ID)
	}
	lib := append([]state.ObjID(nil), ids[:5]...)
	h.g.SetZone(state.ZLibrary, 0, lib)
	for _, id := range ids[:5] {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	return h, ids
}

// TestTwoPilesThreeSeatsChooserOpponentAsked drives `Separator$ You |
// Chooser$ Opponent` at three seats: the asks, in order, are the opponent
// ask to the caster (seat 0, two "player" options -- the live opponents in
// seat order), then the split to the caster (Separator$ You), then the pile
// pick to the CHOSEN opponent (seat 1). The chosen pile lands in the
// caster's hand, the other in the graveyard.
func TestTwoPilesThreeSeatsChooserOpponentAsked(t *testing.T) {
	h, ids := twopilesOppBoard(t)
	for _, id := range ids[:5] {
		if z := h.g.Obj(id).Zone; z != state.ZLibrary {
			t.Fatalf("precondition: pile card %d zone = %d, want the library the split reads", id, z)
		}
	}
	if len(h.g.Players) != 3 {
		t.Fatalf("precondition: %d players, want 3 seats so two opponents are live", len(h.g.Players))
	}
	// The caster picks opponent 1 out of the two candidates.
	h.oppIdx, h.splitIdx, h.pickIdx = 0, []int{0}, 0
	svars := twopilesSVars()
	ctx := NewCtxPtr(ids[5], 0, CtxInit{SVars: svars, Remembered: twopilesRemembered(ids[:5])})
	sa := sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ You | Chooser$ Opponent | ChosenPile$ DBHand | UnchosenPile$ DBGrave")
	Resolve(h, ctx, sa)

	if len(h.served) != 3 {
		t.Fatalf("the seam served %d asks, want 3 (opponent, split, pick): %+v", len(h.served), h.served)
	}
	if k := h.served[0].ResumeKind; k != "twopiles_opponent" {
		t.Fatalf("first ask ResumeKind = %q, want twopiles_opponent", k)
	}
	if p := h.served[0].Player; p != 0 {
		t.Fatalf("opponent ask player = %d, want the caster 0 (CR 801.5b: your choice)", p)
	}
	if h.served[0].Min != 1 || h.served[0].Max != 1 || len(h.served[0].Options) != 2 {
		t.Fatalf("opponent ask bounds/options = %d..%d, %d options, want 1..1 over the two live opponents",
			h.served[0].Min, h.served[0].Max, len(h.served[0].Options))
	}
	for i, want := range []state.PlayerID{1, 2} {
		o := h.served[0].Options[i]
		if o.Kind != "player" || o.Player != want {
			t.Fatalf("opponent ask option %d = kind %q player %d, want kind \"player\" player %d (seat order)",
				i, o.Kind, o.Player, want)
		}
	}
	if k := h.served[1].ResumeKind; k != "twopiles_split" {
		t.Fatalf("second ask ResumeKind = %q, want twopiles_split", k)
	}
	if p := h.served[1].Player; p != 0 {
		t.Fatalf("split ask player = %d, want 0 (Separator$ You)", p)
	}
	if k := h.served[2].ResumeKind; k != "twopiles_pick" {
		t.Fatalf("third ask ResumeKind = %q, want twopiles_pick", k)
	}
	if p := h.served[2].Player; p != 1 {
		t.Fatalf("pick ask player = %d, want 1, the opponent the caster's answer chose", p)
	}
	if z := h.g.Obj(ids[0]).Zone; z != state.ZHand {
		t.Fatalf("pile-A card zone = %d, want hand", z)
	}
	for _, id := range ids[1:5] {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("pile-B card %d zone = %d, want graveyard", id, z)
		}
	}
	for _, e := range h.log {
		if e.Kind == events.Note {
			t.Fatalf("the opponent-ask resolution recorded a Note: %+v", e)
		}
	}
}
