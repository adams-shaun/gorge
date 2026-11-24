package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCompiledPredicateSpellBaseHonoursAsStack: the compiled sidecar must not
// answer a definite No for a Spell base the textual matcher accepts under the
// derived AsStack override. A printed `ValidAfterStack$ Spell.<...>` may-play
// grant (Glarb, Calamity's Augur's `Spell.cmcGE4`; Haakon, Serra Paragon,
// Lurrus, Zask, Thundermane Dragon) evaluates the card still in its origin
// zone as the spell it would become, so a sidecar that answers "not on the
// stack" refused every such grant in a real game while a hand-built engine,
// which has no sidecar, offered it. The precondition asserts the object is
// not on the stack, and that without AsStack the sidecar still says No.
func TestCompiledPredicateSpellBaseHonoursAsStack(t *testing.T) {
	g, ids := board(t)
	bear := g.Obj(ids["myBear"])
	if bear.Zone.String() == "stack" {
		t.Fatalf("precondition: myBear is on the stack")
	}
	ps := CompilePredicatePrograms([]string{"Spell", "Spell.YouCtrl", "SpellAbility"})
	for _, spec := range []string{"Spell", "Spell.YouCtrl"} {
		if got := ps.Evaluate(spec, g, bear, SpecContext{You: 0, Source: ids["myBear"]}); got != PredicateNo {
			t.Fatalf("Evaluate(%q) off the stack without AsStack = %v, want PredicateNo", spec, got)
		}
		got := ps.Evaluate(spec, g, bear, SpecContext{You: 0, Source: ids["myBear"], AsStack: true})
		if got != PredicateYes {
			t.Errorf("Evaluate(%q) with AsStack = %v, want PredicateYes", spec, got)
		}
		if !MatchesSpecCtx(g, spec, ids["myBear"], SpecContext{You: 0, Source: ids["myBear"], AsStack: true}) {
			t.Errorf("textual MatchesSpecCtx(%q) with AsStack = false: the sidecar's Yes has no oracle", spec)
		}
	}
	// SpellAbility is an ability on the stack, never a derived spell.
	if got := ps.Evaluate("SpellAbility", g, bear, SpecContext{You: 0, Source: ids["myBear"], AsStack: true}); got != PredicateNo {
		t.Errorf("Evaluate(SpellAbility) with AsStack = %v, want PredicateNo", got)
	}
}

// TestSpellBaseAgreesAcrossTextualAndCompiledArms: CR 112.1 makes a spell a
// card on the stack, and CR 113.3b/c make an activated or triggered ability
// something else. The textual matcher and both compiled spell matchers must
// answer the same way: Spell admits the spell and rejects the ability;
// SpellAbility -- Forge's union base -- admits the ability.
func TestSpellBaseAgreesAcrossTextualAndCompiledArms(t *testing.T) {
	g, _ := board(t)
	ps := CompilePredicatePrograms([]string{"Spell", "SpellAbility"})
	spell := &state.Object{ID: 1, Zone: state.ZStack, StackKind: state.StackKindSpell, StackKindKnown: true}
	ability := &state.Object{ID: 2, Zone: state.ZStack, StackKind: state.StackKindActivated, StackKindKnown: true}
	cmp := func(name string, got, want bool) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	cmp("textual Spell on a spell", matchesBase(g, "Spell", spell, SpecContext{}), true)
	cmp("textual Spell on an ability", matchesBase(g, "Spell", ability, SpecContext{}), false)
	cmp("textual SpellAbility on an ability", matchesBase(g, "SpellAbility", ability, SpecContext{}), true)
	cmp("compiled Spell on a spell", compiledMatch(compileSpec("Spell"), g, spell, &SpecContext{}), true)
	cmp("compiled Spell on an ability", compiledMatch(compileSpec("Spell"), g, ability, &SpecContext{}), false)
	cmp("compiled SpellAbility on an ability", compiledMatch(compileSpec("SpellAbility"), g, ability, &SpecContext{}), true)
	cmpResult := func(name string, got, want PredicateResult) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	cmpResult("predicate Spell on a spell", ps.Evaluate("Spell", g, spell, SpecContext{}), PredicateYes)
	cmpResult("predicate Spell on an ability", ps.Evaluate("Spell", g, ability, SpecContext{}), PredicateNo)
	cmpResult("predicate SpellAbility on an ability", ps.Evaluate("SpellAbility", g, ability, SpecContext{}), PredicateYes)
}
