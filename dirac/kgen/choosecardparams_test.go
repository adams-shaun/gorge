package kgen

import (
	"strings"
	"testing"
)

// TestChooseCardDefaultParams: a mandatory ChooseCard's default-valued
// params lower. ChoiceTitle$ is display-only (gorge reads it only as the
// decision's Prompt), Amount$ 1 is choiceBounds' max default and Defined$ You
// is the resolving controller, exactly what the kernel's OpChooseCard already
// does. Anything else stays refused by name.
func TestChooseCardDefaultParams(t *testing.T) {
	carriers := []string{
		"Summoner's Grimoire", // ChoiceTitle, hand, Creature.YouOwn
		"Cosmium Confluence",  // ChoiceTitle, a charm mode, Cave.YouCtrl
		"End-Blaze Epiphany",  // Amount 1 + ChoiceTitle, Card.IsRemembered in exile
		"Riverwheel Sweep",    // Amount 1 + ChoiceTitle + Defined You
		"Heroes' Hangout",     // Amount 1 + ChoiceTitle + Defined You
	}
	// Riverwheel Sweep, Heroes' Hangout and Cosmium Confluence now lower
	// cleanly; the other two read the same params but each still has an unrelated
	// second blocker. Pin that blocker so a silent widening here is caught,
	// and assert none of them is the shared unread-params check.
	second := map[string]string{
		"Summoner's Grimoire": "move Hand->Battlefield",
	}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, carriers) {
		seen[c.Name] = true
		want, hasSecond := second[c.Name]
		if !hasSecond && len(c.Refusals) != 0 {
			for _, r := range c.Refusals {
				t.Errorf("%s: refused: %s", c.Name, r.Reason)
			}
			continue
		}
		if hasSecond && len(c.Refusals) == 0 {
			t.Errorf("%s: lowered clean, want the named second blocker %q", c.Name, want)
		}
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "unread params") {
				t.Errorf("%s: ChooseCard parameters still unread: %s", c.Name, r.Reason)
			} else if hasSecond && !strings.Contains(r.Reason, want) {
				t.Errorf("%s: refusal %q, want only the named second blocker %q", c.Name, r.Reason, want)
			}
		}
	}
	for _, n := range carriers {
		if !seen[n] {
			t.Errorf("%s: not lowered", n)
		}
	}

	// A synthetic mandatory ChooseCard with every accepted default reads.
	const base = "Name:ChooseCard Probe\nManaCost:1 U\nTypes:Sorcery\nA:SP$ ChooseCard | Mandatory$ True | Choices$ Creature"
	cases := []struct {
		name, extra, want string
	}{
		{"all defaults", " | Amount$ 1 | Defined$ You | ChoiceTitle$ Choose a creature", ""},
		{"amount 2", " | Amount$ 2", `ChooseCard Amount$ "2"`},
		{"amount X", " | Amount$ X", `ChooseCard Amount$ "X"`},
		{"defined opponent", " | Defined$ Opponent", `ChooseCard Defined$ "Opponent"`},
		{"defined targeted controller", " | Defined$ TargetedController", `ChooseCard Defined$ "TargetedController"`},
	}
	for _, c := range cases {
		got := keclReason(keclSynth(t, "ChooseCard Probe", base+c.extra+"\n"))
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: refused: %s", c.name, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, got, c.want)
		}
	}
}
