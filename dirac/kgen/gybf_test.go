package kgen

import (
	"strings"
	"testing"
)

// TestGybfGraveToBattlefield: a graveyard->battlefield
// hidden pick lowers with WithCountersType$ (amount 1) and with GainControl$
// (over any graveyard: the ownerless game-wide pool); the shapes the kernel cannot read exactly refuse
// by name.
func TestGybfGraveToBattlefield(t *testing.T) {
	const pick = "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Battlefield | ChangeNum$ 1 | Hidden$ True | Mandatory$ True"
	for _, c := range []struct{ name, root, want string }{
		{"finality counter", pick + " | ChangeType$ Creature.YouOwn | WithCountersType$ FINALITY", "counters=FINALITY"},
		{"GainControl over your graveyard", pick + " | ChangeType$ Creature.YouOwn | GainControl$ True", "UnderYourControl"},
		{"GainControl ownerless", pick + " | ChangeType$ Creature | GainControl$ True", "UnderYourControl"},
		{"GainControl over an opponent's graveyard", pick + " | ChangeType$ Creature.OppOwn | GainControl$ True", "UnderYourControl"},
	} {
		t.Run("lowers "+c.name, func(t *testing.T) {
			got, ops := czLowered(t, c.root)
			if len(got) != 0 {
				t.Fatalf("refused: %q", got)
			}
			if j := strings.Join(ops, " "); !strings.Contains(j, "ChooseMove:from=Graveyard") || !strings.Contains(j, c.want) {
				t.Errorf("ops %q lack a graveyard ChooseMove with %q", ops, c.want)
			}
		})
	}
	for _, c := range []struct{ name, root, want string }{
		{"ownerless without GainControl", pick + " | ChangeType$ Creature", "graveyard choice without an owner"},
		{"ownerless to hand", "SP$ ChangeZone | Cost$ B | Origin$ Graveyard | Destination$ Hand | ChangeNum$ 1 | Hidden$ True | Mandatory$ True | ChangeType$ Creature", "graveyard choice without an owner"},
		{"counter amount 2", pick + " | ChangeType$ Creature.YouOwn | WithCountersType$ P1P1 | WithCountersAmount$ 2", "WithCountersAmount$ on a graveyard pick"},
		{"unknown counter kind", pick + " | ChangeType$ Creature.YouOwn | WithCountersType$ NOSUCHKIND", `WithCountersType$ "NOSUCHKIND"`},
	} {
		t.Run("refuses "+c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
}
