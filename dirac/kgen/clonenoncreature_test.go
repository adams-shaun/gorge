package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// cloneNonCreatureCards are the 21 paper cards the three clone refusal classes
// blocked: an as-enters copy whose template may be a land, an enchantment or a
// planeswalker, an as-enters copy with NewName$, and a resolving Clone on the
// battlefield with NewName$.
var cloneNonCreatureCards = []string{
	"Absorbing Man", "Chameleon, Master of Disguise", "Clever Impersonator", "Copy Enchantment", "Copy Land",
	"Echoing Deeps", "Estrid's Invocation", "Ezri Dax, Newly Joined", "Hulkling, Young Avenger", "Impossible Man",
	"Irma, Part-Time Mutant", "Kimahri, Valiant Guardian", "Lazav, Dimir Mastermind", "Lazav, the Multifarious",
	"Mirrormade", "Sakashima the Impostor", "Sakashima's Protege", "Sarkhan, Soul Aflame", "Sunfrill Imitator",
	"Taskmaster, Mercenary Mimic", "Vesuva",
}

// cloneNonCreatureClass reports whether a refusal reason is one of the three
// classes this change lowers.
func cloneNonCreatureClass(reason string) bool {
	return strings.Contains(reason, "(not a creature or artifact)") ||
		strings.Contains(reason, "ETB copy NewName$") || strings.Contains(reason, "Clone NewName$ on the battlefield")
}

// TestCloneNonCreatureLowers: none of the 21 refuses by one of the three
// classes any more (a card may still refuse for another reason: the on-battlefield
// Clones also carry AddTypes$, SetPower$, TgtZone$ or a triggered Defined$, which stay
// refused), and the as-enters copies lower whole. A NewName$ that names no card of
// the pool refuses by name.
func TestCloneNonCreatureLowers(t *testing.T) {
	cs := lowerNamed(t, cloneNonCreatureCards)
	if len(cs) != len(cloneNonCreatureCards) {
		t.Fatalf("lowered %d cards, want %d", len(cs), len(cloneNonCreatureCards))
	}
	whole := map[string]bool{"Copy Land": true, "Copy Enchantment": true, "Clever Impersonator": true,
		"Mirrormade": true, "Sakashima the Impostor": true, "Chameleon, Master of Disguise": true}
	for _, c := range cs {
		for _, r := range c.Refusals {
			if cloneNonCreatureClass(r.Reason) {
				t.Errorf("%s still refuses by class: %s", c.Name, r.Reason)
			} else if whole[c.Name] {
				t.Errorf("%s refuses: %s", c.Name, r.Reason)
			}
		}
	}
	const probe = "Name:Name Probe\nManaCost:2 U\nTypes:Creature Shapeshifter\nPT:0/0\n" +
		"K:ETBReplacement:Copy:" + "DBCopy:Optional\n" +
		"SVar:DBCopy:DB$ Clone | Choices$ Creature.Other | NewName$ Someone Else Entirely\n"
	got := lowerScript(t, "Name Probe", probe)
	if len(got) == 0 || !strings.Contains(strings.Join(got, "|"), "Someone Else Entirely") {
		t.Errorf("a NewName$ outside the pool: refusals %v, want one naming it", got)
	}
}

// TestCloneNewNameUsesTheBuilderTable: a NewName$ resolves only through the
// name table the builder keys cardIdx by (the pool's lowered cards), not every
// name byName's Lookup accepts. A name only byName resolves refuses by name
// instead of passing lowering and failing the build.
func TestCloneNewNameUsesTheBuilderTable(t *testing.T) {
	front := &cards.Card{Faces: []*cards.Face{{Name: "Front"}}}
	other := &cards.Card{Faces: []*cards.Face{{Name: "Other"}}}
	p := &pool{byName: map[string]*cards.Card{"Front": front, "Other": other}, names: []string{"Front"}}
	lc := &lowerCtx{p: p}
	var exc lOverride
	has := false
	if err := lc.cloneNewName(newParams(map[string]string{"NewName": "Other"}), &exc, &has, "probe"); err == nil {
		t.Fatal("a name outside the builder's table lowered")
	} else if !strings.Contains(err.Error(), "Other") {
		t.Fatalf("refusal %q does not name the card", err)
	}
	if err := lc.cloneNewName(newParams(map[string]string{"NewName": "Front"}), &exc, &has, "probe"); err != nil {
		t.Fatalf("a table name refused: %v", err)
	}
	if exc.NameOf != "Front" || !has {
		t.Fatalf("exception %+v has=%v, want NameOf Front", exc, has)
	}
}
