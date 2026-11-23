package kgen

import (
	"fmt"
	"strings"
	"testing"
)

// TestEffectTriggersBlocksForms: an end-of-turn DB$ Effect emblem on
// Mode$ Blocks lowers (kernel dirac/trig_emblemblocks.go). The body reads the
// blocker: Battle Cry's TriggeredBlockerLKICopy, and the two
// TriggeredBlockerController bodies.
func TestEffectTriggersBlocksForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Battle Cry", form: "EOT emblem on Blocks, TriggeredBlockerLKICopy", lir: "CreateEmblem"},
		{card: "Benefactor's Draught", form: "EOT emblem on Blocks, ValidCard$ Creature.OppCtrl", lir: "CreateEmblem"},
		{card: "Mage Hunters' Onslaught", form: "EOT emblem on Blocks, TriggeredBlockerController", lir: "CreateEmblem"},
		{card: "Noxious Assault", form: "EOT emblem on Blocks, TriggeredBlockerController", lir: "CreateEmblem"},
	})
}

const blocksEmblemScript = `Name:Test Blocks Emblem
ManaCost:1 W
Types:Instant
A:SP$ Effect%s | Triggers$ TrigBlock | SpellDescription$ test
SVar:TrigBlock:Mode$ Blocks%s | Execute$ Body | TriggerDescription$ test
SVar:Body:DB$ Pump | Defined$ %s | NumDef$ +1
Oracle:x
`

// TestEffectTriggersBlocksRefusals: the shapes the kernel's blocker-bound hook
// cannot serve refuse by name, and the positive twin lowers.
func TestEffectTriggersBlocksRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, eff, trig, defined, want string
	}{
		{"blocker twin", "", "", "TriggeredBlockerLKICopy", ""},
		{"attacker read", "", "", "TriggeredAttackerLKICopy", "Effect Blocks trigger reads the attacker"},
		{"trigger card read", "", "", "TriggeredCardLKICopy", "Effect Blocks trigger reads the attacker"},
		{"ValidBlocked", "", " | ValidBlocked$ Creature.Black", "TriggeredBlockerLKICopy", "Effect Blocks trigger ValidBlocked$"},
		{"capture", " | ValidTgts$ Creature | RememberObjects$ Targeted", " | ValidCard$ Creature.IsRemembered", "TriggeredBlockerLKICopy", "Effect Blocks trigger with a capture"},
		{"until your next turn", " | Duration$ UntilYourNextTurn", "", "TriggeredBlockerLKICopy", "Effect triggers without a permanent duration"},
	} {
		script := fmt.Sprintf(blocksEmblemScript, tc.eff, tc.trig, tc.defined)
		c := lowerScriptCard(t, "Test Blocks Emblem", script)
		got := fmt.Sprint(c.Refusals)
		if tc.want == "" && len(c.Refusals) != 0 {
			t.Errorf("%s: refused %s, want it to lower", tc.name, got)
		}
		if tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("%s: refusals %s, want %q", tc.name, got, tc.want)
		}
	}
}
