package rules

// A token created as a copy of a permanent that was itself a copy takes the
// copied values as its own copiable values (CR 707.2, 707.3): when a later
// copy effect on the token ends, the token is that copy again, not the
// printed card it was minted on.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestTokenCopyOfACopyRevertsToItsOwnBasis(t *testing.T) {
	t.Parallel()
	const heron = "Name:Fixture Heron\nManaCost:1 U\nTypes:Creature Bird\nPT:1/1\nOracle:x\n"
	const stag = "Name:Fixture Stag\nManaCost:2 G\nTypes:Creature Elk\nPT:4/2\n" +
		"A:AB$ Clone | Cost$ 1 | ValidTgts$ Creature | Duration$ UntilEndOfTurn | SpellDescription$ this becomes a copy of the target until end of turn.\nOracle:x\n"
	const mole = "Name:Fixture Mole\nManaCost:1 B\nTypes:Creature Mole\nPT:2/5\nOracle:x\n"
	e, cfg, heronID := newFixtureDeck(t, 707, heron, stag, mole)
	e.emit(events.Event{Kind: events.MoveZone, Obj: heronID, From: state.ZHand, To: state.ZBattlefield})
	stagID := moveSeeded(t, e, 0, stag, state.ZBattlefield)
	moleID := moveSeeded(t, e, 0, mole, state.ZBattlefield)

	// The Heron is a copy of the Stag; a token copy of the Heron is a Stag
	// minted on the Heron's card.
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: heronID, IDs: []state.ObjID{stagID}, Player: 0})
	tok := e.G.NextID
	e.emit(events.Event{Kind: events.CopyToken, Obj: heronID, Player: 0})
	if e.G.Obj(tok) == nil {
		t.Fatal("no token copy minted")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZLibrary, To: state.ZBattlefield})
	if f := e.G.Obj(tok).Face(); f == nil || f.Name != "Fixture Stag" {
		t.Fatalf("token copy is %v, want Fixture Stag", f)
	}

	// The token's own (copied) ability makes it a Mole until end of turn.
	addMana(t, e, 0, "C")
	activateCloneAbilityIdx(t, e, tok, moleID, 0)
	if f := e.G.Obj(tok).Face(); f == nil || f.Name != "Fixture Mole" {
		t.Fatalf("token is %v after its copy ability, want Fixture Mole", f)
	}
	replayCheck(t, e, cfg)

	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()

	if f := e.G.Obj(tok).Face(); f == nil || f.Name != "Fixture Stag" {
		t.Fatalf("after cleanup the token is %v, want its own copy Fixture Stag", f)
	}
	if d := e.Derived(tok); d.Power != 4 || d.Toughness != 2 {
		t.Fatalf("after cleanup P/T %d/%d, want the Stag's 4/2", d.Power, d.Toughness)
	}
	replayCheck(t, e, cfg)
}
