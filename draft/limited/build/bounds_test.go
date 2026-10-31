package build_test

import (
	"math"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// TestValidateRejectsOutOfBoundsBasics covers the hosting bounds E1-0 adds:
// a negative count, a count over MaxBasicsPerType, and an overflowing count
// are each rejected before the count can reach the engine.
func TestValidateRejectsOutOfBoundsBasics(t *testing.T) {
	pool := poolOf(45)

	neg := build.Deck{Main: []string{pool[0].ID}, Basics: [5]int{-1, 0, 0, 0, 41}}
	if err := build.Validate(pool, neg); err == nil {
		t.Fatal("a deck with a negative basic count validated, want an error")
	} else if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative basic error = %q, want it to name the negative count", err)
	}

	over := build.Deck{Basics: [5]int{41, 0, 0, 0, 0}}
	if err := build.Validate(pool, over); err == nil {
		t.Fatal("a deck with 41 Plains validated, want an error")
	} else if !strings.Contains(err.Error(), "Plains") {
		t.Fatalf("over-bound basic error = %q, want it to name Plains", err)
	}

	huge := build.Deck{Basics: [5]int{1 << 62, 0, 0, 0, 0}}
	if err := build.Validate(pool, huge); err == nil {
		t.Fatal("a deck with an overflowing basic count validated, want an error")
	}

	// The boundary is inclusive: exactly MaxBasicsPerType of one basic is
	// legal as long as the total stays within MaxDeckSize.
	atLimit := build.Deck{Basics: [5]int{build.MaxBasicsPerType, 0, 0, 0, 0}}
	if err := build.Validate(pool, atLimit); err != nil {
		t.Fatalf("a deck at the per-type limit rejected: %v", err)
	}
}

// TestValidateRejectsOversizedDecks checks the total cap: a 101-card deck is
// rejected and an exactly MaxDeckSize deck is accepted.
func TestValidateRejectsOversizedDecks(t *testing.T) {
	pool := poolOf(101)

	over := build.Deck{Main: make([]string, 101)}
	for i := range over.Main {
		over.Main[i] = pool[i].ID
	}
	if err := build.Validate(pool, over); err == nil {
		t.Fatal("a 101-card deck validated, want an error")
	}

	// 61 main + 40 basics = 101, over the total cap even though every
	// individual bound holds.
	mixed := build.Deck{Main: make([]string, 61), Basics: [5]int{40, 0, 0, 0, 0}}
	for i := range mixed.Main {
		mixed.Main[i] = pool[i].ID
	}
	if err := build.Validate(pool, mixed); err == nil {
		t.Fatal("a 61+40=101-card deck validated, want an error")
	}

	// 60 main + 40 basics = 100 is the largest legal deck.
	atLimit := build.Deck{Main: make([]string, 60), Basics: [5]int{40, 0, 0, 0, 0}}
	for i := range atLimit.Main {
		atLimit.Main[i] = pool[i].ID
	}
	if err := build.Validate(pool, atLimit); err != nil {
		t.Fatalf("a 100-card deck rejected: %v", err)
	}
}

// TestTotalSaturates proves Total never wraps for any Basics input, including
// counts that overflow int when summed.
func TestTotalSaturates(t *testing.T) {
	big := build.Deck{Basics: [5]int{1 << 62, 1 << 62, 1 << 62, 1 << 62, 1 << 62}}
	if got := big.Total(); got != math.MaxInt {
		t.Fatalf("Total(5x 1<<62) = %d, want saturated %d", got, math.MaxInt)
	}
	if got := big.Total(); got < 0 {
		t.Fatalf("Total overflowed to a negative %d", got)
	}
	maxed := build.Deck{Basics: [5]int{math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt}}
	if got := maxed.Total(); got != math.MaxInt {
		t.Fatalf("Total(5x MaxInt) = %d, want %d", got, math.MaxInt)
	}
	mined := build.Deck{Basics: [5]int{math.MinInt, math.MinInt, math.MinInt, math.MinInt, math.MinInt}}
	if got := mined.Total(); got != math.MinInt {
		t.Fatalf("Total(5x MinInt) = %d, want saturated %d", got, math.MinInt)
	}
	// A plain sum still works.
	if got := (build.Deck{Main: []string{"a", "b"}, Basics: [5]int{1, 0, 2, 0, 0}}).Total(); got != 5 {
		t.Fatalf("Total(2 main + 3 basics) = %d, want 5", got)
	}
}

// TestToDeckFileOmitsNonPositiveBasics asserts ToDeckFile never writes a
// basic-land entry with a count of zero or less, whatever the Deck holds.
func TestToDeckFileOmitsNonPositiveBasics(t *testing.T) {
	pool := []limited.CardRef{{ID: "c1", Name: "Savannah Lions"}}
	d := build.Deck{
		Main:   []string{"c1"},
		Basics: [5]int{-3, 0, 2, 0, 0},
	}
	f := build.ToDeckFile(pool, d, "bounds")
	for _, e := range f.Cards {
		if e.Count <= 0 {
			t.Fatalf("ToDeckFile emitted %q with count %d, want no non-positive entry", e.Name, e.Count)
		}
	}
	// The positive basic survived and the negative one did not.
	sawSwamp := false
	for _, e := range f.Cards {
		if e.Name == "Swamp" {
			sawSwamp = true
			if e.Count != 2 {
				t.Fatalf("Swamp count = %d, want 2", e.Count)
			}
		}
		if e.Name == "Plains" {
			t.Fatalf("ToDeckFile emitted a Plains entry for a negative count")
		}
	}
	if !sawSwamp {
		t.Fatal("ToDeckFile dropped the positive Swamp entry")
	}
}

// TestAutoLandsStaysWithinBounds proves AutoLands can never produce a count
// outside the hosting bounds Validate enforces.
func TestAutoLandsStaysWithinBounds(t *testing.T) {
	main := make([]string, 23)
	for i := range main {
		main[i] = "c" + itoa(i)
	}
	colourOf := func(id string) [5]int { return [5]int{0, 0, 1, 0, 0} }
	got := build.AutoLands(main, colourOf)
	for i, n := range got {
		if n < 0 || n > build.MaxBasicsPerType {
			t.Fatalf("%s count %d out of bounds [0,%d]", build.BasicNames[i], n, build.MaxBasicsPerType)
		}
	}
	total := build.Deck{Basics: got}.Total()
	if total > build.MaxDeckSize {
		t.Fatalf("AutoLands basics total %d exceeds MaxDeckSize %d", total, build.MaxDeckSize)
	}
}
