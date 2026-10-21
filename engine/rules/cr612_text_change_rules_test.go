package rules

// CR 612.1/612.2 and 613.1c (cr-g36a): a text-changing effect changes the
// words of the object's rules text and type line, so its abilities work
// with the new words -- not only its rendered text. Elvish Champion ("Other
// Elf creatures get +1/+1 and have forestwalk") changed Elf -> Vampire pumps
// Vampires, not Elves, and is itself a Vampire; Bog Wraith's Swampwalk
// changed Swamp -> Island is Islandwalk. CR 707.2: the change is not a
// copiable value. CR 400.7: it ends when the permanent leaves.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	textElfBearSrc     = "Name:Elf Bear\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	textVampireBearSrc = "Name:Vampire Bear\nManaCost:B\nTypes:Creature Vampire\nPT:1/1\nOracle:x\n"
)

// textChangeOn resolves `ChangeTypeWord$ <from> <to>` on id through the real
// api:ChangeText body (literal words, no ask).
func textChangeOn(t *testing.T, e *Engine, id state.ObjID, from, to string) {
	t.Helper()
	src := card(t, "Name:Hack\nManaCost:U\nTypes:Instant\nA:SP$ ChangeText | Defined$ Targeted | ChangeTypeWord$ "+
		from+" "+to+" | Duration$ Permanent\nOracle:x\n")
	effects.Resolve(e, &effects.Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}, TargetsOffered: true},
		src.Faces[0].Abilities[0])
}

// TestCR612ChangedTypeWordDrivesTheLordStatic: Elf -> Vampire on Elvish
// Champion moves its +1/+1 from the Elf to the Vampire and makes the
// Champion a Vampire.
func TestCR612ChangedTypeWordDrivesTheLordStatic(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	champ := onBoardCard(t, e, 0, corpusCard(t, "Elvish Champion"))
	elf := onBoard(t, e, 0, textElfBearSrc)
	vamp := onBoard(t, e, 0, textVampireBearSrc)
	if e.Derived(elf).Power != 2 || e.Derived(vamp).Power != 1 {
		t.Fatalf("precondition: Elf %d, Vampire %d power, want 2 and 1", e.Derived(elf).Power, e.Derived(vamp).Power)
	}
	textChangeOn(t, e, champ, "Elf", "Vampire")
	if p := e.Derived(elf).Power; p != 1 {
		t.Fatalf("CR 612.2: after Elf -> Vampire the Champion still pumps the Elf (power %d, want 1)", p)
	}
	if p := e.Derived(vamp).Power; p != 2 {
		t.Fatalf("CR 612.2: after Elf -> Vampire the Champion does not pump the Vampire (power %d, want 2)", p)
	}
	if ty := e.Derived(champ).Types; !slices.Contains(ty, "Vampire") || slices.Contains(ty, "Elf") {
		t.Fatalf("CR 612.1: the Champion's type line = %v, want Vampire in place of Elf", ty)
	}
}

// TestCR612ChangedLandWordRewritesLandwalk: Swamp -> Island on Bog Wraith.
func TestCR612ChangedLandWordRewritesLandwalk(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	wraith := onBoardCard(t, e, 0, corpusCard(t, "Bog Wraith"))
	textChangeOn(t, e, wraith, "Swamp", "Island")
	kw := e.Derived(wraith).Keywords
	if !slices.Contains(kw, "Landwalk:Island") || slices.Contains(kw, "Landwalk:Swamp") {
		t.Fatalf("CR 612.2: Bog Wraith keywords = %v after Swamp -> Island, want Landwalk:Island only", kw)
	}
}

// TestCR612TextChangeIsNotCopiableAndEndsOnLeaving guards CR 707.2 and
// CR 400.7: the copiable face keeps the printed words, and the change is
// gone once the Champion leaves the battlefield.
func TestCR612TextChangeIsNotCopiableAndEndsOnLeaving(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	champ := onBoardCard(t, e, 0, corpusCard(t, "Elvish Champion"))
	textChangeOn(t, e, champ, "Elf", "Vampire")
	o := e.G.Obj(champ)
	if !slices.Contains(o.CopiableFace().Types, "Elf") {
		t.Fatalf("CR 707.2: copiable types = %v, want the printed Elf", o.CopiableFace().Types)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: champ, From: state.ZBattlefield, To: state.ZGraveyard})
	if ty := o.Face().Types; !slices.Contains(ty, "Elf") || slices.Contains(ty, "Vampire") {
		t.Fatalf("CR 400.7: after leaving the battlefield the card's types are %v, want the printed Elf", ty)
	}
}

// TestCR612ChangedSpellTextOnTheStack: "Destroy target black creature" is
// cast at a black creature, then Black -> White changes the spell's text on
// the stack (Sleight of Mind's "target spell or permanent"). On resolution
// its target is no longer legal (CR 608.2b), so the black creature survives.
func TestCR612ChangedSpellTextOnTheStack(t *testing.T) {
	t.Parallel()
	doom := "Name:Doom Bolt\nManaCost:B\nTypes:Instant\nA:SP$ Destroy | ValidTgts$ Creature.Black | TgtPrompt$ Select target black creature\nOracle:Destroy target black creature.\n"
	bear := "Name:Black Bear\nManaCost:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _, id := newFixtureDeckWithOpponentCard(t, 61, doom, "Name:Filler\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n", bear)
	b := moveSeeded(t, e, 1, bear, state.ZBattlefield)
	addMana(t, e, 0, "B")
	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		for _, o := range d.Options {
			if o.Obj == b {
				submitChoices(t, e, o.Index)
			}
		}
	}
	if z := e.G.Obj(id).Zone; z != state.ZStack {
		t.Fatalf("precondition: Doom Bolt in %s, want the stack", z)
	}
	textChangeOn(t, e, id, "Black", "White")
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(b).Zone; z != state.ZBattlefield {
		t.Fatalf("CR 612.1: Black Bear in %s: the text-changed spell (target white creature) still destroyed the black creature", z)
	}
}
