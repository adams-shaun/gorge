package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Commander-identity mana (CR 903.4: Arcane Signet, Command Tower, Commander's
// Sphere, Hidden Hideout) in a Commander game: gorge adds the colour identity
// of the controller's commanders, and its planner lists the source with the
// choice-shaped ones even when the identity is one colour. The kernel read it
// as no colour (TestCommanderIdentityManaIsNoSource pins the no-commander
// game), so it offered a cast gorge did and tapped another source for a cost
// the rock paid in gorge.
//
// firstAt is the decision the recording diverged at before the kernel read the
// identity (kernel mana_reflected.go identityColours).
var cmd5IdentityMana = []struct {
	name    string
	firstAt int
}{
	{"cmd5_identity_mana_s2_p14", 86}, // options: gorge offers the Signet's cast
	{"cmd5_identity_mana_s2_p1", 149}, // flags: gorge taps the Signet, the kernel a land
}

func cmd5IdentityReplay(t *testing.T, name string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("%s: pool %q, want custom", name, r.Pool)
	}
	return replayStdReproResult(t, synthReg(t), r.Pool, name, r.Surface == "full")
}

func TestCmd5IdentityManaLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range cmd5IdentityMana {
		res := cmd5IdentityReplay(t, c.name)
		if res.Div != nil {
			t.Errorf("%s: divergence at %d: %s %s %v", c.name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Decisions <= c.firstAt {
			t.Errorf("%s: %d decisions, the game must go past decision %d where the identity mana is read", c.name, res.Decisions, c.firstAt)
		}
	}
}

// TestCmd5IdentityManaLockstepMutation: the switch reads no identity, so the
// rock makes no mana. Each recording must diverge, at or before the decision
// it recorded.
func TestCmd5IdentityManaLockstepMutation(t *testing.T) {
	dirac.SetDebugIdentityMutate(1)
	defer dirac.SetDebugIdentityMutate(0)
	for _, c := range cmd5IdentityMana {
		res := cmd5IdentityReplay(t, c.name)
		if res.Div == nil {
			t.Errorf("%s: no divergence with the identity read off", c.name)
		} else if res.Div.At > c.firstAt {
			t.Errorf("%s: diverged at %d, after the recorded decision %d", c.name, res.Div.At, c.firstAt)
		}
	}
}
