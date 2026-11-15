package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// CR 608.2h with CR 113.7a: Lothlórien Blade's "whenever equipped creature
// attacks, it deals damage equal to its power to target creature defending
// player controls" triggered for Watchwing Scarecrow, which Pit Trap
// destroyed before the trigger resolved. The ability exists independently of
// its source, and the departed attacker still deals the damage through its
// last known power (gorge triggeredAttackerLKI): Merchant of Truth is dealt
// 2. The kernel's Bite skipped the hit once its source had left, first
// difference projection Cards[13].Damage (decision 632 of tail-53 pair 790).
func TestMBCDepartedAttackerBiteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "departed_attacker_bites_with_last_known_power")
}

// TestMBCDepartedAttackerBiteMutation: with the departed attacker's bite
// skipped (the kernel before its last known power was dealt), the recording
// must diverge at the Damage projection. Not parallel: it sets a package
// mutation flag.
func TestMBCDepartedAttackerBiteMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "departed_attacker_bites_with_last_known_power.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugNoTrigBiteLKI = true
	defer func() { dirac.DebugNoTrigBiteLKI = false }()
	res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
	if res.Div == nil {
		t.Fatal("no divergence with the departed attacker's bite skipped")
	}
	if res.Div.Field != "Cards[13].Damage" {
		t.Fatalf("divergence at %s, want Cards[13].Damage", res.Div.Field)
	}
}
