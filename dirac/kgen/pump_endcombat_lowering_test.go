package kgen

import (
	"strings"
	"testing"
)

// TestPumpEndCombatLowering: Pump Duration$ UntilEndOfCombat on a battlefield
// object lowers as one override record the kernel drops at end of combat (the
// KW$ can't-block grant, Banding and a constant NumAtt$), while a computed
// amount and a player subject still refuse.
func TestPumpEndCombatLowering(t *testing.T) {
	const head = "T:Mode$ AttackerUnblocked | ValidCard$ Card.Self | Execute$ TrigPump | TriggerDescription$ x\n"
	body := func(pump string) string {
		return head + "SVar:TrigPump:DB$ Pump | " + pump + " | Duration$ UntilEndOfCombat\nSVar:X:Count$Valid Creature"
	}
	for _, c := range []struct{ name, pump, want string }{
		{"cant block", "Defined$ Self | KW$ HIDDEN CARDNAME can't block.", ""},
		{"cant attack and block", "Defined$ Self | KW$ HIDDEN CARDNAME can't attack. & HIDDEN CARDNAME can't block.", ""},
		{"constant pump", "Defined$ Self | NumAtt$ +2", ""},
		{"computed pump", "Defined$ Self | NumAtt$ X", "computed P/T pump"},
		{"banding", "Defined$ Self | KW$ Banding", ""},
		{"player subject", "Defined$ You | KW$ HIDDEN CARDNAME can't block.", `player-scoped Pump grant Duration "UntilEndOfCombat"`},
	} {
		rs := lowerSynthRefusals(t, body(c.pump))
		if c.want == "" {
			if len(rs) != 0 {
				t.Errorf("%s: refused: %v", c.name, rs)
			}
			continue
		}
		if len(rs) != 1 || !strings.Contains(rs[0].Reason, c.want) {
			t.Errorf("%s: refusals %v, want one containing %q", c.name, rs, c.want)
		}
	}
}
