package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const altarPlanLandRepro = "altar_plan_land_choice"

// A planned cast that Ashnod's Altar part-funds, by sacrificing the spell's own
// target, still has to choose two lands among three identical untapped
// Mountains (pool tail-71, surface autopay, seed 2 pair 43, the whole game).
// gorge's plan takes the two lowest object ids in its source (battlefield)
// order, so it taps the Mountain played that turn (the lowest id) and spares
// the one that entered first. The kernel once tapped that other Mountain: the
// two engines differed in two tapped flags at decision 194 (CR 601.2g-h: the
// caster activates mana abilities, then pays).
func TestAltarPlanLandChoiceLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", altarPlanLandRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	type board struct {
		seq             int
		untapped        []state.ObjID // the caster's untapped Mountains, in battlefield order
		shocker         state.Zone
		spellOnTheStack bool
	}
	var asked, paid board // at Blind with Anger's first decision, and at its first after the sacrifice
	script := &ScriptPolicy{Actions: r.Actions}
	pol := PolicyFunc(func(c *PickCtx) Canon {
		g := c.G.E.G
		var spell *state.Object
		for _, id := range g.Stack {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Blind with Anger" {
				spell = o
			}
		}
		if spell != nil && !paid.spellOnTheStack {
			now := board{seq: c.Seq, spellOnTheStack: true}
			for _, id := range g.Zone(state.ZBattlefield, spell.Controller) {
				if o := g.Obj(id); o.Face() != nil && o.Face().Name == "Mountain" && !o.Tapped {
					now.untapped = append(now.untapped, id)
				}
			}
			for id := state.ObjID(1); g.Obj(id) != nil; id++ {
				if o := g.Obj(id); o.Face() != nil && o.Face().Name == "Shocker" {
					now.shocker = o.Zone
				}
			}
			if !asked.spellOnTheStack {
				asked = now
			} else if now.shocker == state.ZGraveyard {
				paid = now
			}
		}
		return script.Pick(c)
	})
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)

	// The preconditions: the cast was announced over a live Shocker with more
	// identical lands than the plan needs, and the altar then sacrificed it.
	if !asked.spellOnTheStack || asked.shocker != state.ZBattlefield || len(asked.untapped) != 3 {
		t.Fatalf("at the cast's announcement: on stack %v, Shocker zone %v, untapped Mountains %v; want it cast over a battlefield Shocker beside three untapped Mountains (divergence %v)",
			asked.spellOnTheStack, asked.shocker, asked.untapped, res.Div)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if msg := reproSkipFailure(r, script); msg != "" || script.next != len(r.Actions) {
		t.Fatalf("recording served %d of %d actions: %s", script.next, len(r.Actions), msg)
	}
	if !paid.spellOnTheStack {
		t.Fatalf("no decision after the altar sacrificed Shocker with Blind with Anger still on the stack (announced at %d)", asked.seq)
	}
	// Battlefield order's first Mountain stays untapped: the choice is by object
	// id, not by entry.
	spared := asked.untapped[0]
	if lowest := slices.Min(asked.untapped); spared == lowest {
		t.Fatalf("the first-entered Mountain %d is also the lowest id of %v: the game no longer tells the two orders apart", spared, asked.untapped)
	}
	if !slices.Equal(paid.untapped, []state.ObjID{spared}) {
		t.Fatalf("untapped Mountains after payment %v of %v, want only the first-entered %d", paid.untapped, asked.untapped, spared)
	}
}
