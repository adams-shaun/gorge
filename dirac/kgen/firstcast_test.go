package kgen

import (
	"fmt"
	"strings"
	"testing"
)

const kfcHead = "Name:Cast Probe\nManaCost:1 G\nTypes:Instant\n"

const kfcStamp = "T:Mode$ SpellCast | ValidCard$ Card.Self | Execute$ TrigStore | Static$ True\nSVar:TrigStore:DB$ StoreSVar | SVar$ X | Type$ Calculate | Expression$ Count$YouCastThisGame\n"

// kfcBody is a spell whose second node is gated on the stamped SVar.
func kfcBody(gate string) string {
	return "A:SP$ GainLife | Defined$ You | LifeAmount$ 1 | SubAbility$ DBGain\nSVar:DBGain:DB$ GainLife | Defined$ You | LifeAmount$ 2 | " + gate
}

// TestFirstCastLowering: the static SpellCast StoreSVar of Count$YouCastThisGame
// read by an EQ1 ConditionCheckSVar$ lowers (the stamp itself to nothing);
// every other read of the stored count refuses by name, since the kernel keeps
// a bit, not a count.
func TestFirstCastLowering(t *testing.T) {
	refs, _ := kwascastLowerHead(t, kfcHead, kfcStamp+kfcBody("ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1"))
	if len(refs) != 0 {
		t.Fatalf("EQ1 gate refused: %v", refs)
	}
	for name, c := range map[string]struct{ lines, want string }{
		"GE2 compare":    {kfcStamp + kfcBody("ConditionCheckSVar$ X | ConditionSVarCompare$ GE2"), "only EQ1"},
		"no compare":     {kfcStamp + kfcBody("ConditionCheckSVar$ X"), "only EQ1"},
		"as an amount":   {kfcStamp + "A:SP$ GainLife | Defined$ You | LifeAmount$ X", "names no SVar"},
		"no stamp":       {kfcBody("ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1"), "names no SVar"},
		"non-Self cast":  {strings.Replace(kfcStamp, "Card.Self", "Card.YouCtrl", 1) + kfcBody("ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1"), "api StoreSVar"},
		"another count":  {strings.Replace(kfcStamp, "Count$YouCastThisGame", "Count$YouCastThisTurn", 1) + kfcBody("ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1"), "api StoreSVar"},
		"permanent read": {kfcStamp + "T:Mode$ Phase | Phase$ Upkeep | TriggerZones$ Battlefield | Execute$ Probe\nSVar:Probe:DB$ GainLife | LifeAmount$ 2 | ConditionCheckSVar$ X | ConditionSVarCompare$ EQ1", "SVar"},
	} {
		refs, _ := kwascastLowerHead(t, kfcHead, c.lines)
		if len(refs) == 0 {
			t.Errorf("%s: lowered, want a refusal containing %q", name, c.want)
			continue
		}
		if got := fmt.Sprint(refs); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %v, want one containing %q", name, got, c.want)
		}
	}
}
