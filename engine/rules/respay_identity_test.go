package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestResPayBoundsCommanderIdentityMana: with one untapped Arcane Signet under
// a red commander, a "you may pay" cost is fundable exactly when one red mana
// covers it (CR 903.4, 118.3). The payment bound used to price the Signet's
// ColorIdentity word unbounded in every colour, so a triggered "you may pay
// {5}{R}{R}" (Hellkite Charger) offered a pay that could only fail (dirac cmd5
// slice pair 1205).
func TestResPayBoundsCommanderIdentityMana(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	signet := corpusCommander(t, reg, "Arcane Signet")
	cmdr := card(t, "Name:Red Commander\nManaCost:1 R\nTypes:Legendary Creature Dragon\nPT:4/4\nOracle:x\n")
	e, _ := colourIdentityGame(t, 91, FormatCommander, cmdr, nil, signet)
	id := moveToBattlefieldByName(t, e, 0, "Arcane Signet")
	for _, tc := range []struct {
		cost string
		want bool
	}{
		{"R", true},
		{"1", true},
		{"U", false},
		{"5 R R", false},
		{"1 R", false},
	} {
		if got := e.resPayNeedsMana(0, id, ParseCost(tc.cost), paymentOther); got != tc.want {
			t.Errorf("resPayNeedsMana(%s) with one Signet under a red commander = %v, want %v", tc.cost, got, tc.want)
		}
	}
}
