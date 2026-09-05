package deck

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// fixtureRegistry holds two synthetic cards so the tests need no corpus.
// The scripts are authored here, not copied from Forge.
func fixtureRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	r := cards.NewRegistry()
	for _, src := range []string{
		"Name:Mountain\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\nOracle:({T}: Add {R}.)\n",
		"Name:Goblin Guide\nManaCost:R\nTypes:Creature Goblin Scout\nPT:2/2\nK:Haste\nOracle:Haste\n",
	} {
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("fixture parse: %v", diags)
		}
		r.Add(c)
	}
	return r
}

func TestParseReadsNameFormatAndEntries(t *testing.T) {
	raw, err := os.ReadFile("testdata/tiny.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Tiny Test Deck" || f.Format != "legacy" || len(f.Cards) != 2 {
		t.Fatalf("parsed %+v", f)
	}
	if f.Cards[0].Name != "Mountain" || f.Cards[0].Count != 2 {
		t.Fatalf("first entry %+v", f.Cards[0])
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	for _, raw := range []string{"", "{", "{}", `{"cards":[]}`, `{"cards":[{"name":"","count":1}]}`, `{"cards":[{"name":"Mountain","count":0}]}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("Parse(%q) accepted malformed input", raw)
		}
	}
}

func TestResolveExpandsCountsInOrder(t *testing.T) {
	r := fixtureRegistry(t)
	f := File{Cards: []Entry{{"Mountain", 2}, {"Goblin Guide", 1}}}
	cs, err := f.Resolve(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 || cs[0].Faces[0].Name != "Mountain" || cs[1] != cs[0] || cs[2].Faces[0].Name != "Goblin Guide" {
		t.Fatalf("resolved %d cards: %v", len(cs), cs)
	}
}

func TestResolveNamesTheFirstUnknownCard(t *testing.T) {
	r := fixtureRegistry(t)
	f := File{Name: "x", Cards: []Entry{{"Mountain", 1}, {"Black Lotus", 1}}}
	_, err := f.Resolve(r)
	if err == nil || !strings.Contains(err.Error(), "Black Lotus") {
		t.Fatalf("want an error naming Black Lotus, got %v", err)
	}
}

func TestLoadAndStem(t *testing.T) {
	r := fixtureRegistry(t)
	f, cs, err := Load(r, "testdata/tiny.json")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Tiny Test Deck" || len(cs) != 3 {
		t.Fatalf("Load: %+v, %d cards", f, len(cs))
	}
	if got := Stem("internal/testutil/decks/mono-red-goblins.json"); got != "mono-red-goblins" {
		t.Fatalf("Stem = %q", got)
	}
	if _, _, err := Load(r, "testdata/missing.json"); err == nil {
		t.Fatal("Load of a missing file succeeded")
	}
}
