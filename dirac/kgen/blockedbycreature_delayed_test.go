package kgen

import (
	"strings"
	"testing"
)

// TestDeathgazerLowers: both AttackerBlockedByCreature halves register an
// end-of-combat delayed program over the pair's other member (the blocker's
// half remembers the attacker, the attacker's half the blocker), whose body
// destroys the remembered object.
func TestDeathgazerLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Deathgazer"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	for _, want := range []string{
		"pairs anchor-blocker=true",
		"pairs anchor-blocker=false",
		"DelayedTrigger obj=Triggered C=1 CS=NextEndCombat",
		"DelayedTrigger obj=Triggered C=3 CS=NextEndCombat",
		"Destroy obj=DelayedRemembered",
	} {
		if !strings.Contains(sb.String(), want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, sb.String())
		}
	}
}

// blockedByProbe is a creature whose "blocks" trigger runs body.
func blockedByProbe(body string) string {
	return strings.Join([]string{
		"Name:Probe",
		"ManaCost:1 R",
		"Types:Creature Human",
		"PT:2/2",
		"T:Mode$ AttackerBlockedByCreature | ValidCard$ Creature | ValidBlocker$ Card.Self | Execute$ ProbeBody | TriggerDescription$ probe",
		"SVar:ProbeBody:" + body,
		"Oracle:probe",
	}, "\n")
}

// TestBlockedByCreatureRememberedStillRefused: only the delayed
// registration's own Triggered{Attacker,Blocker}LKICopy naming is exempt; a
// body that reads the trigger's remembered set (gorge seeds the pair's other
// member there, the kernel's is empty) still refuses by name.
func TestBlockedByCreatureRememberedStillRefused(t *testing.T) {
	const want = "AttackerBlockedByCreature trigger body reads Remembered"
	for name, body := range map[string]string{
		"remembered read":    "DB$ Destroy | Defined$ Remembered",
		"remembered in SVar": "DB$ Pump | Defined$ Self | NumAtt$ X | SVar$ X | References$ X\nSVar:X:Count$Remembered",
		"delayed with body read of the capture": "DB$ DelayedTrigger | Mode$ Phase | Phase$ EndCombat | Execute$ ProbeEnd\n" +
			"SVar:ProbeEnd:DB$ Destroy | Defined$ Remembered",
	} {
		got := abReasons(abLower(t, blockedByProbe(body), false))
		if !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", name, got, want)
		}
	}
}
