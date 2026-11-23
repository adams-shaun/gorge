package kgen

import (
	"strings"
	"testing"
)

// TestVoteSecretlyFixedList pins the Secretly$ parameter on a fixed-list
// Vote: ballots are asked and tallied in the same order whether or not they
// are revealed at the end (gorge effVote only withholds the picks from the
// public log), so the parameter lowers. Elrond of the White Council and
// Truth or Consequences now refuse at their branch bodies. The player and
// card ballots that carry Secretly$ (Mob Verdict, Círdan the Shipwright,
// Vault 11: Voter's Dilemma) still refuse the parameter by name.
func TestVoteSecretlyFixedList(t *testing.T) {
	names := []string{"Elrond of the White Council", "Truth or Consequences",
		"Mob Verdict", "Círdan the Shipwright", "Vault 11: Voter's Dilemma"}
	kp := kfhBuild(t, names, nil)
	for _, n := range names[:2] {
		if got := refusalText(kp.cards[n]); strings.Contains(got, `Vote param "Secretly"`) {
			t.Errorf("%s: still refuses the fixed-list Secretly$: %s", n, got)
		}
	}
	for _, n := range names[2:] {
		if got := refusalText(kp.cards[n]); !strings.Contains(got, `Vote param "Secretly"`) {
			t.Errorf("%s: refusals %q, want Vote param \"Secretly\"", n, got)
		}
	}
}
