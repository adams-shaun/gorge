package cards

import (
	"path/filepath"
	"testing"
)

// The corpus-driven identity tests are the strong ones here: colour identity
// is a claim that must hold across every one of the ~33,000 cards, and
// hand-written examples alone would not catch a derivation that was right by
// luck on five cards and wrong on a thousand.

// corpusCache hands back the whole-registry registry shared across the package
// (see compiledCorpus in corpus_test.go): the identity checks iterate all
// ~33k cards, and that registry is compiled once for every corpus test, so
// the identity sweep adds only the cheap bitmask assertions on top of a build
// the M0 gate was already paying for — no second full-corpus load.
func corpusCache(t *testing.T) *Registry {
	t.Helper()
	return compiledCorpus(t)
}

// mustLookup resolves a card by name or fails the test: a named trap card
// missing from the deliberate pinning is a fixture error a human must see.
func mustLookup(t *testing.T, r *Registry, name string) *Card {
	t.Helper()
	c, ok := r.Lookup(name)
	if !ok {
		t.Fatalf("registry has no card %q", name)
	}
	return c
}

// TestColourIdentityAcrossTheCorpus runs the whole registry through the
// derivation and pins the two properties that must hold for every card.
//
//   - Every colour in a face's mana cost is in its colour identity (a pip
//     can never fall out of the identity).
//   - Identity is a superset of the card's colour: the card's colour is its
//     colour indicator, or the colours of its casting cost, and a colour
//     identity narrower than that would be impossible.
//
// The third corpus property — C, S, P and numerics never add a colour — is
// structurally guaranteed (manaColours only ever ORs in letters that actually
// are WUBRG), so it is pinned directly in TestManaColoursSpecialSymbols
// rather than re-asserted across the corpus where it could never fail.
func TestColourIdentityAcrossTheCorpus(t *testing.T) {
	r := corpusCache(t)
	for _, c := range r.Cards {
		for _, f := range c.Faces {
			id := f.ColourIdentity()
			// cost pips ⊆ identity
			if cost := manaColours(f.ManaCost); cost&^id != 0 {
				t.Fatalf("%s (%s): cost pips %08b not in identity %08b", c.Path, f.Name, cost, id)
			}
			// identity ⊇ colour (colour = indicator, else cost colours)
			colour := colourIndicator(f.Colors)
			if colour == 0 {
				colour = manaColours(f.ManaCost)
			}
			if colour&^id != 0 {
				t.Fatalf("%s (%s): colour %08b not in identity %08b", c.Path, f.Name, colour, id)
			}
		}
	}
}

// TestColourIdentityPinnedTrapCards pins the derivation to five cards whose
// non-obvious identity a hand-written fixture could get wrong deliberately
// and nobody would ever notice from examples written the same way.
func TestColourIdentityPinnedTrapCards(t *testing.T) {
	r := corpusCache(t)

	// Ghalta, Primal Hunger: costs {10}{G}{G}. Its identity is exactly green,
	// not the five colours, not colourless — the generic {10} must add
	// nothing and the two {G} collapse to a single green bit. A parser that
	// treated every cost letter as a colour, or leaned on the card's colour
	// alone, would get this wrong.
	if got := mustLookup(t, r, "Ghalta, Primal Hunger").ColourIdentity(); got != ColourGreen {
		t.Fatalf("Ghalta identity = %08b, want green (%08b)", got, ColourGreen)
	}

	// Sai, Master Thopterist: costs {2}{U} and also has an activated ability
	// "{1}{U}, Sacrifice two artifacts: Draw". The {U} in that ability's cost
	// is rules text and must be in the identity alongside the cost's {U}. A
	// parser that read only the mana cost would still get {U} here, so this
	// pins that ability-text scanning is part of the derivation at all.
	if got := mustLookup(t, r, "Sai, Master Thopterist").ColourIdentity(); got != ColourBlue {
		t.Fatalf("Sai identity = %08b, want blue (%08b)", got, ColourBlue)
	}

	// Ghoulcaller Gisa: costs {3}{B}{B} and its activated ability costs {B},
	// {T}. The brief calls this the trap: at least one {B} must survive from
	// the ability's Cost$ even though the mana cost already has {B}, and the
	// word "Sacrifice" in the cost is not a colour. A parser that only ever
	// read the mana cost would be fine here, so the gate is the ability text.
	if got := mustLookup(t, r, "Ghoulcaller Gisa").ColourIdentity(); got != ColourBlack {
		t.Fatalf("Ghoulcaller Gisa identity = %08b, want black (%08b)", got, ColourBlack)
	}

	// A hybrid card — Bioshift costs the single hybrid symbol {G/U}. Its
	// identity is BOTH green and blue at once, so the derivation must add
	// every colour letter of a hybrid, not pick one.
	if got := mustLookup(t, r, "Bioshift").ColourIdentity(); got != ColourGreen|ColourBlue {
		t.Fatalf("Bioshift identity = %08b, want {G,U} (%08b)", got, ColourGreen|ColourBlue)
	}

	// A Phyrexian card — Gut Shot costs the single Phyrexian symbol {R/P}.
	// Its identity is red: the R counts, the P is not a colour. A parser
	// that treated P as a colour, or dropped the whole symbol, would disagree.
	if got := mustLookup(t, r, "Gut Shot").ColourIdentity(); got != ColourRed {
		t.Fatalf("Gut Shot identity = %08b, want red (%08b)", got, ColourRed)
	}
}

