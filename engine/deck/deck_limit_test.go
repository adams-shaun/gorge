package deck

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDeckLimitCapsNamedCardCount pins CR 100.2a's bounded deck-construction
// keyword (Nazgûl's "A deck can have up to nine cards named Nazgûl.", Seven
// Dwarves' seven): DeckLimit replaces the Commander singleton rule for the
// named card with its own count, so seven copies of a limit-7 card validate
// where the singleton rule alone would reject even two, and twelve do not.
func TestDeckLimitCapsNamedCardCount(t *testing.T) {
	r := commanderFixture(t)
	// The K line's sentence tail is decoration (DeckLimitOf reads only the
	// count segment), so the fixture paraphrases the cap instead of quoting
	// the corpus line verbatim (the GPL boundary).
	src := "Name:Seven Dwarves\nManaCost:1 W\nTypes:Creature Dwarf\nPT:2/2\nK:DeckLimit:7:CARDNAME may appear at most seven times in one deck.\n"
	c, diags := cards.ParseBytes("fixture.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("Seven Dwarves parse: %v", diags)
	}
	r.Add(c)
	if got := DeckLimitOf(c); got != 7 {
		t.Fatalf("DeckLimitOf(Seven Dwarves) = %d, want 7", got)
	}

	seven := func(dwarves int) File {
		f := legalMonoWhiteCommander()
		f.Cards = []Entry{{"Amalia", 1}, {"Knight", 1}, {"Seven Dwarves", dwarves}, {"Plains", 100 - 2 - dwarves}}
		return f
	}
	if err := seven(7).ValidateCommander(r); err != nil {
		t.Fatalf("seven Dwarves must validate under DeckLimit:7: %v", err)
	}
	err := seven(12).ValidateCommander(r)
	if err == nil {
		t.Fatal("twelve Dwarves must not validate under DeckLimit:7")
	}
	if !strings.Contains(err.Error(), "up to 7") {
		t.Errorf("DeckLimit violation must name the limit, got: %v", err)
	}
	if strings.Contains(err.Error(), "singleton") {
		t.Errorf("DeckLimit replaced the singleton rule, got: %v", err)
	}
}
