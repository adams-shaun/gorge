package feedback

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestCompileTokenDerivesTheCard pins the token recompilation Load does:
// a recorded script text must come back as a fully derived card — printed
// fields AND the derived P/T the JSON round trip cannot carry — through
// the same ParseBytes/Link/ApplyIntrinsics pipeline the corpus compiler
// runs. A zero-derived shell here would replay a 1/1 token as a 0/0 and
// diverge at its first combat.
func TestCompileTokenDerivesTheCard(t *testing.T) {
	src := "Name:Goblin Warrior Token\nManaCost:no cost\nColors:red,green\nTypes:Creature Goblin Warrior\nPT:1/1\nOracle:\n"
	c, err := compileToken("test_goblin", src)
	if err != nil {
		t.Fatalf("compileToken: %v", err)
	}
	f := c.Faces[0]
	if got := f.Power(); got != 1 || f.Toughness() != 1 {
		t.Errorf("derived P/T = %d/%d, want 1/1", got, f.Toughness())
	}
	if !strings.Contains(strings.Join(f.Types, " "), "Goblin") {
		t.Errorf("types = %v, want Goblin Warrior among them", f.Types)
	}
}

// TestConfigResolvesDecksThroughTheRegistry pins the deck half of Load's
// config rebuild: recorded card names resolve through the registry (the
// same normalised lookup a deck file gets), and a name the registry does
// not know is an error naming the seat and card, never a silent nil card.
func TestConfigResolvesDecksThroughTheRegistry(t *testing.T) {
	reg := cards.NewRegistry()
	c, diags := cards.ParseBytes("test_fixture_card.txt", []byte("Name:Test Pipewhale\nTypes:Creature Whale\nPT:2/2\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("fixture card: %v", diags)
	}
	reg.Add(c)

	m := matchJSON{DeckCards: [][]string{{"Test Pipewhale"}}}
	cfg, err := config(m, reg)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if len(cfg.Decks[0]) != 1 || cfg.Decks[0][0] != c {
		t.Errorf("deck resolved to %+v", cfg.Decks[0])
	}

	m.DeckCards = [][]string{{"Card The Corpus Never Knew"}}
	if _, err := config(m, reg); err == nil {
		t.Fatal("config accepted an unknown card name")
	} else if !strings.Contains(err.Error(), "seat 0 card 0") {
		t.Errorf("error does not name the seat and card: %v", err)
	}
}
