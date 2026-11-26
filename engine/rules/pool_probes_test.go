package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPoolGatesMatchesTheSingleProbes pins poolGates (the fused genesis pass)
// to the single-gate walks it replaces, over every split of four
// carriers and a plain card between the decks and the token table.
func TestPoolGatesMatchesTheSingleProbes(t *testing.T) {
	plain := card(t, "Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	rename := card(t, "Name:Renamer\nManaCost:1 U\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature | SetName$ Frog | Description$ x\nOracle:x\n")
	retype := card(t, "Name:Retyper\nManaCost:1 U\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature | AddType$ Artifact | Description$ x\nOracle:x\n")
	control := card(t, "Name:Taker\nManaCost:1 U\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.EnchantedBy | GainControl$ You | Description$ x\nOracle:x\n")
	colours := card(t, "Name:Swirler\nManaCost:1 U\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Spell,Permanent | AffectedZone$ Stack,Battlefield | ChangeColorWordsTo$ ChosenColor | Description$ x\nOracle:x\n")
	carriers := []*cards.Card{rename, retype, control, colours}
	if !rename.SetsName() || !retype.ChangesTypes() || !control.MayCarryControlStatic() || !colours.ChangesColourWords() {
		t.Fatal("precondition: a fixture carrier is not recognised by its own probe")
	}
	// mask bit i: carrier i present; deckBit i: in a deck (else a token).
	for mask := 0; mask < 16; mask++ {
		for deckBits := 0; deckBits < 16; deckBits++ {
			deck := []*cards.Card{plain}
			tokens := map[string]*cards.Card{"plain": plain}
			for i, c := range carriers {
				if mask&(1<<i) == 0 {
					continue
				}
				if deckBits&(1<<i) != 0 {
					deck = append(deck, c)
				} else {
					tokens[string(rune('a'+i))] = c
				}
			}
			cfg := Config{Decks: [][]*cards.Card{deck, {plain}}, Tokens: tokens}
			s, l, c, w := poolGates(cfg)
			// Genesis gates intentionally census decks only. A token raises
			// the corresponding gate when it is minted, through notePoolCard.
			wantS := mask&deckBits&1 != 0
			wantL := mask&deckBits&2 != 0
			wantC := mask&deckBits&4 != 0
			wantW := mask&deckBits&8 != 0
			if s != wantS || l != wantL || c != wantC || w != wantW {
				t.Fatalf("mask %04b decks %04b: poolGates = (%v %v %v %v), want deck gates = (%v %v %v %v)", mask, deckBits,
					s, l, c, w, wantS, wantL, wantC, wantW)
			}
		}
	}
}
