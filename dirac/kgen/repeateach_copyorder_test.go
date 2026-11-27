package kgen

import (
	"strings"
	"testing"
)

// repeatCopyOrderProbe is Mirror-Style Master's batched CopyPermanent body
// (ChangeZoneTable$ True, a copy per attacking creature) with the CR 101.4
// ChooseOrder$ rider the shape cannot carry.
func repeatCopyOrderProbe() string {
	return "Name:Probe Mirror\nManaCost:2 R R\nTypes:Sorcery\n" +
		"A:SP$ RepeatEach | RepeatCards$ Creature.YouCtrl | ChangeZoneTable$ True | ChooseOrder$ True | UseImprinted$ True | RepeatSubAbility$ DBCopy\n" +
		"SVar:DBCopy:DB$ CopyPermanent | Defined$ Imprinted\n" +
		"Oracle:Probe.\n"
}

// TestRepeatEachCopyOrderRefused: a ChooseOrder$ True batched CopyPermanent
// body refuses by name. The copy loop is a resumable frame
// (act_repeatcards.go repeatEachCardsBatchFrame) entered before the order ask,
// so kgen must not set kop.RepeatEachOrder on a body it cannot order; the same
// probe without ChooseOrder still lowers, so the refusal is the order rider's.
func TestRepeatEachCopyOrderRefused(t *testing.T) {
	ratchetCards(t)
	ordered := strings.Replace(repeatCopyOrderProbe(), " | ChooseOrder$ True", "", 1)
	if got := lowerScript(t, "Probe Mirror", ordered); len(got) != 0 {
		t.Fatalf("the copy body without ChooseOrder refused: %v", got)
	}
	got := strings.Join(lowerScript(t, "Probe Mirror", repeatCopyOrderProbe()), "; ")
	if !strings.Contains(got, "ChooseOrder") {
		t.Fatalf("refusals %q, want one naming ChooseOrder", got)
	}
}
