package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// fusedDownDirtyEngine deals seat 0 a 40-card deck led by the corpus split
// card Down // Dirty and seats the opening hands the fused suspension needs:
// seat 0 holds the Down // Dirty in hand plus one Island parked in its
// graveyard (Dirty's "return target card from your graveyard" pick), seat 1
// holds its dealt hand of Mountains (>=3, so Down's NumCards$ 2 Mode$
// TgtChoose discard really asks rather than discarding silently).
func fusedDownDirtyEngine(t *testing.T, reg *cards.Registry, seed uint64) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	down := searchCorpusCard(t, reg, "Down")
	island := searchCorpusCard(t, reg, "Island")
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{down}
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"fuse", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, "Down", state.ZHand)
	// Park one Island in seat 0's graveyard, event-sourced so a log-only
	// replay rebuilds the same board.
	var gyID state.ObjID
	for _, lid := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(lid); o != nil && o.Face() != nil && o.Face().Name == "Island" {
			gyID = lid
			break
		}
	}
	if gyID == 0 {
		t.Fatal("no Island in seat 0's library to park in the graveyard")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: gyID, From: state.ZLibrary, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	return e, cfg, id, gyID
}

// TestFusedDownDirtyAlternateHalfRunsAfterTheDiscardAsk pins the fuse-rest
// continuation: a fused Down // Dirty whose FRONT half suspends mid-resolution
// on its Mode$ TgtChoose discard ask still runs the alternate half (Dirty)
// after the answer -- the targeted graveyard card returns to hand -- instead
// of being dropped behind the "fuse: alternate half not run" Note.
func TestFusedDownDirtyAlternateHalfRunsAfterTheDiscardAsk(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, id, gyID := fusedDownDirtyEngine(t, reg, 8431)

	addMana(t, e, 0, "BBBBBBG")
	fuse := splitOption(t, e, id, "fuse")
	if fuse == nil {
		t.Fatalf("fused offer missing for Down // Dirty: %+v", castOptions(t, e))
	}
	submitChoices(t, e, fuse.Index)

	// Stage 0: Down's player target -- seat 1 (the discarder).
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("fused target stage 0: pending=%+v, want target", d)
	}
	p1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			p1 = o.Index
		}
	}
	if p1 < 0 {
		t.Fatalf("stage 0 offered no player option for seat 1: %+v", d.Options)
	}
	submitChoices(t, e, p1)

	// Stage 1: Dirty's graveyard-card target -- the parked Island.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("fused target stage 1: pending=%+v, want target", d)
	}
	gy := -1
	for _, o := range d.Options {
		if o.Obj == gyID {
			gy = o.Index
		}
	}
	if gy < 0 {
		t.Fatalf("stage 1 offered no option for the parked graveyard card %d: %+v", gyID, d.Options)
	}
	submitChoices(t, e, gy)

	// Pass priority round the table so the fused spell resolves; the front
	// half (Down) then suspends on its discard ask, posed to the target
	// player (seat 1, Mode$ TgtChoose): it chooses which 2 of its >=3 hand
	// cards to discard.
	for i := 0; i < 6; i++ {
		d = e.Pending()
		if d == nil {
			t.Fatal("no pending decision while waiting for the fused resolution")
		}
		if d.Kind != decision.KPriority {
			break
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			t.Fatalf("priority with no pass option: %+v", d.Options)
		}
		submitChoices(t, e, pass)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "discard" || d.Player != 1 {
		t.Fatalf("discard ask pending=%+v, want seat 1's KChoose discard", d)
	}
	if len(d.Options) < 3 {
		t.Fatalf("discard ask offered %d options, want strictly more than NumCards$ 2", len(d.Options))
	}
	discard := []state.ObjID{d.Options[0].Obj, d.Options[1].Obj}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)

	// The answered resume must complete BOTH halves: Dirty returns the
	// parked graveyard card to hand and the resolution leaves the stack.
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty after the answered discard: %+v", e.G.Stack)
	}
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("pending after the answered discard = %+v, want priority", d)
	}
	if z := e.G.Obj(gyID).Zone; z != state.ZHand {
		t.Fatalf("Dirty did not run: the graveyard card's zone = %s, want hand", z)
	}
	if got := e.G.Obj(gyID).Controller; got != 0 {
		t.Fatalf("returned card controller = %d, want 0", got)
	}
	for i, did := range discard {
		if o := e.G.Obj(did); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("discarded card %d (index %d) zone = %v, want graveyard", did, i, o)
		}
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("resolved fused spell zone=%s, want graveyard", z)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "fuse: alternate half not run") {
			t.Fatalf("the alternate half was still dropped: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}
