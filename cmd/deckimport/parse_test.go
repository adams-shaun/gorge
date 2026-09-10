package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustParse reads and parses one testdata decklist, failing the test on error
// (the fixture is expected to parse).
func mustParse(t *testing.T, file string) *parsedDeck {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseDecklist(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	return d
}

// countOf returns the count of the card whose name matches (case-insensitive)
// in a parsed deck, or 0 if absent.
func countOf(d *parsedDeck, name string) int {
	want := strings.ToLower(name)
	for _, c := range d.Cards {
		if strings.ToLower(c.Name) == want {
			return c.Count
		}
	}
	return 0
}

func TestParseBasic(t *testing.T) {
	d := mustParse(t, "basic.txt")
	if len(d.Cards) != 3 {
		t.Fatalf("cards = %d, want 3: %+v", len(d.Cards), d.Cards)
	}
	if countOf(d, "Lightning Bolt") != 4 {
		t.Errorf("Lightning Bolt count = %d", countOf(d, "Lightning Bolt"))
	}
	if countOf(d, "Monastery Swiftspear") != 4 {
		t.Errorf("Monastery Swiftspear count = %d", countOf(d, "Monastery Swiftspear"))
	}
	if countOf(d, "Mountain") != 20 {
		t.Errorf("Mountain count = %d", countOf(d, "Mountain"))
	}
	if d.Commander != "" || d.Sideboard != 0 {
		t.Errorf("basic list: commander=%q sideboard=%d", d.Commander, d.Sideboard)
	}
}

func TestParseXCount(t *testing.T) {
	d := mustParse(t, "xcount.txt")
	if countOf(d, "Lightning Bolt") != 1 || countOf(d, "Goblin Guide") != 2 || countOf(d, "Mountain") != 20 {
		t.Fatalf("x-counts parsed wrong: %+v", d.Cards)
	}
}

func TestParseAnnotations(t *testing.T) {
	d := mustParse(t, "annotations.txt")
	if countOf(d, "Lightning Bolt") != 4 {
		t.Errorf("annotated Lightning Bolt not matched: %+v", d.Cards)
	}
	if countOf(d, "Monastery Swiftspear") != 4 {
		t.Errorf("annotated Monastery Swiftspear not matched: %+v", d.Cards)
	}
	if countOf(d, "Mishra's Bauble") != 2 {
		t.Errorf("apostrophe card with annotation not matched: %+v", d.Cards)
	}
	if countOf(d, "Mountain") != 20 {
		t.Errorf("annotated Mountain not matched: %+v", d.Cards)
	}
	// The annotation must be stripped, never merged into the name.
	for _, c := range d.Cards {
		if strings.Contains(c.Name, "(") || strings.Contains(c.Name, "2X2") || strings.Contains(c.Name, "MIR") {
			t.Errorf("annotation leaked into name %q", c.Name)
		}
	}
}

func TestParseCommentsAndWhitespace(t *testing.T) {
	d := mustParse(t, "comments.txt")
	if countOf(d, "Lightning Bolt") != 4 || countOf(d, "Goblin Guide") != 4 || countOf(d, "Mountain") != 4 {
		t.Fatalf("comments/whitespace list parsed wrong: %+v", d.Cards)
	}
}

func TestParseDropsSideboard(t *testing.T) {
	d := mustParse(t, "sideboard.txt")
	if len(d.Cards) != 2 {
		t.Fatalf("cards = %d (sideboard must be dropped): %+v", len(d.Cards), d.Cards)
	}
	if d.Sideboard != 3 {
		t.Errorf("sideboard dropped = %d, want 3 (2 Pyroblast + 1 Red Elemental Blast)", d.Sideboard)
	}
}

func TestParseCommander(t *testing.T) {
	d := mustParse(t, "commander.txt")
	if d.Commander != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander = %q", d.Commander)
	}
	// The commander is lifted into the commander field but must also be a
	// card in the list (a Commander deck's 100 includes its commander).
	if countOf(d, "Atraxa, Praetors' Voice") != 1 {
		t.Errorf("commander not in card list: %+v", d.Cards)
	}
	if countOf(d, "Birds of Paradise") != 1 || countOf(d, "Llanowar Elves") != 4 || countOf(d, "Sol Ring") != 2 {
		t.Errorf("maindeck cards wrong: %+v", d.Cards)
	}
	if d.Sideboard != 1 {
		t.Errorf("sideboard dropped = %d, want 1 (Wrath of God)", d.Sideboard)
	}
}

func TestParseRejectsUnparseableLine(t *testing.T) {
	// A count-less card line outside a commander section is a hard error that
	// names the line — never a silent skip or a guessed name.
	_, err := parseDecklist([]byte("4 Lightning Bolt\nLightning Bolt\n"))
	if err == nil {
		t.Fatal("expected an error for a count-less card line")
	}
	if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "Lightning Bolt") {
		t.Fatalf("error should name the line and the text, got %v", err)
	}
}

func TestParseRejectsEmptyDeck(t *testing.T) {
	if _, err := parseDecklist([]byte("# only a comment\n\n// another\n")); err == nil {
		t.Fatal("expected an error for a decklist with no cards")
	}
}

func TestParseCommanderWithoutCount(t *testing.T) {
	// The commander section is the one place a count-less card is accepted.
	d, err := parseDecklist([]byte("Commander\nAtraxa, Praetors' Voice\n1 Birds of Paradise\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Commander != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander = %q", d.Commander)
	}
}

func TestParseCommanderInBothHeaderAndMainIsSingleton(t *testing.T) {
	// Some exports print the commander in the Commander section AND once in
	// the maindeck. The commander is one singleton copy; the two must not add
	// up to a count of two.
	d, err := parseDecklist([]byte("Commander:\nAtraxa, Praetors' Voice\n\n1 Atraxa, Praetors' Voice\n1 Birds of Paradise\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Commander != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander = %q", d.Commander)
	}
	if countOf(d, "Atraxa, Praetors' Voice") != 1 {
		t.Fatalf("commander must be a single copy, got %d: %+v", countOf(d, "Atraxa, Praetors' Voice"), d.Cards)
	}
}
