package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// xIdentityEngineSrc is an artifact whose only ability costs {X}: the X ask
// is bounded by the mana the CR 601.2g window can still make.
const xIdentityEngineSrc = "Name:X Engine\nTypes:Artifact\n" +
	"A:AB$ GainLife | Cost$ X | LifeAmount$ X | Defined$ You | SpellDescription$ You gain X life.\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// TestXAskCountsCommanderIdentityMana: an untapped Arcane Signet under a red
// commander makes one red mana, which pays generic {X} (CR 903.4, 107.3), so
// with one Mountain beside it the {X} ask offers X up to 2. The cast window's
// probe priced the Signet's ColorIdentity production as no mana and offered
// only up to 1 (dirac cmd5 slice pair 657, Steel Hellkite). Under a colourless
// commander the Signet makes nothing (CR 903.4f) and the bound stays 1.
func TestXAskCountsCommanderIdentityMana(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, cmdr string
		want       []int
	}{
		{"red", "Name:Red Commander\nManaCost:1 R\nTypes:Legendary Creature Dragon\nPT:4/4\nOracle:x\n", []int{0, 1, 2}},
		{"colourless", "Name:Grey Commander\nManaCost:3\nTypes:Legendary Artifact Creature Golem\nPT:3/3\nOracle:x\n", []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			signet := corpusCommander(t, reg, "Arcane Signet")
			xe := card(t, xIdentityEngineSrc)
			e, _ := colourIdentityGame(t, 92, FormatCommander, card(t, tc.cmdr), nil, signet, xe)
			moveToBattlefieldByName(t, e, 0, "Arcane Signet")
			moveToBattlefieldByName(t, e, 0, "Mountain")
			id := moveToBattlefieldByName(t, e, 0, "X Engine")
			e.priorityRound()
			submitChoices(t, e, abilityOption(t, e, id, 0).Index)
			d := e.Pending()
			if d == nil {
				t.Fatal("no decision after activating the {X} ability")
			}
			var got []int
			for _, o := range d.Options {
				if o.Kind == "x" {
					got = append(got, o.Amount)
				}
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("X offered %v with a Signet and a Mountain under a %s commander, want %v", got, tc.name, tc.want)
			}
		})
	}
}
