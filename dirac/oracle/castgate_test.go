package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The gorge-fix pool, two cards gorge once mis-played:
//   - Confront the Assault: "Cast this spell only if a creature is attacking
//     you" (a spell's IsPresent$ cast restriction, Trig.Cond on the spell
//     ability; attackingYou). gorge offered it always before
//     cr-spell-ispresent-cast-gate (feb134b15).
//   - Thraben Charm's third mode: exile any number of target players'
//     graveyards; with none targeted nothing moves. gorge swept every
//     graveyard before cr-changezoneall-zero-targets (56ba2e185).
//
// The expected answer at every decision is gorge's.
const (
	gorgeSpellPresentFixTime = "20261026092325"
	gorgeZoneAllZeroFixTime  = "20261026082922"
)

var kstd2gateDecks = [2][]cardset.Entry{
	{
		{Name: "Confront the Assault", Count: 6}, {Name: "Thraben Charm", Count: 6}, {Name: "Savannah Lions", Count: 6},
		{Name: "Healer's Hawk", Count: 3}, {Name: "Pacifism", Count: 3}, {Name: "Plains", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 8}, {Name: "Pacifism", Count: 3},
		{Name: "Thraben Charm", Count: 3}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 7},
	},
}

func kstd2gatePinned(t *testing.T) {
	t.Helper()
	if !gorgePinAtLeast(t, gorgeSpellPresentFixTime) || !gorgePinAtLeast(t, gorgeZoneAllZeroFixTime) {
		t.Skip("gorge before cr-spell-ispresent-cast-gate (feb134b15) / cr-changezoneall-zero-targets (56ba2e185)")
	}
}

func TestCastRestrictionGateLockstep(t *testing.T) {
	t.Parallel()
	kstd2gatePinned(t)
	lockstepSet(t, "castgate", kstd2gateDecks, 800, "KSTD2GATE_SEEDS")
}

// TestCastRestrictionGateLockstepMutation proves the pool reaches the cast
// restriction: with Confront the Assault's restriction dropped, some seed
// diverges.
func TestCastRestrictionGateLockstepMutation(t *testing.T) {
	t.Parallel()
	kstd2gatePinned(t)
	n := lockstepRun(t, "castgate", kstd2gateDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbSpell {
					abs[j].Trig.Cond = 0
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the cast restriction dropped")
	}
}
