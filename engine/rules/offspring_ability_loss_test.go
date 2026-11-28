package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// offspringStackGrantSrc is Zinnia, Valley's Voice's granting static alone
// (offspring {2} for creature spells you cast, on the stack), inline.
const offspringStackGrantSrc = "Name:Stack Muse\nManaCost:3\nTypes:Enchantment\n" +
	"S:Mode$ Continuous | Affected$ Creature.wasCastByYou | AffectedZone$ Stack | AddKeyword$ Offspring:2 | Description$ x\nOracle:x\n"

// TestGrantedOffspringUnderAbilityLoss pins CR 613.1f with CR 603.6d: an
// Offspring trigger (CR 702.175a) is an ability of the entering permanent,
// read as it exists on the battlefield. A "loses all abilities" effect that
// applies there removes it whatever its timestamp relative to a grant that
// reaches only the stack (that grant never applies on the battlefield, so it
// cannot be the newer layer-6 effect there). The control row keeps the
// granted token; the printed rows keep their existing (correct) answer.
func TestGrantedOffspringUnderAbilityLoss(t *testing.T) {
	t.Parallel()
	const plain = "Name:Plain Pup\nManaCost:1 G\nTypes:Creature Dog\nPT:2/2\nOracle:x\n"
	const printed = "Name:Printed Pup\nManaCost:1 G\nTypes:Creature Dog\nPT:2/2\nK:Offspring:1 G\nOracle:x\n"
	cases := []struct {
		name    string
		src     string
		statics []string
		want    int
	}{
		{"granted, grant then loss", plain, []string{offspringStackGrantSrc, mutateAbilityLossSrc}, 0},
		{"granted, loss then grant", plain, []string{mutateAbilityLossSrc, offspringStackGrantSrc}, 0},
		{"granted, no loss (control)", plain, []string{offspringStackGrantSrc}, 1},
		{"printed, grant then loss", printed, []string{offspringStackGrantSrc, mutateAbilityLossSrc}, 0},
		{"printed, loss only", printed, []string{mutateAbilityLossSrc}, 0},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, hero := newFixtureDeck(t, 9301+uint64(i), tc.src)
			toMain1(t, e)
			for _, s := range tc.statics {
				onBoard(t, e, 0, s)
			}
			if !e.hasCastOffspring(hero) {
				t.Fatalf("precondition: the cast does not carry Offspring: %v", e.derivedWith(hero, state.ZStack).Keywords)
			}
			addMana(t, e, 0, "GGCCCC")
			submitChoices(t, e, replicateOption(t, e, hero, "offspring").Index)
			passUntilStackEmpty(t, e, 20)
			if o := e.G.Obj(hero); o == nil || o.Zone != state.ZBattlefield || !o.OffspringPaid {
				t.Fatalf("precondition: the creature did not enter with its offspring cost paid: %+v", o)
			}
			if !offspringCastInfo(e, hero) {
				t.Fatal("precondition: no offspringpaid CastInfo")
			}
			lossOnBoard := len(tc.statics) > 1 || tc.statics[0] == mutateAbilityLossSrc
			if e.printedAbilitiesGone(e.G.Obj(hero)) != lossOnBoard {
				t.Fatalf("precondition: printedAbilitiesGone = %v, want %v", !lossOnBoard, lossOnBoard)
			}
			if got := triggersPushedFrom(e, hero); got != tc.want {
				t.Errorf("offspring triggers pushed = %d, want %d", got, tc.want)
			}
			if got := len(offspringTokens(t, e, hero)); got != tc.want {
				t.Errorf("offspring token copies = %d, want %d", got, tc.want)
			}
		})
	}
}
