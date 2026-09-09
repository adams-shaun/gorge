package rules

// Task av3 Job 1: a computed ActivationLimit is enforced, not silently
// ignored. activationLimitReached used to strconv.Atoi the limit and return
// false (unenforced) for anything not a plain integer, so Withering Wisps'
// "ActivationLimit$ X" (SVar:X:Count$Valid Swamp.Snow+YouCtrl) was offered
// without limit. These leaves pin the fix: a computed limit sourced from the
// Count$ path is enforced; a literal limit behaves exactly as before; and an
// expression the evaluator genuinely cannot resolve still degrades to
// unenforced, deliberately.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// moveByName moves seat p's card named `name` (seeded via newFixtureDeck's
// extras or the deck slice) to `to`, matching by face name rather than by
// re-parsing source text -- so a real corpus card can be moved even though the
// test helper card() would reject a bare name.
func moveByName(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				return id
			}
		}
	}
	t.Fatalf("card %q not found in seat %d's hand/library", name, p)
	return 0
}

// witheringEngine builds a two-seat fixture game seeded with the REAL corpus
// Withering Wisps plus `swamps` real Snow-Covered Swamps (all on seat 0's
// battlefield), and returns the engine together with the Withering Wisps id
// and the snow-swamp ids.
func witheringEngine(t *testing.T, reg *cards.Registry, swamps int) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	ww, ok := reg.Lookup("Withering Wisps")
	if !ok {
		t.Fatal("corpus fixture: Withering Wisps missing")
	}
	deck := []*cards.Card{ww}
	var swampIDs []state.ObjID
	for i := 0; i < swamps; i++ {
		scs, ok := reg.Lookup("Snow-Covered Swamp")
		if !ok {
			t.Fatal("corpus fixture: Snow-Covered Swamp missing")
		}
		deck = append(deck, scs)
	}
	e := New(Config{Seed: 42, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)}})
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "Withering Wisps", state.ZBattlefield)
	for i := 0; i < swamps; i++ {
		swampIDs = append(swampIDs, moveByName(t, e, 0, "Snow-Covered Swamp", state.ZBattlefield))
	}
	return e, id, swampIDs
}

// TestActivationLimitComputedZeroSnowSwampsWithholdsTheOffer is av3-1: with
// ZERO snow Swamps the computed limit is 0, so Withering Wisps' activated
// ability must NOT be offered at all.
func TestActivationLimitComputedZeroSnowSwampsWithholdsTheOffer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, id, swamps := witheringEngine(t, reg, 0)
	if len(swamps) != 0 {
		t.Fatalf("fixture seeded %d snow swamps, want 0", len(swamps))
	}
	addMana(t, e, 0, "B")
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatalf("Withering Wisps ability offered with 0 snow Swamps (computed ActivationLimit should be 0): %+v", e.Pending().Options)
	}
}

// TestActivationLimitComputedTwoSnowSwampsOfferedTwiceNotThird is av3-2: with
// TWO snow Swamps the computed limit is 2, so the ability is offered twice
// this turn and NOT a third time.
func TestActivationLimitComputedTwoSnowSwampsOfferedTwiceNotThird(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, id, swamps := witheringEngine(t, reg, 2)
	if len(swamps) != 2 {
		t.Fatalf("fixture seeded %d snow swamps, want 2", len(swamps))
	}
	// Three black: enough for three {B} activations, so the withheld third
	// offer is genuinely the limit (2), never a dry pool.
	addMana(t, e, 0, "BBB")

	// First activation is offered and goes on the stack.
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("first activation did not push an ability object: stack=%v", e.G.Stack)
	}
	// Second activation is STILL offered (used=1 < 2).
	opt2 := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt2.Index)
	if len(e.G.Stack) != 2 {
		t.Fatalf("second activation did not push an ability object: stack=%v", e.G.Stack)
	}
	// Third activation is withheld (used=2 >= 2).
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatalf("Withering Wisps ability offered a third time with 2 snow Swamps: %+v", e.Pending().Options)
	}
}

// TestActivationLimitLiteralStillEnforcedExactly is av3-3: the literal-limit
// shape (Basking Rootwalla's ActivationLimit$ 1) behaves exactly as it did
// before -- offered once, withheld on the second activation the same turn.
func TestActivationLimitLiteralStillEnforcedExactly(t *testing.T) {
	src := "Name:LimitedBeast\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ 1 G | Defined$ Self | Power$ 1 | Toughness$ 1 | ActivationLimit$ 1 | SpellDescription$ CARDNAME gets +1/+1.\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 7, src)
	moveByName(t, e, 0, "LimitedBeast", state.ZBattlefield)
	// Four green: enough for two {1}{G} activations, so the second offer is
	// withheld by the limit, not by a dry pool.
	addMana(t, e, 0, "GGGG")

	if _, ok := findAbilityOption(e, id, 0); !ok {
		t.Fatalf("literal ActivationLimit$ 1 ability not offered on first activation")
	}
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatalf("literal ActivationLimit$ 1 ability offered twice in the same turn: %+v", e.Pending().Options)
	}
}

