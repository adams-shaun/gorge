package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLazavGainedCloneKeepsItsOwnX pins the reverse direction of the
// GainThisAbility$ SVar merge: the gained ability's OWN SVar must win a name
// collision with the copied face. Lazav, the Multifarious and Likeness Looter
// both carry "{X}: become a copy of target creature card in your graveyard
// with mana value X ... it has this ability" with SVar:X = Count$xPaid and no
// AddSVars$. After copying a creature that defines its own SVar:X (a fixed
// count), the offered target filter Creature.YouOwn+cmcEQX must still read
// the announced X, not the copied card's count.
func TestLazavGainedCloneKeepsItsOwnX(t *testing.T) {
	t.Parallel()
	const lazav = "Name:Lazav, the Multifarious\nManaCost:U B\nTypes:Legendary Creature Shapeshifter\nPT:1/3\n" +
		"A:AB$ Clone | Cost$ X | ValidTgts$ Creature.YouOwn+cmcEQX | TgtZone$ Graveyard | TgtPrompt$ Select target creature card in your graveyard | AddTypes$ Legendary | NewName$ Lazav, the Multifarious | GainThisAbility$ True | SpellDescription$ Become a copy.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const newsie = "Name:Snooping Newsie\nManaCost:U B\nTypes:Creature Human Rogue\nPT:2/2\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | AddPower$ 1 | AddToughness$ 1 | AddKeyword$ Lifelink | CheckSVar$ X | SVarCompare$ GE5 | Description$ Pumped with five mana values.\n" +
		"SVar:X:Number$1\nOracle:x\n"
	e := handEngine(t)
	lazavID := onBoardReady(t, e, 0, lazav)
	newsieID := e.G.AddObject(card(t, newsie), 0)
	newsieID.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{newsieID.ID})

	// Lazav copies the Newsie, keeping its own "{X}: Clone" ability
	// (GainThisAbility$ True). The granted ability's X is Count$xPaid; the
	// copied face defines X as a fixed 1.
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: lazavID,
		IDs: []state.ObjID{newsieID.ID}, Counter: "gain-this-ability", Amount: 1,
		Text: "Lazav, the Multifarious"})
	face := e.G.Obj(lazavID).Face()
	if face == nil {
		t.Fatal("precondition: Lazav must have a face after the copy")
	}
	if got := face.SVars["X"]; got != "Count$xPaid" {
		t.Errorf("gained Clone's SVar:X = %q, want Count$xPaid (the copied Newsie body must not shadow the gained ability's own X)", got)
	}
	if got, want := face.Name, "Lazav, the Multifarious"; got != want {
		t.Fatalf("precondition: copy name = %q, want %q", got, want)
	}

	// The graveyard holds only Snooping Newsie (mana value 2). The gained
	// Clone's target is Creature.YouOwn+cmcEQX, so only an announced X of 2
	// can find a target: the ability must stay offered with two mana
	// floating. The copied Newsie body (a fixed 1) would look for a
	// mana-value-1 card and withhold it.
	e.G.Players[0].Pool[state.MC] = 2
	e.pending = nil
	if !xBoundOffered(e, "ability", lazavID) {
		t.Fatalf("gained Clone not offered with X=2 and a mana-value-2 graveyard card: the copied face's X stood in for the announced X (%+v)", e.Pending())
	}
}
