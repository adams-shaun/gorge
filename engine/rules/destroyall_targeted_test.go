package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// destroyAllBoard seats Awaken the Sleeper in seat 0's hand and an opposing
// Grizzly Bears wearing Eater of Virtue, plus a second Bears wearing a second
// Eater of Virtue (the equipment the destroy must leave alone). withEquip
// false leaves the stolen Bears bare.
type destroyAllBoard struct {
	e                                *Engine
	cfg                              Config
	awaken, bears, eater, otherEater state.ObjID
}

func newDestroyAllBoard(t *testing.T, withEquip bool) destroyAllBoard {
	t.Helper()
	awaken := corpusCard(t, "Awaken the Sleeper")
	bears := corpusCard(t, "Grizzly Bears")
	eater := corpusCard(t, "Eater of Virtue")
	// Eater of Virtue is legendary, so the bystander Equipment is another card.
	splitter := corpusCard(t, "Bonesplitter")
	e, cfg := tokenReplGameSeats(t, 5,
		[]*cards.Card{awaken, eater, splitter},
		[]*cards.Card{bears, bears})
	var b destroyAllBoard
	b.e, b.cfg = e, cfg
	toMain1(t, e)
	b.awaken = findAndMoveToHand(t, e, 0, "Awaken the Sleeper")
	b.bears = moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	otherBears := moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	b.eater = moveSeededCard(t, e, 0, eater, state.ZBattlefield)
	b.otherEater = moveSeededCard(t, e, 0, splitter, state.ZBattlefield)
	if withEquip {
		e.emit(events.Event{Kind: events.Attach, Obj: b.eater, IDs: []state.ObjID{b.bears}})
	}
	e.emit(events.Event{Kind: events.Attach, Obj: b.otherEater, IDs: []state.ObjID{otherBears}})
	if o := e.G.Obj(b.otherEater); o.AttachedTo != otherBears {
		t.Fatalf("precondition: second Equipment attached to %d, want %d", o.AttachedTo, otherBears)
	}
	if withEquip && e.G.Obj(b.eater).AttachedTo != b.bears {
		t.Fatalf("precondition: Equipment not attached to the stolen Bears")
	}
	e.pending = nil
	e.priorityRound()
	return b
}

// castAwaken casts Awaken the Sleeper on the stolen Bears and returns the
// decision the resolution next suspends on (nil if the stack drained).
func (b destroyAllBoard) castAwaken(t *testing.T) *decision.Decision {
	t.Helper()
	e := b.e
	addMana(t, e, 0, "RRRRR")
	castFromPriority(t, e, b.awaken)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the target ask, got %+v", d)
	}
	answerKTarget(t, e, b.bears)
	d = passUntilNonPriority(t, e, 20)
	if d != nil && d.Kind == decision.KChoose && d.ResumeKind == "destroyall_optional" {
		return d
	}
	return nil
}

func (b destroyAllBoard) answer(t *testing.T, d *decision.Decision, kind string) {
	t.Helper()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == kind {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no %q option in %+v", kind, d.Options)
	}
	submitChoices(t, b.e, idx)
	passUntilStackEmpty(t, b.e, 20)
}

func TestAwakenTheSleeperMayDestroyAttachedEquipment(t *testing.T) {
	t.Parallel()
	b := newDestroyAllBoard(t, true)
	d := b.castAwaken(t)
	if d == nil {
		t.Fatalf("no destroy-all yes/no ask was posed; pending %+v", b.e.Pending())
	}
	b.answer(t, d, "yes")
	if z := b.e.G.Obj(b.eater).Zone; z != state.ZGraveyard {
		t.Errorf("the stolen Bears' Equipment is in %s, want graveyard", z)
	}
	if z := b.e.G.Obj(b.otherEater).Zone; z != state.ZBattlefield {
		t.Errorf("the other creature's Equipment is in %s, want battlefield", z)
	}
	if o := b.e.G.Obj(b.bears); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Errorf("stolen Bears zone %s controller %d, want battlefield under seat 0", o.Zone, o.Controller)
	}
	replayCheck(t, b.e, b.cfg)
}

func TestAwakenTheSleeperDeclineKeepsEquipment(t *testing.T) {
	t.Parallel()
	b := newDestroyAllBoard(t, true)
	d := b.castAwaken(t)
	if d == nil {
		t.Fatalf("no destroy-all yes/no ask was posed; pending %+v", b.e.Pending())
	}
	b.answer(t, d, "no")
	for _, id := range []state.ObjID{b.eater, b.otherEater} {
		if z := b.e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Errorf("Equipment %d is in %s after declining, want battlefield", id, z)
		}
	}
	if b.e.G.Obj(b.bears).Controller != 0 {
		t.Errorf("the GainControl before the declined destroy did not take effect")
	}
	replayCheck(t, b.e, b.cfg)
}

func TestAwakenTheSleeperUnequippedAsksNothing(t *testing.T) {
	t.Parallel()
	b := newDestroyAllBoard(t, false)
	if d := b.castAwaken(t); d != nil {
		t.Fatalf("an unequipped target posed a yes/no ask: %+v", d)
	}
	if b.e.G.Obj(b.bears).Controller != 0 {
		t.Errorf("Awaken the Sleeper did not take control of the Bears")
	}
	if z := b.e.G.Obj(b.otherEater).Zone; z != state.ZBattlefield {
		t.Errorf("the other creature's Equipment is in %s, want battlefield", z)
	}
	replayCheck(t, b.e, b.cfg)
}

func TestMaelstromPulseUnchanged(t *testing.T) {
	t.Parallel()
	pulse := corpusCard(t, "Maelstrom Pulse")
	bears := corpusCard(t, "Grizzly Bears")
	e, cfg := tokenReplGameSeats(t, 7, []*cards.Card{pulse}, []*cards.Card{bears, bears, bears})
	toMain1(t, e)
	pulseID := findAndMoveToHand(t, e, 0, "Maelstrom Pulse")
	first := moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	second := moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	third := moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	addMana(t, e, 0, "BBBGG")
	castSpellAtTarget(t, e, pulseID, first)
	for _, id := range []state.ObjID{first, second, third} {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Errorf("Bears %d is in %s, want graveyard (target and same-name permanents die)", id, z)
		}
	}
	replayCheck(t, e, cfg)
}
