package oracle

import (
	"path/filepath"
	"testing"
)

const gainsOfGraveyardManaCast = "gains_of_graveyard_mana_cast"

// CR 613.1f, 605.1a: Necrotic Ooze ("has all activated abilities of all
// creature cards in all graveyards") gains Drumhunter's "{T}: Add {C}" from
// the other seat's graveyard, and a gained mana ability is a mana ability:
// beside three Swamps, two Forests and a Manor Gate (chosen white) it makes
// Wit's End ({5}{B}{B}) payable at exactly seven mana. gorge's planner prices
// the cast (its payment window taps the Ooze for the {C}); the kernel's
// planner left a graveyard GainsOf's gained mana ability out of its census
// (mana_gainedgrants.go gainedIdxs skipped the zone), so it withheld the
// offer: pool tail-61 pair 12 parts at "options options" on decision 475.
func TestMBCWitSEndReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, gainsOfGraveyardManaCast)
}

// The recording's priority is passed in the recorded game, so the payment the
// census prices never runs in TestMBCWitSEndReprosLockstep: the free-play tail
// casts instead, and the Ooze's gained {T} must tap into the payment (CR
// 605.1a) without stranding the cast on either side.
func TestMBCWitSEndFreePlayTailLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", gainsOfGraveyardManaCast+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	sawCast := false
	pol := &ScriptPolicy{Actions: r.Actions, Tail: PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActCast {
				sawCast = true
			}
		}
		return castFirstPolicy(r.Seed).Pick(c)
	})}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	if !sawCast {
		t.Fatal("the free-play tail never posed a cast option: it did not cast Wit's End")
	}
	if res.Div != nil {
		t.Fatalf("free-play tail diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
