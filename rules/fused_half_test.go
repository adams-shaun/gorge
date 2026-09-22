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

// fusedFarAwayEngine deals seat 0 a 40-card deck led by the corpus split card
// Far // Away and puts two Grizzly Bears on seat 1's battlefield: one is Far's
// return-to-hand target, and the pair makes Away's sacrifice ask a real choice
// (the strict-supersets no-ask shape would never suspend).
func fusedFarAwayEngine(t *testing.T, reg *cards.Registry, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	farAway := searchCorpusCard(t, reg, "Far")
	island := searchCorpusCard(t, reg, "Island")
	mountain := searchCorpusCard(t, reg, "Mountain")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := []*cards.Card{farAway, bear}
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	opp := []*cards.Card{bear, bear}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"fuse", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, "Far", state.ZHand)
	return e, cfg, id
}

// TestFusedFarAwayAlternateHalfSuspensionAsksOnlyTheSacrifice pins the
// last-half suspension tail: a fused Far // Away whose ALTERNATE half (Away)
// suspends on its CR 701.21a sacrifice ask completes after that one ask --
// the spell leaves the stack, the chosen creature is sacrificed and Far's
// creature is in its owner's hand -- with NO spurious "Choose target"
// (ResumeKind "tgts") re-posed between the sacrifice answer and the
// completion, which the generic completion chain's fresh ctx used to trigger
// by re-deriving the object's flat target list for the re-entered half.
func TestFusedFarAwayAlternateHalfSuspensionAsksOnlyTheSacrifice(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, id := fusedFarAwayEngine(t, reg, 9412)

	bearA := splitMoveFromHandOrLibrary(t, e, 0, "Grizzly Bears")
	bearB := splitMoveFromHandOrLibrary(t, e, 1, "Grizzly Bears")
	bearC := splitMoveFromHandOrLibrary(t, e, 1, "Grizzly Bears")
	addMana(t, e, 0, "UUUUB")
	fuse := splitOption(t, e, id, "fuse")
	if fuse == nil {
		t.Fatalf("fused offer missing for Far // Away: %+v", castOptions(t, e))
	}
	submitChoices(t, e, fuse.Index)

	// Stage 0: Far's creature target -- seat 0's own bearA (returns to its
	// owner's hand), so both of seat 1's bears stay eligible when Away's
	// sacrifice ask runs and the ask is a real choice.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("fused target stage 0: pending=%+v, want target", d)
	}
	far := -1
	for _, o := range d.Options {
		if o.Obj == bearA {
			far = o.Index
		}
	}
	if far < 0 {
		t.Fatalf("stage 0 offered no option for bear %d: %+v", bearA, d.Options)
	}
	submitChoices(t, e, far)

	// Stage 1: Away's player target -- seat 1 (the sacrificer).
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("fused target stage 1: pending=%+v, want target", d)
	}
	p1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			p1 = o.Index
		}
	}
	if p1 < 0 {
		t.Fatalf("stage 1 offered no player option for seat 1: %+v", d.Options)
	}
	submitChoices(t, e, p1)

	// Pass priority round the table so the fused spell resolves; the front
	// half (Far) returns bearA to hand, and the alternate half (Away)
	// suspends on its sacrifice ask posed to seat 1.
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
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "sacrifice" || d.Player != 1 {
		t.Fatalf("sacrifice ask pending=%+v, want seat 1's KChoose sacrifice", d)
	}
	sac := -1
	for _, o := range d.Options {
		if o.Obj == bearB {
			sac = o.Index
		}
	}
	if sac < 0 {
		t.Fatalf("sacrifice ask offered no option for bear %d: %+v", bearB, d.Options)
	}
	_ = bearC
	submitChoices(t, e, sac)

	// The answered sacrifice must complete the fused spell with NO further
	// ask: the next pending decision is a priority (not the spurious
	// "Choose target" the generic completion used to pose).
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("pending after the answered sacrifice = %+v (%s), want priority",
			d, d.ResumeKind)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty after the answered sacrifice: %+v", e.G.Stack)
	}
	if z := e.G.Obj(bearA).Zone; z != state.ZHand {
		t.Fatalf("Far did not run: bearA's zone = %s, want hand", z)
	}
	if z := e.G.Obj(bearB).Zone; z != state.ZGraveyard {
		t.Fatalf("Away did not run: bearB's zone = %s, want graveyard", z)
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("resolved fused spell zone=%s, want graveyard", z)
	}
	replayCheck(t, e, cfg)
}

// splitMoveFromHandOrLibrary event-sources a card from player p's hand or
// library onto p's battlefield (the opening deal may hold the corpus card),
// the splitMoveFromLibrary shape widened for a seat whose deck is not all
// one basic.
func splitMoveFromHandOrLibrary(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, from := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(from, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: state.ZBattlefield})
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus card %q absent from seat %d's hand or library", name, p)
	return 0
}
