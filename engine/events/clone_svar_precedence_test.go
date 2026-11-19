package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestClonePermanentCopyBasisSVarWinsOverPriorCopy pins the precedence of the
// GainThisAbility$ SVar merge (CR 707.2, 707.3): a recurring Clone carrier's
// current face is already a copy, so its SVar table carries the previous
// copy's entries. Merging that table over the newly copied face let a stale
// same-named SVar shadow the copy's own value. Measured on tail-34 pair 5:
// Cryptoplasm (a copy of Snooping Newsie) copied Xerex Strobe-Knight and the
// copy's "activate only if you've cast two or more spells this turn" gate read
// Snooping Newsie's SVar:X instead of Xerex's, offering the {T} token ability
// at zero spells cast.
func TestClonePermanentCopyBasisSVarWinsOverPriorCopy(t *testing.T) {
	g := state.NewGame([]string{"Ann", "Bob"})
	// The become object: a printed recurring-copy trigger whose body needs
	// SVar CopyBody, exactly Cryptoplasm's shape.
	become, _ := cards.ParseBytes("become.txt", []byte("Name:Recurring Shape\nManaCost:1 U\nTypes:Creature Shapeshifter\nPT:2/2\n"+
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | Execute$ CopyBody | OptionalDecider$ You\n"+
		"SVar:CopyBody:DB$ Clone | ValidTgts$ Creature.Other | Optional$ True | GainThisAbility$ True | AddSVars$ CopyBody\n"+
		"Oracle:x\n"))
	become.Link()
	// The first copy source defines X with a body of its own.
	first, _ := cards.ParseBytes("first.txt", []byte("Name:Newsie Shape\nManaCost:U B\nTypes:Creature Human Rogue\nPT:2/2\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigFixture\n"+
		"SVar:TrigFixture:DB$ Mill | NumCards$ 7 | Defined$ You\n"+
		"SVar:X:Number$1\nOracle:x\n"))
	first.Link()
	// The second copy source defines the SAME name with a different body.
	second, _ := cards.ParseBytes("second.txt", []byte("Name:Knight Shape\nManaCost:2 U\nTypes:Creature Human Knight\nPT:2/2\n"+
		"A:AB$ GainLife | Cost$ T | CheckSVar$ X | SVarCompare$ GE2 | LifeAmount$ 1\n"+
		"SVar:X:Number$2\nOracle:x\n"))
	second.Link()

	firstID := g.AddObject(first, 0).ID
	secondID := g.AddObject(second, 0).ID
	becomeID := g.AddObject(become, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{firstID, secondID, becomeID})
	for _, id := range []state.ObjID{firstID, secondID, becomeID} {
		g.Obj(id).Zone = state.ZBattlefield
	}
	if len(become.Faces[0].Triggers) != 1 || first.Faces[0].SVars["X"] == second.Faces[0].SVars["X"] {
		t.Fatalf("precondition: the two copy sources must define different SVar:X bodies, got %q and %q",
			first.Faces[0].SVars["X"], second.Faces[0].SVars["X"])
	}

	// First copy: the become object becomes a copy of first, keeping its own
	// trigger. Its face now carries first's X beside the become's CopyBody.
	Apply(g, Event{Kind: ClonePermanent, Obj: becomeID, IDs: []state.ObjID{firstID}, Counter: "gain-this-trigger", Amount: 1})
	if got := g.Obj(becomeID).Face().SVars["X"]; got != first.Faces[0].SVars["X"] {
		t.Fatalf("precondition: the first copy's SVar:X = %q, want the first source's %q", got, first.Faces[0].SVars["X"])
	}
	if got, want := len(g.Obj(becomeID).Face().Triggers), 1+len(first.Faces[0].Triggers); got != want {
		t.Fatalf("precondition: the first copy must carry the first source's trigger beside the recurring one, got %d want %d", got, want)
	}

	// Second copy: the become object (still carrying the prior copy's X)
	// becomes a copy of second. The new copy's own X must win.
	Apply(g, Event{Kind: ClonePermanent, Obj: becomeID, IDs: []state.ObjID{secondID}, Counter: "gain-this-trigger", Amount: 2})
	face := g.Obj(becomeID).Face()
	if face == nil {
		t.Fatal("precondition: the become object must have a face after the second copy")
	}
	if got, want := face.SVars["X"], second.Faces[0].SVars["X"]; got != want {
		t.Fatalf("second copy's SVar:X = %q, want the copied face's %q (a prior copy's table must not shadow it)", got, want)
	}
	if face.SVars["CopyBody"] == "" {
		t.Fatal("the gained trigger's own SVar must still merge onto the copy")
	}
}
