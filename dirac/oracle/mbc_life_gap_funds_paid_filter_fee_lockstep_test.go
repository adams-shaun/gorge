package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Treasonous Ogre's "Pay 3 life: Add {R}" is a mana ability whose whole cost is
// life. gorge's planner has no unit for it, so a play only its mana pays is
// insufficient on the planner's walk and gorge's one prefix search activates
// the source; beside Unknown Shores' "{1}, {T}: Add one mana of any color" the
// life funds the filter's fee and the filter's mana casts Opt. The kernel had
// no lowering for the shape at all, so every such card was refused and the
// cast was never offered. The recording is on a custom pool, the table its own
// decks need: the card enters a synthetic pool only once it lowers.
func TestMBCLifeGapFundsPaidFilterFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_gap_funds_paid_filter_fee")
}

// The no-filter control: the same life-only ability pays for a Shock after the
// Mountains have tapped out, with no filter on the board. The seeds are the
// ones the first attempt diverged on (the option list and the floating pool):
// the replay must leave the floated life mana spent exactly as gorge does.
func TestLifeGapNoFilterControlReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		life bool
	}{
		{"life_gap_no_filter_control_seed7", true},
		{"life_gap_no_filter_control_seed22", false},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			life := false
			ec := effectLongCase{
				name:  c.name,
				decks: r.Cards,
				reached: func(g *GorgeSide) bool {
					for i := range g.E.L.Events {
						if e := &g.E.L.Events[i]; e.Kind == events.LifeChange && e.Amount == -3 {
							life = true
							return true
						}
					}
					return false
				},
			}
			res, _ := effectLongPlay(t, synthReg(t), ec, r, printedDelveSet(t, r.Cards), nil)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if c.life && !life {
				t.Fatal("the recorded game never paid the Ogre's life; the control no longer exercises the gap")
			}
		})
	}
}
