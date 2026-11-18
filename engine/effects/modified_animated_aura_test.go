package effects

// CR 700.9's Aura half reads the attachment's kind as a LAYER-4 type (CR
// 613.1d), not the printed face: a Convulsing Licid its Animate has turned
// into an Aura attached to the bearer is a modification, and its printed
// type line is still "Creature Licid" — only the published layer-4 table
// carries "Aura" for it (RemoveCardTypes$ drops the card types, Types$
// Enchantment,Aura grants the Aura). The board-walk fallback
// modifiedPermanent keeps (no bound table, printed-face read) and the
// opponent's-Aura negative are pinned alongside, so the context-aware read
// cannot drift into the map's legacy body or lose the CR 700.9 controller
// condition.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestModifiedPredicateAnimatedAura pins that an attached permanent whose
// published layer-4 type line carries Aura satisfies `modified` although its
// printed face does not carry the word.
func TestModifiedPredicateAnimatedAura(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears").ID
	licid := corpusObject(t, reg, g, "Convulsing Licid").ID
	placeOnBattlefield(g, bear, 0)
	placeOnBattlefield(g, licid, 0)
	g.Obj(licid).AttachedTo = bear

	// Precondition: the Licid really is attached to the bear and under the
	// bear's controller, and its printed type line has no Aura to fall back
	// to, so a green would be a real layer-4 read.
	if g.Obj(licid).AttachedTo != bear || g.Obj(licid).Controller != g.Obj(bear).Controller {
		t.Fatalf("precondition failed: licid attachedTo=%d controller=%d bear controller=%d",
			g.Obj(licid).AttachedTo, g.Obj(licid).Controller, g.Obj(bear).Controller)
	}
	if !hasType(g.Obj(licid), "Creature") || hasType(g.Obj(licid), "Aura") {
		t.Fatalf("precondition failed: licid printed face has Creature=%t Aura=%t, want true/false",
			hasType(g.Obj(licid), "Creature"), hasType(g.Obj(licid), "Aura"))
	}

	// No bound table: the printed-face fallback, and the Licid is not read as
	// an Aura there.
	if matchesModified(g, bear) {
		t.Fatalf("precondition failed: with no derived-type table bound the Licid must not read as an Aura yet")
	}

	// The state rules publishes at Resolve entry (engine/rules/ctx_new.go):
	// the Licid's Animate in the layer-4 table.
	sc := SpecContext{You: 0}
	sc.Layers.DerivedTypes = []ObjectTypes{{ID: licid, Types: []string{"Enchantment", "Aura"}}}
	if !MatchesObjectCtx(g, "Creature.modified", g.Obj(bear), sc) {
		t.Fatalf("a creature enchanted by a PERMANENT UNDER ITS CONTROLLER whose layer-4 type line is an Aura must satisfy modified (CR 700.9, CR 613.1d)")
	}
}

// TestModifiedPredicateAnimatedAuraOpponentControlled keeps the CR 700.9
// controller condition under the same layer-4 read: an opponent-controlled
// animated Aura is not a modification.
func TestModifiedPredicateAnimatedAuraOpponentControlled(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears").ID
	licid := corpusObject(t, reg, g, "Convulsing Licid").ID
	placeOnBattlefield(g, bear, 0)
	placeOnBattlefield(g, licid, 1)
	g.Obj(licid).Controller = 1
	g.Obj(licid).AttachedTo = bear

	// Precondition: the attached Aura really is the opponent's.
	if g.Obj(licid).Controller == g.Obj(bear).Controller {
		t.Fatalf("precondition failed: licid controller=%d, want the opponent's", g.Obj(licid).Controller)
	}
	sc := SpecContext{You: 0}
	sc.Layers.DerivedTypes = []ObjectTypes{{ID: licid, Types: []string{"Enchantment", "Aura"}}}
	if MatchesObjectCtx(g, "Creature.modified", g.Obj(bear), sc) {
		t.Fatalf("a creature enchanted ONLY by an opponent's layer-4 Aura must not satisfy modified (CR 700.9 controller condition)")
	}
}
