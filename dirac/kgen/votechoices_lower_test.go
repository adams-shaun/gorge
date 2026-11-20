package kgen

import (
	"strings"
	"testing"
)

// TestVoteChoicesLowers pins the Choices$ fixed-list Vote lowering. The four
// non-triggered carriers (the sorceries) lower end to end; Capital Punishment
// is the StoreVoteNum$ form (TestVoteStoreNumLowers). A fixed-list Vote on a
// triggered ability without StoreVoteNum$ refuses by name: it has no lockstep
// evidence (gorge poses no placement mode for a triggered Vote). The remaining carriers still refuse at a
// branch or build shape (design item 3: a branch that refuses keeps the card
// refused by that branch's reason, never by the vote's own Choices$
// parameter). No card refuses the vote parameter itself. A Secretly$ vote
// still refuses that parameter by name.
func TestVoteChoicesLowers(t *testing.T) {
	const triggered = "Vote on a triggered ability without StoreVoteNum$"
	lowered := []string{"Bite of the Black Rose", "Capital Punishment", "Plea for Power", "Tyrant's Choice"}
	refused := map[string]string{
		"Coercive Portal":        triggered,
		"Council Guardian":       triggered,
		"Galadriel, Elven-Queen": triggered,
		"Magister of Worth":      triggered,
		"Trial of a Time Lord":   triggered,
		"Expropriate":            "RememberedPlayerOwn off the battlefield",
		"Sail into the West":     "Hidden$ without ChangeType$",
		"Split Decision":         "outside a SpellCast trigger",
	}
	names := append([]string{}, lowered...)
	names = append(names, "Expropriate", "Coercive Portal", "Council Guardian",
		"Galadriel, Elven-Queen", "Magister of Worth", "Sail into the West",
		"Split Decision", "Trial of a Time Lord", "Mob Verdict")
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
	// The vote itself admits Choices$ on all twelve: no card is refused at
	// the parameter any more.
	for _, n := range names[:len(names)-1] {
		if got := refusalText(kp.cards[n]); strings.Contains(got, `Vote param "Choices"`) {
			t.Errorf("%s: still refuses the fixed-list vote parameter: %s", n, got)
		}
	}
	if got := refusalText(kp.cards["Mob Verdict"]); !strings.Contains(got, `Vote param "Secretly"`) {
		t.Errorf("Mob Verdict: refusals %q, want Vote param \"Secretly\"", got)
	}

	// A lowered spell carries one Vote op with its branches following it.
	// Tyrant's Choice has two choices and a tie branch: C = fixed-mode 2 |
	// 2<<2 | 1<<5 = 42, then Branch C=0, C=1 and the tie at C=2. No placement
	// Charm wraps it (the triggered shape is refused, not emulated).
	lir, _ := lowerCarriers(t, []string{"Tyrant's Choice"})
	sec := section(lir, "Tyrant's Choice")
	if strings.Count(sec, "Vote C=42") != 1 {
		t.Errorf("Tyrant's Choice: want one fixed-list Vote C=42:\n%s", sec)
	}
	for _, want := range []string{"Branch skip=1", "Branch C=1 skip=1", "Branch C=2 skip=1"} {
		if !strings.Contains(sec, want) {
			t.Errorf("Tyrant's Choice: missing %s:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "Charm") {
		t.Errorf("Tyrant's Choice: a non-triggered vote has no placement Charm:\n%s", sec)
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
