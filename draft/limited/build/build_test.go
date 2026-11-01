package build_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// poolOf builds a CardRef pool with ids c0..cN.
func poolOf(n int) []limited.CardRef {
	pool := make([]limited.CardRef, n)
	for i := range pool {
		pool[i] = limited.CardRef{ID: "c" + itoa(i), Name: "Card " + itoa(i), Set: "FDN"}
	}
	return pool
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func TestAutoLandsPipShareLargestRemainder(t *testing.T) {
	// 23 maindeck cards: 6 carry two black pips each (12 black pips), 9
	// carry one red pip (9 red pips), 8 are colourless. 17 land slots split
	// by pip share: black 17*12/21 → floor 9, remainder 15/21; red
	// 17*9/21 → floor 7, remainder 6/21; the last land goes to black's
	// larger remainder: Swamp 10, Mountain 7.
	main := make([]string, 0, 23)
	for i := 0; i < 6; i++ {
		main = append(main, "b"+itoa(i))
	}
	for i := 0; i < 9; i++ {
		main = append(main, "r"+itoa(i))
	}
	for i := 0; i < 8; i++ {
		main = append(main, "x"+itoa(i))
	}
	if len(main) != 23 {
		t.Fatalf("built %d maindeck cards, want 23", len(main))
	}
	colourOf := func(id string) [5]int {
		switch {
		case strings.HasPrefix(id, "b"):
			return [5]int{0, 0, 2, 0, 0}
		case strings.HasPrefix(id, "r"):
			return [5]int{0, 0, 0, 1, 0}
		default:
			return [5]int{}
		}
	}
	got := build.AutoLands(main, colourOf)
	want := [5]int{0, 0, 10, 7, 0}
	if got != want {
		t.Fatalf("AutoLands = %v, want %v", got, want)
	}
}

func TestAutoLandsEdgeCases(t *testing.T) {
	full := make([]string, 40)
	for i := range full {
		full[i] = "c" + itoa(i)
	}
	colourOf := func(id string) [5]int { return [5]int{0, 0, 1, 0, 0} }
	if got := build.AutoLands(full, colourOf); got != ([5]int{}) {
		t.Fatalf("a full main got lands %v, want none", got)
	}
	if got := build.AutoLands(full[:20], colourOf); got != [5]int{0, 0, 20, 0, 0} {
		t.Fatalf("a 20-card one-pip-per-card main got lands %v, want 20 Swamps", got)
	}
	pipless := func(string) [5]int { return [5]int{} }
	if got := build.AutoLands(full[:20], pipless); got != ([5]int{}) {
		t.Fatalf("a pipless main got lands %v, want none", got)
	}
	if got := build.AutoLands(full[:20], nil); got != ([5]int{}) {
		t.Fatalf("a nil colourOf got lands %v, want none", got)
	}
	oversized := make([]string, 45)
	if got := build.AutoLands(oversized, colourOf); got != ([5]int{}) {
		t.Fatalf("an oversized main got lands %v, want none", got)
	}
}

func TestDeckValidateRejectsBadDecks(t *testing.T) {
	pool := poolOf(45)
	main := make([]string, 39)
	for i := range main {
		main[i] = pool[i].ID
	}
	if err := build.Validate(pool, build.Deck{Main: main}); err == nil {
		t.Fatal("a 39-card deck validated, want an error")
	}
	// Basics count toward the total, so 39 main plus one basic is legal.
	if err := build.Validate(pool, build.Deck{Main: main, Basics: [5]int{1, 0, 0, 0, 0}}); err != nil {
		t.Fatalf("a 39+1 deck rejected: %v", err)
	}
	// A foreign ID is not in the pool.
	foreign := build.Deck{Main: append(main, "not-in-pool"), Basics: [5]int{}}
	if err := build.Validate(pool, foreign); err == nil {
		t.Fatal("a deck with a foreign ID validated, want an error")
	}
	// A duplicate maindeck ID is rejected.
	dup := build.Deck{Main: append(main[:38], pool[0].ID, pool[0].ID), Basics: [5]int{2, 0, 0, 0, 0}}
	if err := build.Validate(pool, dup); err == nil {
		t.Fatal("a deck with a duplicate ID validated, want an error")
	}
	// A legal 40-card deck passes.
	var okMain []string
	for i := 0; i < 40; i++ {
		okMain = append(okMain, pool[i].ID)
	}
	if err := build.Validate(pool, build.Deck{Main: okMain}); err != nil {
		t.Fatalf("a legal 40-card deck rejected: %v", err)
	}
}

func TestDeckToDeckFileEntries(t *testing.T) {
	pool := []limited.CardRef{
		{ID: "c1", Name: "Savannah Lions"},
		{ID: "c2", Name: "Healer's Hawk"},
		{ID: "c3", Name: "Llanowar Elves"},
	}
	d := build.Deck{Main: []string{"c3", "c1"}, Basics: [5]int{2, 0, 0, 0, 1}}
	f := build.ToDeckFile(pool, d, "seat-0 draft deck")
	if f.Name != "seat-0 draft deck" {
		t.Fatalf("file name %q, want seat-0 draft deck", f.Name)
	}
	want := []entry{
		{"Llanowar Elves", 1},
		{"Savannah Lions", 1},
		{"Plains", 2},
		{"Forest", 1},
	}
	if len(f.Cards) != len(want) {
		t.Fatalf("file has %d entries, want %d: %+v", len(f.Cards), len(want), f.Cards)
	}
	for i, w := range want {
		if f.Cards[i].Name != w.name || f.Cards[i].Count != w.count {
			t.Fatalf("entry %d = %+v, want %s x%d", i, f.Cards[i], w.name, w.count)
		}
	}
}

type entry struct {
	name  string
	count int
}
