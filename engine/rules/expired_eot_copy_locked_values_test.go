package rules

// A copy effect uses the copiable values as they stood when it began
// (CR 707.2, 613.2a): when a later until-end-of-turn copy effect ends, the
// permanent is re-based on the values the surviving permanent-copy layer
// LOCKED when it landed, not on what its source shows now.
//
// The defect this pins had two visible shapes: the permanent fell back to its
// printed card (the clear), or re-based on the permanent marker's source as it
// is now (a fresh source read). The fixture copies an Ox permanently, then the
// Ox becomes a copy of a Bruiser, then a temporary copy of the Bruiser is
// made on top and expires at cleanup: the permanent must be the LOCKED Ox
// (2/3), never the Bruiser (3/3) and never the fixture's printed card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// lockedValuesMimic is the permanent-copy carrier: ability 0 is a permanent
// copy, so the copied face it keeps carries its own (temporary) copy ability.
const lockedValuesMimic = "Name:Fixture Twinmimic\nManaCost:2\nTypes:Artifact\n" +
	"A:AB$ Clone | Cost$ 1 | ValidTgts$ Creature | GainThisAbility$ True | SpellDescription$ becomes a permanent copy.\n" +
	"Oracle:x\n"

// lockedValuesOx is the permanent copy's source. It changes after the
// permanent copy lands: the temporary copy below is one it can make itself.
const lockedValuesOx = "Name:Fixture Ox\nManaCost:2 G\nTypes:Creature Ox\nPT:2/3\n" +
	"A:AB$ Clone | Cost$ 1 | ValidTgts$ Creature | Duration$ UntilEndOfTurn | SpellDescription$ becomes a copy until end of turn.\n" +
	"Oracle:x\n"

func TestExpiredEOTCopyKeepsEarlierCopyValues(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 146, lockedValuesMimic, lockedValuesOx, cloneBruiserSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	ox := moveSeeded(t, e, 0, lockedValuesOx, state.ZBattlefield)
	bruiser := moveSeeded(t, e, 0, cloneBruiserSrc, state.ZBattlefield)

	// The permanent copy: the fixture becomes the Ox and locks its 2/3.
	addMana(t, e, 0, "C")
	activateCloneAbilityIdx(t, e, id, ox, 0)
	obj := e.G.Obj(id)
	if obj == nil || obj.Face() == nil || obj.Face().Name != "Fixture Ox" {
		t.Fatalf("precondition: the permanent copy must be Fixture Ox, got %+v", obj.Face())
	}
	if d := e.Derived(id); d.Power != 2 || d.Toughness != 3 {
		t.Fatalf("precondition: the permanent copy must lock 2/3, got %d/%d", d.Power, d.Toughness)
	}

	// The source changes after the permanent copy began: the Ox is now the
	// Bruiser (3/3). This is the "source as it is now" the re-base must NOT read.
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: ox, IDs: []state.ObjID{bruiser}})
	if f := e.G.Obj(ox).Face(); f == nil || f.Name != "Fixture Bruiser" {
		t.Fatalf("precondition: the source must have changed to Fixture Bruiser, got %v", f)
	}
	if d := e.Derived(id); d.Power != 2 || d.Toughness != 3 {
		t.Fatalf("the source change leaked onto the copy: %d/%d, want the locked 2/3", d.Power, d.Toughness)
	}

	// The temporary copy on top, of the card the source shows now.
	addMana(t, e, 0, "C")
	activateCloneAbilityIdx(t, e, id, bruiser, 0)
	if o := e.G.Obj(id); o.Face() == nil || o.Face().Name != "Fixture Bruiser" {
		t.Fatalf("precondition: the until-end-of-turn copy must be Fixture Bruiser, got %v", o.Face())
	}
	var permanentSeq int32
	for _, ce := range e.continuous {
		if ce.CloneTarget != id || ce.Layer != LCopy {
			continue
		}
		if ce.UntilEOT {
			continue
		}
		permanentSeq = ce.CloneSeq
	}
	if permanentSeq == 0 {
		t.Fatal("precondition: the permanent copy marker must be registered with a layer Seq")
	}
	// The permanent layer really holds the Ox's values where the re-base reads.
	var locked *state.CloneLayer
	for i := range e.G.Obj(id).CloneLayers {
		if l := &e.G.Obj(id).CloneLayers[i]; l.Seq == permanentSeq {
			locked = l
		}
	}
	if locked == nil || locked.Face == nil || locked.Face.Name != "Fixture Ox" {
		t.Fatalf("precondition: the permanent layer must lock Fixture Ox, got %+v", locked)
	}
	replayCheck(t, e, cfg)

	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()

	// The temporary layer is gone; the permanent marker is not.
	var markers int
	for _, ce := range e.continuous {
		if ce.CloneTarget == id && ce.Layer == LCopy {
			markers++
			if ce.UntilEOT {
				t.Fatal("the until-end-of-turn marker survived cleanup")
			}
		}
	}
	if markers != 1 {
		t.Fatalf("after cleanup %d permanent copy markers, want 1", markers)
	}
	obj = e.G.Obj(id)
	if obj == nil || obj.Face() == nil {
		t.Fatal("the settled permanent has no face")
	}
	if obj.Face().Name != "Fixture Ox" {
		t.Fatalf("after cleanup the permanent is %q, want the LOCKED Fixture Ox (not the source as it is now %q, not the printed %q)",
			obj.Face().Name, "Fixture Bruiser", "Fixture Twinmimic")
	}
	if d := e.Derived(id); d.Power != 2 || d.Toughness != 3 {
		t.Fatalf("after cleanup the permanent is %d/%d, want the locked Ox's 2/3", d.Power, d.Toughness)
	}
	replayCheck(t, e, cfg)
}
