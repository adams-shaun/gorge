package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const actFeeAnyColourRepro = "act_fee_anycolour_funds_hybrid"

// Signpost Scarecrow's "{2}: Add one mana of any colour" is a repeatable
// fee-only mana ability (no {T}, no limit), so a payment activates it as often
// as it can fund the fee (CR 605.3a, 602.2b). Shaman of the Great Hunt's
// Ferocious activation, {2}{G/U}{G/U}, is therefore legal on a red-and-white
// board with the untapped Scarecrow: the tapped lands pay the two {2} fees and
// the activation's {2}, and the Scarecrow's two activations make the {G/U}
// pips (pool tail-45 pair 41, seed 980704, decision 482: gorge's options alone
// carry activate(#73{Shaman of the Great Hunt})). Gorge's payment census lists
// the paid ability and its search charges each fee from the floating mana
// (rules/pay feeFromMana); the kernel's census prices no fee-only mana ability
// beyond the trigger pay gate (mana_repeatpotential.go), and a fee-unit class
// in the planner search and the manual window is far past bounded dirac code.
// Shaman of the Great Hunt is refused by name and its recorded game still
// diverges with it implicated.
func TestMBCShamanOfTheGreatHuntReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Shaman of the Great Hunt" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != actFeeAnyColourRepro {
			t.Fatalf("Shaman of the Great Hunt's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, actFeeAnyColourRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Shaman of the Great Hunt has no dirac/support/refused entry: gorge offers its Ferocious activation through a repeatable fee-only mana ability the kernel never prices")
	}
}
