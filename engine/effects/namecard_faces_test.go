package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// multiFaceUniverse is a universe of inline multi-face scripts (never
// committed Forge data): a transforming DFC, an adventurer card, a split
// card and an Alchemy specialize card.
func multiFaceUniverse(t *testing.T) *cards.Universe {
	t.Helper()
	return cards.UniverseOf([]*cards.Card{
		mkCard(t, "Name:Delver Test\nManaCost:U\nTypes:Creature Human Wizard\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n"+
			"ALTERNATE\nName:Aberration Test\nTypes:Creature Human Insect\nPT:3/2\nOracle:x\n"),
		mkCard(t, "Name:Giant Test\nManaCost:2 R\nTypes:Creature Giant\nPT:4/3\nAlternateMode:Adventure\nOracle:x\n"+
			"ALTERNATE\nName:Stomp Test\nManaCost:1 R\nTypes:Instant Adventure\nOracle:x\n"),
		mkCard(t, "Name:Fire Test\nManaCost:1 R\nTypes:Instant\nAlternateMode:Split\nOracle:x\n"+
			"ALTERNATE\nName:Ice Test\nManaCost:1 U\nTypes:Instant\nOracle:x\n"),
		mkCard(t, "Name:Spec Test\nManaCost:2 G\nTypes:Creature Elf\nPT:2/2\nAlternateMode:Specialize\nOracle:x\n"+
			"ALTERNATE\nName:Spec Test White\nManaCost:2 G\nTypes:Creature Elf\nPT:3/3\nOracle:x\n"),
	})
}

// TestNameChoicesOfferEveryNameableFace is CR 201.4b-f / 709.3 / 712.19 /
// 715.5: a player may choose the name of either half of a split card, the
// back face of a double-faced card and an adventurer card's alternative
// name. A specialize face is not a separately nameable card (CR 201.4
// covers no such layout), so it is not offered.
func TestNameChoicesOfferEveryNameableFace(t *testing.T) {
	g := state.NewGame(names(2))
	g.NameUniverse = multiFaceUniverse(t)
	all := NameChoices(g, "", "")
	for _, want := range []string{"Delver Test", "Aberration Test", "Giant Test", "Stomp Test", "Fire Test", "Ice Test", "Spec Test"} {
		if !containsName(all, want) {
			t.Errorf("unrestricted NameChoices = %v, missing %q", all, want)
		}
	}
	if containsName(all, "Spec Test White") {
		t.Errorf("unrestricted NameChoices offered the specialize face: %v", all)
	}
}

// TestNameChoicesFilterEachFaceByItsOwnCharacteristics is CR 201.4b/d/f: a
// name "with certain characteristics" is judged by that face's (that half's)
// characteristics alone.
func TestNameChoicesFilterEachFaceByItsOwnCharacteristics(t *testing.T) {
	g := state.NewGame(names(2))
	g.NameUniverse = multiFaceUniverse(t)
	creature := NameChoices(g, "Card.Creature", "")
	if !containsName(creature, "Aberration Test") || !containsName(creature, "Giant Test") || containsName(creature, "Stomp Test") {
		t.Errorf("Card.Creature names = %v, want the DFC back face and the adventurer, not its Adventure", creature)
	}
	instant := NameChoices(g, "Card.Instant", "")
	if !containsName(instant, "Stomp Test") || containsName(instant, "Giant Test") {
		t.Errorf("Card.Instant names = %v, want the Adventure name and not the creature", instant)
	}
	two := NameChoices(g, "Card.cmcEQ2", "")
	if !containsName(two, "Fire Test") || !containsName(two, "Ice Test") || containsName(two, "Giant Test") {
		t.Errorf("Card.cmcEQ2 names = %v, want both split halves (each half's own mana value, CR 201.4b)", two)
	}
}
