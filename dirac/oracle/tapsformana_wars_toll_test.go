package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// War's Toll is the tapsformana pool's card with two modelled lines: the
// stack TapsForMana trigger (Activator$ Opponent, ValidCard$ Land; the body
// taps all the tapper's lands) and the AttackRequirement static ("If a
// creature an opponent controls attacks, all creatures that player controls
// attack if able", dirac/kgen static_combat.go + dirac/combat_attackrequirement.go).
// The decks put War's Toll on one side and a wide creature side on the other:
// the opponent's lands are deferred by the tap watcher, so its Llanowar Elves
// and Sol Ring carry its casts, and its attackers are the requirement's
// members.
func warTollDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "War's Toll", Count: 4},
			{Name: "Mountain", Count: 14},
			{Name: "Plains", Count: 6},
			{Name: "Grizzly Bears", Count: 8},
			{Name: "Samite Healer", Count: 4},
			{Name: "Shock", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 12},
			{Name: "Llanowar Elves", Count: 8},
			{Name: "Sol Ring", Count: 4},
			{Name: "Forest", Count: 16},
		},
	}
}

// TestTapsForManaWarTollLockstep replays the pool's games with War's Toll's
// two lines in the kernel's table; any divergence is a kernel/gorge mismatch
// in the tap watcher or the attack requirement.
func TestTapsForManaWarTollLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsformana", warTollDecks(), 150, "TAPSMANA_WARTOLL_SEEDS")
}

// dropWarTollAbility drops the abilities War's Toll's CardDef matches from the
// kernel's table (the mutation tests' mutator).
func dropWarTollAbility(tbl *kop.Table, drop func(kop.Ability) bool) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		if cards[i].Name != "War's Toll" {
			continue
		}
		keep := cards[i].Abilities[:0:0]
		for _, a := range cards[i].Abilities {
			if !drop(a) {
				keep = append(keep, a)
			}
		}
		cards[i].Abilities = keep
	}
	tbl.Cards = cards
}

// TestTapsForManaWarTollMutation proves the games exercise both lines: with
// the TapsForMana trigger dropped, or with the AttackRequirement static
// dropped, some seed diverges. A green lockstep alone would pass a pool where
// War's Toll's line never fires.
func TestTapsForManaWarTollMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		drop func(kop.Ability) bool
	}{
		{"taps_line", func(a kop.Ability) bool {
			return a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps &&
				a.Trig.Flags5&kop.Tf5TapsForMana != 0
		}},
		{"attack_requirement", func(a kop.Ability) bool {
			return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAttackRequirement
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := lockstepRun(t, "tapsformana", warTollDecks(), 150, false, func(tbl *kop.Table) {
				dropWarTollAbility(tbl, tc.drop)
			})
			if n == 0 {
				t.Fatal("no seed diverged with War's Toll's line dropped")
			}
		})
	}
}
