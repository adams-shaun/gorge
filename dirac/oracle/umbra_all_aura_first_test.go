package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.89a and the 6/7/2024 Hyena Umbra ruling: a destroy-all that destroys
// an umbra Aura and the creature it enchants at the same time still saves the
// creature, whichever of the two the per-seat battlefield order visits first.
// The recording puts seat 0's Boar Umbra on seat 1's Grizzly Bears, so the Aura
// is the earlier victim, and then sweeps with seat 0's Planar Cleansing.
func TestUmbraAllAuraFirstReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/umbra_all_aura_first.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	seen := false
	pol := PolicyFunc(func(c *PickCtx) Canon {
		a := script.Pick(c)
		if c.D.Kind == DPriority && c.D.Player == 0 && c.G != nil && umbraAllAuraAttached(c.G) &&
			gowronName(c.G, a) == "Planar Cleansing" {
			seen = true
		}
		return a
	})
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if msg := reproSkipFailure(r, script); msg != "" {
		t.Fatal(msg)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !seen {
		t.Fatal("the recording never casts Planar Cleansing while the Aura is attached to the higher seat's creature")
	}
}

// umbraAllAuraAttached reports a Boar Umbra on the battlefield attached to a
// creature its controller does not control.
func umbraAllAuraAttached(g *GorgeSide) bool {
	gm := g.E.G
	for i := range gm.Objs {
		o := &gm.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Boar Umbra" || o.AttachedTo == 0 {
			continue
		}
		host := gm.Obj(o.AttachedTo)
		if host != nil && host.Zone == state.ZBattlefield && host.Controller != o.Controller {
			return true
		}
	}
	return false
}
