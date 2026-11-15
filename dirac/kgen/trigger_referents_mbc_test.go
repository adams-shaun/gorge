package kgen

import (
	"strings"
	"testing"
)

// TestTriggerReferents10: the spelling guards of the MBC trigger-referents-10
// lowerings (dirac/oracle TestMBCTriggerReferents10Lowers plays them against
// gorge).
func TestTriggerReferents10(t *testing.T) {
	has := func(rs []refusal, want string) bool {
		for _, r := range rs {
			if strings.Contains(r.Reason, want) {
				return true
			}
		}
		return false
	}
	const destroy = "DB$ Destroy | Defined$ TriggeredTarget"
	// a players-only DamageDoneOnce line names the damaged player as the
	// trigger player (ddoRecipient), so its body may read TriggeredTarget
	for _, vt := range []string{"Player", "Opponent", "You"} {
		if rs := kddoRefusals(map[string]string{"ValidTarget": vt, "CombatDamage": "True"}, destroy); has(rs, "body reads TriggeredTarget") {
			t.Errorf("ValidTarget$ %s: refused: %v", vt, rs)
		}
	}
	// no ValidTarget$ (object or player) or a mixed one is not a single referent kind
	if rs := kddoRefusals(map[string]string{}, destroy); !has(rs, "body reads TriggeredTarget") {
		t.Errorf("no ValidTarget$: not refused: %v", rs)
	}

	// Discarded ValidCause$: only the card's own OppCtrl cause lowers
	for _, c := range []struct {
		m    map[string]string
		ok   bool
		flag bool
	}{
		{map[string]string{"ValidCard": "Card.Self", "ValidCause": "SpellAbility.OppCtrl"}, true, true},
		{map[string]string{"ValidCard": "Card.Self"}, true, false},
		{map[string]string{"ValidCard": "Card.Self", "ValidCause": "SpellAbility.YouCtrl"}, false, false},
		{map[string]string{"ValidCard": "Card.YouOwn", "ValidCause": "SpellAbility.OppCtrl"}, false, false},
	} {
		tr := &lTrigger{}
		err := lowerDiscardedCause(newParams(c.m), tr)
		if (err == nil) != c.ok || tr.OppCause != c.flag {
			t.Errorf("%v: err=%v OppCause=%v, want ok=%v OppCause=%v", c.m, err, tr.OppCause, c.ok, c.flag)
		}
	}
}
