package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestNonCreatureSubtypeVocabKnown: a non-creature subtype no pool card
// carries is still a filter word when the corpus's type vocabulary names it
// and the kernel's table can hold it as a non-creature subtype. A word
// outside that set (a supertype the kernel does not model, or a made-up
// word) stays unknown, so it cannot widen the changeling grant.
func TestNonCreatureSubtypeVocabKnown(t *testing.T) {
	_, cardsDir := testInputs(t)
	p, err := loadCardNames([]string{"Unwilling Vessel"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Room", "Lesson", "Shrine"} {
		if !p.knownSubtype(w) {
			t.Errorf("knownSubtype(%q) = false; want true (corpus vocabulary + kernel non-creature subtype)", w)
		}
	}
	for _, w := range []string{"Snow", "World", "Ongoing", "BogusSubtype"} {
		if p.knownSubtype(w) {
			t.Errorf("knownSubtype(%q) = true; want false (not a kernel non-creature subtype)", w)
		}
	}
	if _, err := parseFilter("Land.Snow", p.knownSubtype, p.isTypeWord); err == nil ||
		!strings.Contains(err.Error(), `unknown property "Snow"`) {
		t.Errorf("parseFilter(Land.Snow) = %v; want unknown property \"Snow\"", err)
	}
	f, err := parseFilter("Card.Room", p.knownSubtype, p.isTypeWord)
	if err != nil {
		t.Fatalf("parseFilter(Card.Room): %v", err)
	}
	if len(f.Clauses) != 1 || f.Clauses[0].Subtype != "Room" {
		t.Errorf("parseFilter(Card.Room) = %s; want one clause with Subtype Room", f)
	}
}

// TestSubtypevocabMutation turns the kernel's non-creature-subtype reader
// false: the corpus vocabulary alone must not lower a subtype word, so
// Sokka's Lesson filter refuses (as it did before the vocabulary arm) and a
// direct Card.Lesson property parses only with the reader true.
func TestSubtypevocabMutation(t *testing.T) {
	_, cardsDir := testInputs(t)
	old := dirac.DebugSubtypeVocabNone
	defer func() { dirac.DebugSubtypeVocabNone = old }()

	p, err := loadCardNames([]string{"Sokka, Bold Boomeranger"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	refusal := func() string {
		for _, c := range lowerPool(p).cards {
			for _, r := range c.Refusals {
				return r.Reason
			}
		}
		return ""
	}

	dirac.DebugSubtypeVocabNone = true
	if got := refusal(); !strings.Contains(got, `unknown filter head "Lesson"`) {
		t.Fatalf("with DebugSubtypeVocabNone: Sokka refusal %q; want unknown filter head \"Lesson\"", got)
	}
	if _, err := parseFilter("Card.Lesson", p.knownSubtype, p.isTypeWord); err == nil ||
		!strings.Contains(err.Error(), `unknown property "Lesson"`) {
		t.Fatalf("with DebugSubtypeVocabNone: parseFilter(Card.Lesson) = %v; want unknown property \"Lesson\"", err)
	}

	dirac.DebugSubtypeVocabNone = false
	if got := refusal(); got != "" {
		t.Fatalf("without DebugSubtypeVocabNone: Sokka refused: %s", got)
	}
	f, err := parseFilter("Card.Lesson", p.knownSubtype, p.isTypeWord)
	if err != nil {
		t.Fatalf("without DebugSubtypeVocabNone: parseFilter(Card.Lesson): %v", err)
	}
	if len(f.Clauses) != 1 || f.Clauses[0].Subtype != "Lesson" {
		t.Errorf("parseFilter(Card.Lesson) = %s; want one clause with Subtype Lesson", f)
	}
}
