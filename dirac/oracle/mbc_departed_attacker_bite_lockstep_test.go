package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

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
