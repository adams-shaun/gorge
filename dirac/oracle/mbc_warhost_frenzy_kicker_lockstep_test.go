package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const actFeeAnyColourKickerRepro = "act_fee_anycolour_funds_kicker"

// Signpost Scarecrow's "{2}: Add one mana of any colour" is a repeatable
// fee-only mana ability (CR 605.3a, 602.2b), so a payment can fund a pip no
// land makes by paying its fee from tapped lands. Warhost's Frenzy's kicker
// is {B}, and on a red-and-white board with the untapped Scarecrow the kicked
// {2}{R}+{B} cast is payable that way (pool tail-45 pair 36, seed 585512,
// decision 472: gorge's options alone carry cast(#38,-,1)). It is the class
// Shaman of the Great Hunt is refused for: gorge's census lists the paid
// ability and its search charges the fee from the floating mana (rules/pay
// feeFromMana), the kernel's census prices no fee-only mana ability beyond the
// trigger pay gate (mana_repeatpotential.go), and a fee-unit class in the
// planner search is far past bounded dirac code. Warhost's Frenzy is refused
// by name and its recorded game still diverges with it implicated.
func TestMBCWarhostSFrenzyReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Warhost's Frenzy" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != actFeeAnyColourKickerRepro {
			t.Fatalf("Warhost's Frenzy's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, actFeeAnyColourKickerRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Warhost's Frenzy has no dirac/support/refused entry: gorge offers its kicked cast through a repeatable fee-only mana ability the kernel never prices")
	}
}