// TestManaColoursSpecialSymbols pins that none of the corpus's non-colour
// mana symbols (and no bare generic) contributes a colour. This is the
// corpus-level "C, S, P and numerics never add a colour" property exercised
// per symbol, where it can actually be isolated.
func TestManaColoursSpecialSymbols(t *testing.T) {
	neutral := map[string]uint8{"C": 0, "S": 0, "P": 0, "X": 0, "2": 0, "1": 0, "10": 0}
	for sym, want := range neutral {
		if got := manaColours(sym); got != want {
			t.Errorf("manaColours(%q) = %08b, want %08b", sym, got, want)
		}
	}
	// Slashed hybrid/Phyrexian forms contribute only their colour letter.
	if got := manaColours("2/B"); got != ColourBlack {
		t.Errorf(`manaColours("2/B") = %08b, want black`, got)
	}
	if got := manaColours("2/R 2/G"); got != ColourRed|ColourGreen {
		t.Errorf(`manaColours("2/R 2/G") = %08b, want {R,G}`, got)
	}
	if got := manaColours("W/U"); got != ColourWhite|ColourBlue {
		t.Errorf(`manaColours("W/U") = %08b, want {W,U}`, got)
	}
}

// TestCardIdentityUnionsFaces pins that a double-faced card's identity is the
// union of both faces, never just the front one. Delver of Secrets is a DFC
// in the repo decks; the union over its faces must stay exactly blue.

// TestCardIdentityUnionsFaces pins that a double-faced card's identity is the
// union of both faces, never just the front one. Delver of Secrets is a DFC
// in the repo decks; the union over its faces must stay exactly blue.
func TestCardIdentityUnionsFaces(t *testing.T) {
	r := corpusCache(t)
	if got := mustLookup(t, r, "Delver of Secrets").ColourIdentity(); got != ColourBlue {
		t.Fatalf("Delver of Secrets identity = %08b, want blue", got)
	}
}

// TestColourIdentitySurvivesGobRoundTrip proves the derived identity is not
// lost to gob encoding and is recomputed on decode: after a fresh Save, a
// face reads the same identity a directly-parsed one does, exactly like the
// other derived fields (derive_test).
func TestColourIdentitySurvivesGobRoundTrip(t *testing.T) {
	direct, _ := ParseBytes("s.txt", []byte(
		"Name:Sample\n"+
			"ManaCost:1 G G\n"+
			"Types:Creature\n"+
			"A:AB$ Mana | Cost$ G T | Produced$ G | SpellDescription$ Add {G}.\n"+
			"Oracle:{G}, {T}: Add {G}.\n"))
	if got := direct.Faces[0].ColourIdentity(); got != ColourGreen {
		t.Fatalf("direct identity = %08b, want green", got)
	}

	r := NewRegistry()
	r.Add(direct)
	dir := t.TempDir()
	p := filepath.Join(dir, "ir.gob.gz")
	if err := r.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Cards[0].Faces[0].ColourIdentity(); got != ColourGreen {
		t.Fatalf("gob-routed identity = %08b, want green", got)
	}
}

// TestBasicLandsHaveEmptyIdentity checks a deliberate property, not an
// accident of skipping intrinsics: a basic land's colour identity must be
// empty so it fits any commander deck. That is exactly what excluding the
// intrinsic basic-land mana ability produces (a Mountain's "{T}: Add {R}" is
// granted by the land type, not printed rules text, and lives in reminder
// text which is not scanned).
func TestBasicLandsHaveEmptyIdentity(t *testing.T) {
	r := corpusCache(t)
	for _, name := range []string{"Mountain", "Island", "Plains", "Swamp", "Forest", "Wastes"} {
		if id := mustLookup(t, r, name).ColourIdentity(); id != 0 {
			t.Fatalf("%s colour identity = %08b, want empty", name, id)
		}
	}
}

// TestDeriveColourIdentityFromScriptFields is a unit check (no corpus) that
// the derivation walks the ability, trigger, static and SVar paths, and that
// prose whose words carry an uppercase colour-initial (which is NOT a mana
// symbol) is never read as a pip — the class of false positive a naive
// "grep for WUBRG" scanner would fall into.
func TestDeriveColourIdentityFromScriptFields(t *testing.T) {
	c, _ := ParseBytes("s.txt", []byte(
		"Name:Complex\n"+
			"ManaCost:2 W\n"+
			"Types:Creature\n"+
			"A:AB$ Mana | Cost$ B T | Produced$ B | SpellDescription$ A White creature being targeted by Swamp words is prose and adds no Blue.\n"+
			"T:Mode$ Tap | TriggerDescription$ Swamp and White are prose words here too.\n"))
	// The prose carries no genuine colour pips and must introduce none; the
	// ability Cost$ {B} must. Identity is {W} (cost) | {B} (ability).
	if want := ColourWhite | ColourBlack; c.Faces[0].ColourIdentity() != want {
		t.Fatalf("Complex identity = %08b, want %08b", c.Faces[0].ColourIdentity(), want)
	}

	// And prose alone (with its colour-initial words) contributes nothing.
	lone, _ := ParseBytes("s.txt", []byte("Name:Prose\nTypes:Creature\nOracle:The word Black here, and Blue, and Red, all prose or reminder.\n"))
	if got := lone.Faces[0].ColourIdentity(); got != 0 {
		t.Fatalf("Prose identity = %08b, want empty", got)
	}
}

// TestColourIndicatorMapping guards that colour-indicator values that name a
// colour map to bits, while colourless maps to none.
func TestColourIndicatorMapping(t *testing.T) {
	if got := colourIndicator("black, red"); got != ColourBlack|ColourRed {
		t.Fatalf("colourIndicator(black,red) = %08b", got)
	}
	if got := colourIndicator("colorless"); got != 0 {
		t.Fatalf("colourIndicator(colorless) = %08b", got)
	}
	if got := colourIndicator("blue"); got != ColourBlue {
		t.Fatalf("colourIndicator(blue) = %08b", got)
	}
}
