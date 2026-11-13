package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window FDN-s513 (xe:RAND:0:1:w24, class 3way:FX|G, field `permanents`):
// p0's High Fae Trickster sits under two Confiscates, p0's own and p1's, and
// Forge and XMage had p1 controlling it while gorge had p0, already at the
// `@ setup` checkpoint ("gorge c0 o0 High Fae Trickster vs forge c1 o0 High Fae
// Trickster"). CR 613.7: control-changing effects apply in timestamp order and
// the later one wins, so the setup places p0's Confiscate before p1's and p1
// controls the creature. The window predates the fix (FDN-s513 was stamped
// 2026-11-08T03:38:46Z; the fix's first commit, cb4ab50a69, is 09:16Z the same
// day; b3a1a7c91c, 3f0fb29db6 and e971bae22b complete it). The REPRO.md was
// pruned, so this pin rebuilds the Izzet Guildgate board with Crash Through
// added: the spell grants trample to `Creature.YouCtrl`, so it reads the
// post-layer-2 controller, and the bystanders must be unaffected.
func TestFarmCrashThrough(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(`{
	  "name": "farm-crash-through-two-confiscates",
	  "setup": {
	    "p0": {"battlefield": ["High Fae Trickster", "Confiscate", "Grizzly Bears"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]},
	    "p1": {"battlefield": ["Confiscate", "Tolarian Terror"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}], "hand": ["Crash Through"], "mana": "R"}
	  },
	  "steps": [
	    {"op": "pass_to", "seat": 1, "step": "main1", "active": "p1"},
	    {"op": "cast", "seat": 1, "card": "p1:Crash Through", "mana": "R"},
	    {"op": "resolve", "seat": 1},
	    {"op": "pass", "seat": 1}
	  ]
	}`))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	if len(res.Snapshots) < 2 {
		t.Fatalf("got %d snapshots, want at least the setup and final checkpoints", len(res.Snapshots))
	}
	perm := func(s OracleSnapshot, ref string) OracleSnapPerm {
		for _, p := range s.Permanents {
			if p.Ref == ref {
				return p
			}
		}
		t.Fatalf("no %s on the battlefield at %s", ref, s.Checkpoint)
		return OracleSnapPerm{}
	}

	// The farm's first differing value, and the precondition this pin turns
	// on: at setup both Auras are on the Trickster and p1's later one wins.
	if got := perm(res.Snapshots[0], "p0:High Fae Trickster"); got.Owner != 0 || got.Controller != 1 {
		t.Fatalf("setup Trickster is c%d o%d, want c1 o0 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
	}
	final := res.Snapshots[len(res.Snapshots)-1]
	for _, ref := range []string{"p0:Confiscate", "p1:Confiscate"} {
		if got := perm(final, ref); got.AttachedTo != "p0:High Fae Trickster" {
			t.Fatalf("%s attached to %q at %s, want the Trickster", ref, got.AttachedTo, final.Checkpoint)
		}
	}
	if got := perm(final, "p0:High Fae Trickster"); got.Owner != 0 || got.Controller != 1 {
		t.Fatalf("final Trickster is c%d o%d, want c1 o0 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
	}

	// Crash Through's trample goes to the creatures p1 controls: the stolen
	// Trickster and p1's own Terror, not p0's Bears.
	for _, ref := range []string{"p0:High Fae Trickster", "p1:Tolarian Terror"} {
		if got := perm(final, ref); !slices.Contains(got.Keywords, "Trample") {
			t.Fatalf("%s keywords = %v at %s, want Trample", ref, got.Keywords, final.Checkpoint)
		}
	}
	if got := perm(final, "p0:Grizzly Bears"); slices.Contains(got.Keywords, "Trample") {
		t.Fatalf("p0:Grizzly Bears keywords = %v at %s, want no Trample (p0 controls it)", got.Keywords, final.Checkpoint)
	}
}
