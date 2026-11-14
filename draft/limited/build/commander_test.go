package build

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
)

// TestValidateCommander pins the Commander-draft deck shape: at least 60
// cards, one or two distinct commanders, each maindecked, and a limited
// deck (Validate) never names one.
func TestValidateCommander(t *testing.T) {
	var pool []limited.CardRef
	var main []string
	for i := 0; i < 45; i++ {
		id := fmt.Sprintf("c%d", i)
		pool = append(pool, limited.CardRef{ID: id, Name: "Card " + id})
		main = append(main, id)
	}
	ok := Deck{Main: main, Basics: [5]int{15}, Commanders: []string{"c0", "c1"}}
	if err := ValidateCommander(pool, ok); err != nil {
		t.Fatalf("legal deck refused: %v", err)
	}
	f := ToDeckFile(pool, ok, "cmdr")
	if got := strings.Join(f.Commanders, ","); got != "Card c0,Card c1" {
		t.Fatalf("ToDeckFile commanders = %q, want both names", got)
	}
	for name, d := range map[string]Deck{
		"short":        {Main: main, Basics: [5]int{14}, Commanders: []string{"c0"}},
		"no commander": {Main: main, Basics: [5]int{15}},
		"three":        {Main: main, Basics: [5]int{15}, Commanders: []string{"c0", "c1", "c2"}},
		"twice":        {Main: main, Basics: [5]int{15}, Commanders: []string{"c0", "c0"}},
		"not main":     {Main: main[1:], Basics: [5]int{16}, Commanders: []string{"c0"}},
	} {
		if err := ValidateCommander(pool, d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := Validate(pool, ok); err == nil {
		t.Error("Validate accepted a limited deck that names commanders")
	}
}

// TestValidateCommanderSize pins an event-named minimum: a 50-card deck
// passes a 50 minimum and fails the default 60, a 49-card one fails 50, and
// a minimum outside MinCommanderDeckSize..MaxDeckSize is itself refused.
func TestValidateCommanderSize(t *testing.T) {
	var pool []limited.CardRef
	var main []string
	for i := 0; i < 36; i++ {
		id := fmt.Sprintf("c%d", i)
		pool = append(pool, limited.CardRef{ID: id, Name: "Card " + id})
		main = append(main, id)
	}
	fifty := Deck{Main: main, Basics: [5]int{14}, Commanders: []string{"c0"}}
	if err := ValidateCommanderSize(pool, fifty, 50); err != nil {
		t.Fatalf("50-card deck refused at 50: %v", err)
	}
	if err := ValidateCommander(pool, fifty); err == nil {
		t.Fatal("50-card deck accepted at the default 60")
	}
	short := Deck{Main: main, Basics: [5]int{13}, Commanders: []string{"c0"}}
	if err := ValidateCommanderSize(pool, short, 50); err == nil {
		t.Fatal("49-card deck accepted at 50")
	}
	for _, min := range []int{MinCommanderDeckSize - 1, MaxDeckSize + 1, 0} {
		if err := ValidateCommanderSize(pool, fifty, min); err == nil {
			t.Errorf("minimum %d accepted", min)
		}
	}
}
