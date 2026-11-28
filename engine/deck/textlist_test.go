package deck

import (
	"strings"
	"testing"
)

// TestParseTextCommanderSection pins the text-decklist parser on the request
// shape POST /api/games takes: a Commander header, the commander, a blank
// line, then the maindeck. The commander is lifted into Commanders, kept once
// in Cards, and the file the list builds is a commander-format file.
func TestParseTextCommanderSection(t *testing.T) {
	raw := "Commander\n1 Ghoulcaller Gisa\n\n1 Sol Ring\n35 Swamp\n"
	tl, err := ParseText([]byte(raw))
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if len(tl.Commanders) != 1 || tl.Commanders[0] != "Ghoulcaller Gisa" {
		t.Fatalf("Commanders = %q, want [Ghoulcaller Gisa]", tl.Commanders)
	}
	gisa := 0
	for _, e := range tl.Cards {
		if e.Name == "Ghoulcaller Gisa" {
			gisa += e.Count
		}
	}
	if gisa != 1 {
		t.Fatalf("Ghoulcaller Gisa appears %d times in Cards %v, want 1", gisa, tl.Cards)
	}
	if len(tl.Cards) != 3 {
		t.Fatalf("Cards = %v, want the commander, Sol Ring and Swamp", tl.Cards)
	}
	f := tl.File("x")
	if f.Format != "commander" || f.Name != "x" {
		t.Fatalf("File(x) = {Name:%q Format:%q}, want {x commander}", f.Name, f.Format)
	}
	if names := f.CommanderNames(); len(names) != 1 || names[0] != "Ghoulcaller Gisa" {
		t.Fatalf("File(x).CommanderNames() = %q", names)
	}

	plain, err := ParseText([]byte("40 Swamp\n"))
	if err != nil {
		t.Fatalf("ParseText(no commander): %v", err)
	}
	if got := plain.File("y").Format; got != "constructed" {
		t.Fatalf("File(y).Format = %q for a list with no commander, want constructed", got)
	}

	if _, err := ParseText([]byte("1 Sol Ring\nnot a card line\n")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("ParseText(bad line) err = %v, want one naming line 2", err)
	}
}
