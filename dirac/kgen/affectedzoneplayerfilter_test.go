package kgen

import (
	"fmt"
	"strings"
	"testing"
)

// TestAffectedZonePlayerFilterRefuses: an off-battlefield grant with a
// player-spec Affected$ filter keeps the old refusal. The build would stamp
// a player half on the off-zone grant, which gorge projects onto every card
// of the zone. The object-only twin lowers, so the refusal is the player half.
func TestAffectedZonePlayerFilterRefuses(t *testing.T) {
	const tmpl = `Name:Test Zone Grant
ManaCost:1
Types:Enchantment
S:Mode$ Continuous | Affected$ %s | AffectedZone$ All | AddKeyword$ Hexproof | Description$ test
`
	for _, tc := range []struct {
		aff    string
		refuse bool
	}{{"You", true}, {"Card.YouOwn", false}} {
		c := lowerScriptCard(t, "Test Zone Grant", strings.Replace(tmpl, "%s", tc.aff, 1))
		got := strings.Contains(fmt.Sprint(c.Refusals), "without MayPlay")
		if got != tc.refuse {
			t.Errorf("Affected$ %s: refusals %v, want refused=%v", tc.aff, c.Refusals, tc.refuse)
		}
	}
}
