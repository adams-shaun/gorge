package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The chosen-source shield's recipient and lifetime variants
// (kop.PrevNextChosenTo/Any/Turn, dirac/repl_prevchosen.go), played decision
// by decision against gorge's effEffect DamageDone replacement: the shield on
// the Effect's own target (Martyr's Cause and Sanctum Guardian over
// RememberObjects$ Targeted, Charm Peddler and Samite Blessing over
// ImprintCards$ Targeted), on any recipient (Pilgrim of Justice, Pilgrim of
// Virtue), and the bodyless Prevent$ True line that prevents every event from
// the source this turn (Pay No Heed, Auriok Replica, Burrenton Forge-Tender);
// and the shield whose body first gains the prevented life (kop.PrevNextGain:
// Reverse Damage, Cho-Arrim Alchemist, Intervention Pact).
var kchosenVariantDecks = [2][]cardset.Entry{
	{
		{Name: "Pay No Heed", Count: 2}, {Name: "Auriok Replica", Count: 2}, {Name: "Martyr's Cause", Count: 2},
		{Name: "Sanctum Guardian", Count: 2}, {Name: "Pilgrim of Justice", Count: 2}, {Name: "Pilgrim of Virtue", Count: 2},
		{Name: "Burrenton Forge-Tender", Count: 3}, {Name: "Charm Peddler", Count: 2}, {Name: "Samite Blessing", Count: 2},
		{Name: "Reverse Damage", Count: 2}, {Name: "Cho-Arrim Alchemist", Count: 2}, {Name: "Intervention Pact", Count: 1},
		{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 14},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Dusk Imp", Count: 4},
		{Name: "Shock", Count: 5}, {Name: "Lightning Bolt", Count: 5}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8},
	},
}

// kchosenVariantShields are the pool's shield cards: kchosenPolicyOf holds
// them until the blockers are declared and answers the pick with an attacker.
var kchosenVariantShields = map[string]bool{"Pay No Heed": true, "Auriok Replica": true, "Martyr's Cause": true,
	"Sanctum Guardian": true, "Pilgrim of Justice": true, "Pilgrim of Virtue": true, "Burrenton Forge-Tender": true,
	"Charm Peddler": true, "Samite Blessing": true, "Reverse Damage": true, "Cho-Arrim Alchemist": true,
	"Intervention Pact": true}

func kchosenVariantPolicy(seed uint64) Policy { return kchosenPolicyOf(seed, kchosenVariantShields) }

// kchosenVariantRun plays the pool's seeds with the shield policy (every
// eighth seed castFirstPolicy's random-free cast order) and returns the
// divergent games.
func kchosenVariantRun(t *testing.T, seeds uint64, report bool) int {
	t.Helper()
	return lockstepRunPolicy(t, "chosenshields", kchosenVariantDecks, seeds, report, nil, nil, func(seed uint64) Policy {
		if seed%8 == 0 {
			return castFirstPolicy(seed)
		}
		return kchosenVariantPolicy(seed)
	})
}

func TestChooseSourceVariantsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chosenshields", kchosenVariantDecks, 200, "KCHOSENSHIELDS_SEEDS")
}

// TestChooseSourceVariantsHeld: the same pool with the shields held for the
// blockers and the pick answered with an attacker, so they meet the damage.
func TestChooseSourceVariantsHeld(t *testing.T) {
	kchosenVariantRun(t, 200, true)
}

// TestChooseSourceVariantsMutation: with the shields installed but preventing
// nothing, some seed diverges, so the variants do prevent damage.
func TestChooseSourceVariantsMutation(t *testing.T) {
	dirac.DebugNoPrevChosen = true
	defer func() { dirac.DebugNoPrevChosen = false }()
	if kchosenVariantRun(t, 200, false) == 0 {
		t.Fatal("no seed diverged with the chosen-source shields disabled")
	}
}

// TestChooseSourceGainMutation: with the gain rider skipped (the damage still
// prevented), some seed diverges, so the rider runs.
func TestChooseSourceGainMutation(t *testing.T) {
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	if kchosenVariantRun(t, 200, false) == 0 {
		t.Fatal("no seed diverged with the chosen-source gain rider skipped")
	}
}
