package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSurvivorsCanTargetReadsTheMinimum: the candidates a witness's sacrifices
// leave must still make a legal declaration (CR 601.2c). One survivor of a
// "two target creatures" spell is not one: Combo Attack with a Plant and two
// sacrificed Spawns is payable only by paying with a target it announced.
func TestSurvivorsCanTargetReadsTheMinimum(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9417, "Name:Witness Pair\nManaCost:2 G\nTypes:Sorcery\nA:SP$ Pump | ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2 | TgtPrompt$ Select two target creatures\nOracle:x\n")
	o := e.G.Obj(spell)
	if o == nil || o.Face() == nil || o.Face().SpellAbility() == nil {
		t.Fatal("the fixture spell has no spell ability")
	}
	sa := o.Face().SpellAbility()
	if min, dynamic := targetOfferMin(e, 0, spell, sa, 0, false, nil); min != 2 || dynamic {
		t.Fatalf("precondition: the spell's resolved minimum is %d (dynamic %v), want 2", min, dynamic)
	}
	one := []targetCandidate{{kind: "obj", obj: state.ObjID(901)}}
	two := append(one, targetCandidate{kind: "obj", obj: state.ObjID(902)})
	if survivorsCanTarget(e, 0, spell, sa, one) {
		t.Error("one survivor of a two-target spell must not make a legal declaration")
	}
	if !survivorsCanTarget(e, 0, spell, sa, two) {
		t.Error("two survivors of a two-target spell make a legal declaration")
	}
}