// TestActivationLimitInlineCountWithheldAtComputedCount is the fix-round-1
// leaf. The Withering Wisps leaves above reach the counted limit through the
// SVar-indirection arm (ActivationLimit$ X -> SVar body Count$Valid ...), so
// deleting the DIRECT-RAW arm of resolveActivationLimit -- the one that
// evaluates an INLINE ActivationLimit$ Count$Valid ... value as its own body
// -- leaves all four of them green: EvalCount strips the Count$ prefix from
// the SVar body itself. This leaf pins that arm: the fixture's ActivationLimit$
// IS the inline Count$ expression, so only the direct-raw arm can resolve it.
// Deleting the arm makes resolveActivationLimit fall through to a failed SVar
// lookup, report ok=false, and silently leave the limit unenforced -- which is
// exactly the divergence this task exists to remove.
func TestActivationLimitInlineCountWithheldAtComputedCount(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	src := "Name:InlineBeast\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ 1 G | Defined$ Self | Power$ 1 | Toughness$ 1 | ActivationLimit$ Count$Valid Swamp.Snow+YouCtrl | SpellDescription$ CARDNAME gets +1/+1.\nOracle:x\n"
	beast := card(t, src)
	deck := []*cards.Card{beast}
	scs, ok := reg.Lookup("Snow-Covered Swamp")
	if !ok {
		t.Fatal("corpus fixture: Snow-Covered Swamp missing")
	}
	// Two real snow Swamps on the battlefield: the inline count is 2.
	deck = append(deck, scs, scs)
	e := New(Config{Seed: 42, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)}})
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "InlineBeast", state.ZBattlefield)
	moveByName(t, e, 0, "Snow-Covered Swamp", state.ZBattlefield)
	moveByName(t, e, 0, "Snow-Covered Swamp", state.ZBattlefield)
	// Six green: enough for three {1}{G} activations (each is one generic +
	// one green, and generic is payable by green), so the withheld third offer
	// is genuinely the computed limit (2), never a dry pool. (Four green, the
	// amount Withering Wisps' {B} test needs, would make a dry pool do the
	// withholding and mask the limit -- this leaf must fund the third offer.)
	addMana(t, e, 0, "GGGGGG")

	// First activation is offered and goes on the stack.
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("first activation did not push an ability object: stack=%v", e.G.Stack)
	}
	// Second activation is STILL offered (used=1 < 2) -- if the direct-raw arm
	// is gone the limit is unenforced, so this is offered regardless, but a
	// third-iteration check is what actually fails the mutation.
	if _, ok := findAbilityOption(e, id, 0); !ok {
		t.Fatalf("second activation withheld: used=1 must still be < inline count 2")
	}
	opt2 := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt2.Index)
	if len(e.G.Stack) != 2 {
		t.Fatalf("second activation did not push an ability object: stack=%v", e.G.Stack)
	}
	// Third activation is withheld (used=2 >= 2). With the direct-raw arm
	// deleted this comes back offered (unenforced), failing the leaf.
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatalf("inline Count$Valid ActivationLimit withheld: ability offered a third time with 2 snow Swamps: %+v", e.Pending().Options)
	}
}

// TestActivationLimitUnresolvableItDegradesToUnenforced is av3-4: an
// ActivationLimit expression the evaluator genuinely cannot resolve (an SVar
// name absent from the face's table) stays unenforced, deliberately -- the
// ability is offered exactly as it was before the fix.
func TestActivationLimitUnresolvableItDegradesToUnenforced(t *testing.T) {
	src := "Name:MysteryBeast\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\n" +
		"A:AB$ Pump | Cost$ 1 G | Defined$ Self | Power$ 1 | Toughness$ 1 | ActivationLimit$ NoSuchSVar | SpellDescription$ CARDNAME gets +1/+1.\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 7, src)
	moveByName(t, e, 0, "MysteryBeast", state.ZBattlefield)
	// Four green: enough for two {1}{G} activations, so the second offer is
	// genuinely an unenforced limit, not a dry pool.
	addMana(t, e, 0, "GGGG")

	// Unresolvable: no SVar "NoSuchSVar" exists on this face, so
	// resolveActivationLimit reports ok=false and the limit is unenforced.
	if _, ok := findAbilityOption(e, id, 0); !ok {
		t.Fatalf("unresolvable ActivationLimit$ NoSuchSVar should stay unenforced (ability offered): %+v", e.Pending().Options)
	}
	// It is also still offered a second time -- the unenforced limit never
	// withholds, exactly today's behaviour.
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	if _, ok := findAbilityOption(e, id, 0); !ok {
		t.Fatalf("unresolvable ActivationLimit$ NoSuchSVar should stay unenforced across activations: %+v", e.Pending().Options)
	}
}
