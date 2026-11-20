package kgen

import (
	"strings"
	"testing"
)

// TestVoteChoicesLowers pins the Choices$ fixed-list Vote lowering. Six of
// the eleven carriers lower end to end; the other five still refuse at a
// branch or build shape (design item 3: a branch that refuses keeps the card
// refused by that branch's reason, never by the vote's own Choices$
// parameter). Every one of the eleven therefore no longer refuses the vote
// parameter itself. A StoreVoteNum$ or Secretly$ vote still refuses that
// parameter by name.
func TestVoteChoicesLowers(t *testing.T) {
	lowered := []string{
		"Bite of the Black Rose", "Coercive Portal", "Council Guardian",
		"Magister of Worth", "Plea for Power", "Tyrant's Choice",
	}
	refused := map[string]string{
		"Expropriate":            "RememberedPlayerOwn off the battlefield",
		"Galadriel, Elven-Queen": "PutCounter: unread params [Mandatory]",
		"Sail into the West":     "Hidden$ without ChangeType$",
		"Split Decision":         "outside a SpellCast trigger",
		"Trial of a Time Lord":   "Effect without statics",
	}
	names := append([]string{}, lowered...)
	names = append(names, "Expropriate", "Galadriel, Elven-Queen", "Sail into the West",
		"Split Decision", "Trial of a Time Lord", "Capital Punishment", "Mob Verdict")
	kp := kfhBuild(t, names, nil)

	for _, n := range lowered {
		if got := refusalText(kp.cards[n]); got != "" {
			t.Errorf("%s: refused: %s", n, got)
		}
	}
	for n, want := range refused {
		if got := refusalText(kp.cards[n]); !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", n, got, want)
		}
	}
	// The vote itself admits Choices$ on all eleven: no card is refused at
	// the parameter any more.
	for _, n := range names[:len(lowered)+len(refused)] {
		if got := refusalText(kp.cards[n]); strings.Contains(got, `Vote param "Choices"`) {
			t.Errorf("%s: still refuses the fixed-list vote parameter: %s", n, got)
		}
	}
	for n, want := range map[string]string{
		"Capital Punishment": `Vote param "StoreVoteNum"`,
		"Mob Verdict":        `Vote param "Secretly"`,
	} {
		if got := refusalText(kp.cards[n]); !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", n, got, want)
		}
	}

	// The branches follow the op. Coercive Portal is a triggered vote, so
	// gorge asks its Choices$ as a placement mode: each mode's block holds
	// the same Vote (C = fixed-mode 2 | 2<<2 | 1<<5 = 42) with its two
	// choice branches and the tie branch. Council Guardian has four choices
	// and no tie branch (C = 2 | 4<<2 = 18).
	lir, _ := lowerCarriers(t, []string{"Coercive Portal", "Council Guardian"})
	if sec := section(lir, "Coercive Portal"); strings.Count(sec, "Vote C=42") != 2 ||
		!strings.Contains(sec, "Branch C=1") || !strings.Contains(sec, "Branch C=2") {
		t.Errorf("Coercive Portal: fixed-list Vote/branches missing:\n%s", sec)
	}
	if sec := section(lir, "Council Guardian"); !strings.Contains(sec, "Vote C=18") ||
		strings.Contains(sec, "Branch C=4") {
		t.Errorf("Council Guardian: want four branches and no tie branch:\n%s", sec)
	}
}

// refusalText joins a card's refusals for a substring check.
func refusalText(c *lCard) string {
	if c == nil {
		return ""
	}
	var out []string
	for _, r := range c.Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, " | ")
}
