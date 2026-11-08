package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window class 3way:FX|G (FDN-s504/s505/s508, field `permanents`, Izzet
// Guildgate on the board): a creature its owner p0 holds under two Confiscates,
// p0's own and p1's, is seat 0's in gorge and seat 1's in Forge and XMage
// ("gorge c0 o0 High Fae Trickster vs forge c1 o0 High Fae Trickster @ setup").
// CR 613.7: control-changing effects apply in timestamp order and the later
// one wins. The setup places p0's Confiscate before p1's, so p1's is later and
// p1 controls the creature. gorge registered a static control grant only
// where it moved the bearer, so p0's older Confiscate, which changed nothing
// when it was attached, registered on a later emit, after p1's grant, and won
// by registration order. Izzet Guildgate is only a bystander on those boards.
func TestFarmIzzetGuildgate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, scenario string) map[string]OracleSnapPerm {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v", res.Fails)
		}
		out := map[string]OracleSnapPerm{}
		for _, p := range res.Snapshots[len(res.Snapshots)-1].Permanents {
			out[p.Ref] = p
		}
		return out
	}
	t.Run("opponent's later Confiscate wins over the owner's earlier one", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-two-confiscates",
		  "setup": {
		    "p0": {"battlefield": ["High Fae Trickster", "Confiscate", "Izzet Guildgate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]},
		    "p1": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}]}
		  },
		  "steps": [{"op": "pass", "seat": 0}]
		}`)
		// Precondition: both Auras are on the creature, so the two statics compete.
		for _, ref := range []string{"p0:Confiscate", "p1:Confiscate"} {
			if b[ref].AttachedTo != "p0:High Fae Trickster" {
				t.Fatalf("%s attached to %q, want the Trickster", ref, b[ref].AttachedTo)
			}
		}
		if got := b["p0:High Fae Trickster"]; got.Owner != 0 || got.Controller != 1 {
			t.Fatalf("Trickster is c%d o%d, want c1 o0 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
		}
	})
	t.Run("a later own Confiscate wins over the opponent's earlier one", func(t *testing.T) {
		t.Parallel()
		b := run(t, `{
		  "name": "farm-izzet-guildgate-later-own-confiscate",
		  "setup": {
		    "p0": {"battlefield": ["Confiscate"], "attached": [{"card": "Confiscate", "to": "p1:High Fae Trickster"}]},
		    "p1": {"battlefield": ["High Fae Trickster", "Confiscate"], "attached": [{"card": "Confiscate", "to": "p1:High Fae Trickster"}]}
		  },
		  "steps": [{"op": "pass", "seat": 0}]
		}`)
		for _, ref := range []string{"p0:Confiscate", "p1:Confiscate"} {
			if b[ref].AttachedTo != "p1:High Fae Trickster" {
				t.Fatalf("%s attached to %q, want the Trickster", ref, b[ref].AttachedTo)
			}
		}
		if got := b["p1:High Fae Trickster"]; got.Owner != 1 || got.Controller != 1 {
			t.Fatalf("Trickster is c%d o%d, want c1 o1 (p1's later Confiscate, CR 613.7)", got.Controller, got.Owner)
		}
	})
}
