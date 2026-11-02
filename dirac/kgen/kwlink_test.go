package kgen

import (
	"strings"
	"testing"
)

// The multi-object ExileOnMoved$ Battlefield Effect lowers to a per-object grant:
// it ends for each permanent as that permanent leaves (CR 611.2c). Secret Tunnel
// (a fixed pair) and B.O.B.'s plus ability (a variable maximum) lower this way.
// Agility Bobblehead's ValidBlocker$ Creature.withoutHaste form lowers to ExHaste
// (kernel ExtExHaste); B.O.B. as a card stays refused for its Beeble mechanics,
// never for ExileOnMoved or ValidBlocker.
func TestLinkedKeywordLowering(t *testing.T) {
	var tunnel, bob, agility *lCard
	for _, c := range lowerNamed(t, []string{"Secret Tunnel", "B.O.B. (Bevy of Beebles)", "Agility Bobblehead"}) {
		switch c.Name {
		case "Secret Tunnel":
			tunnel = c
		case "B.O.B. (Bevy of Beebles)":
			bob = c
		case "Agility Bobblehead":
			agility = c
		}
	}
	if tunnel == nil || bob == nil || agility == nil {
		t.Fatal("a card is missing")
	}
	for _, r := range tunnel.Refusals {
		t.Errorf("Secret Tunnel refused: %s", r.Reason)
	}
	for _, c := range []*lCard{tunnel, bob} {
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "GrantKeyword obj=Target0 CS=Unblockable") {
			t.Errorf("%s: no grant:\n%s", c.Name, sb.String())
		}
	}
	for _, r := range bob.Refusals {
		if strings.Contains(r.Reason, "ExileOnMoved") {
			t.Errorf("B.O.B. ability refused: %s", r.Reason)
		}
	}
	for _, r := range bob.Refusals {
		if strings.Contains(r.Reason, "ValidBlocker") {
			t.Errorf("B.O.B. ability refused: %s", r.Reason)
		}
	}
	for _, r := range agility.Refusals {
		t.Errorf("Agility Bobblehead refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, agility)
	if !strings.Contains(sb.String(), "ExHaste obj=Target") {
		t.Errorf("Agility Bobblehead: no ExHaste:\n%s", sb.String())
	}
	if strings.Contains(sb.String(), "CS=Unblockable") {
		t.Errorf("Agility Bobblehead lowered as plain Unblockable:\n%s", sb.String())
	}
}
