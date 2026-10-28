package cards

// cr-g277: CR 605.1a classifies an activated ability as a mana ability by what
// it can do, not by its head word, so a non-Mana head whose SubAbility$ chain
// reaches a DB$ Mana is one -- unless the chain targets, is a loyalty ability
// or moves a card to or from a library.

import (
	"slices"
	"strings"
	"testing"
)

func parseTestFace(t *testing.T, src string) *Face {
	t.Helper()
	c, diags := ParseBytes("test.txt", []byte(src))
	if c == nil {
		t.Fatalf("parse failed: %v", diags)
	}
	c.Link()
	return c.Faces[0]
}

func firstActivated(t *testing.T, f *Face) *SA {
	t.Helper()
	for _, a := range f.Abilities {
		if a != nil && a.IsActivated() {
			return a
		}
	}
	t.Fatalf("precondition: no activated ability on %s", f.Name)
	return nil
}

func TestManaSubChainWithTargetStaysOnStack(t *testing.T) {
	const tmpl = "Name:Probe\nManaCost:1\nTypes:Creature Bear\nPT:1/1\n" +
		"A:AB$ %HEAD% | Cost$ T | SubAbility$ DBMana\n" +
		"SVar:DBMana:DB$ Mana | Produced$ G%SUB%\nOracle:x\n"
	build := func(head, sub string) *SA {
		src := tmpl
		src = strings.ReplaceAll(src, "%HEAD%", head)
		src = strings.ReplaceAll(src, "%SUB%", sub)
		return firstActivated(t, parseTestFace(t, src))
	}
	// Control: the same shape without a target is a mana ability, so the
	// target is what moves the classification.
	ctl := build("Reveal | RevealValid$ Creature", "")
	if ManaChainProduction(ctl) == nil || !IsManaAbilitySA(ctl) {
		t.Fatalf("precondition: the untargeted Reveal + DB$ Mana control is not a mana ability: %q", ctl.Line)
	}
	if hd := build("Reveal | ValidTgts$ Creature", ""); ManaChainProduction(hd) == nil || IsManaAbilitySA(hd) {
		t.Errorf("a head ValidTgts$ chain classifies as a mana ability: %q", hd.Line)
	}
	if sb := build("Reveal | RevealValid$ Creature", " | ValidTgts$ Player"); ManaChainProduction(sb) == nil || IsManaAbilitySA(sb) {
		t.Errorf("a sub-ability ValidTgts$ chain classifies as a mana ability: %q", sb.Line)
	}
	if lo := build("Reveal | Planeswalker$ True | RevealValid$ Creature", ""); IsManaAbilitySA(lo) {
		t.Errorf("a loyalty ability classifies as a mana ability: %q", lo.Line)
	}
	if lib := build("Draw", ""); IsManaAbilitySA(lib) {
		t.Errorf("a library-moving chain classifies as a mana ability: %q", lib.Line)
	}
}

// subChainManaCarriers are the printed activated abilities with a non-Mana,
// non-ChooseColor head and a DB$ Mana sub that classify as mana abilities
// (measured over the pinned corpus; CR 605.1a: no target, no loyalty marker,
// no library move). The excluded carriers of the shape are Deathrite Shaman,
// Priest of Forgotten Gods, Red Death Shipwrecker, Soulbright Flamekin and
// Seeker (target); Selvala, Explorer Returned (draw); and the loyalty
// abilities of Chandra Heart of Fire, Narset, Quintorius, Sarkhan Unbroken,
// Tamiyo and Valki.
var subChainManaCarriers = []string{
	"Empowered Autogenerator", "Jack-in-the-Mox", "Mana Screw", "Metalworker",
	"Rosheen, Roaring Prophet", "Sacellum Godspeaker", "Spectral Searchlight",
	"Valleymaker", "Victory Chimes",
}

func TestManaSubChainClassifiesTheCorpusCarriers(t *testing.T) {
	reg := openCardsTestCorpus(t)
	var got []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, a := range f.Abilities {
				if a == nil || !a.IsActivated() || IsManaAbilityAPI(a.API) || a.API == "ChooseColor" || ManaChainProduction(a) == nil {
					continue
				}
				if IsManaAbilitySA(a) {
					got = append(got, c.Faces[0].Name)
					if !slices.Contains(f.ManaAbilities(), a) {
						t.Errorf("%s: a classified sub-chain carrier is absent from Face.ManaAbilities()", c.Faces[0].Name)
					}
				}
			}
		}
	}
	slices.Sort(got)
	if want := slices.Sorted(slices.Values(subChainManaCarriers)); !slices.Equal(got, want) {
		t.Errorf("sub-chain mana abilities = %v, want %v", got, want)
	}
}
