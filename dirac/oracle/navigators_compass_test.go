package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Navigator's Compass in lockstep over the navcompass fixture pool: "{T}:
// Until end of turn, target land you control becomes the basic land type of
// your choice in addition to its other types." gorge effChooseType asks over
// the five basic land types on EVERY activation (CR 608.2: a second
// activation does not reuse the first answer), and the Animate | Types$
// ChosenType adds that type, and with it the type's mana ability (CR 305.6),
// to the target. The kernel lowers it to OpChooseType B =
// kop.ChooseTypeBasicLand and an OvChosenType override over five records
// (act_choosebasicland.go). Each deck holds a colour only the granted type
// can pay for (Savannah Lions without a Plains).
var navCompassDecks = [2][]cardset.Entry{
	{
		{Name: "Navigator's Compass", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Navigator's Compass", Count: 6}, {Name: "Raging Goblin", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Savannah Lions", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 3},
	},
}

// navCompassTally counts the basic land type asks gorge posed, and those
// posed while the decider already had a Compass with a recorded type (the
// second activation's ask, which the retired reuse skipped).
type navCompassTally struct{ asks, reasks int }

// navCompassProbe wraps castFirstPolicy: a Compass activation gorge offers is
// taken three times in four; a basic land type ask is counted.
func navCompassProbe(seed uint64, tl *navCompassTally) Policy {
	base := castFirstPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0xc0a5))
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		var act []Canon
		basic := 0
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			switch {
			case o.Kind == "type" && (o.Label == "Forest" || o.Label == "Island" || o.Label == "Mountain" || o.Label == "Plains" || o.Label == "Swamp"):
				basic++
			case o.Kind == "ability" && mbcOptionName(c, o) == "Navigator's Compass":
				act = append(act, x)
			}
		}
		if basic == 5 {
			tl.asks++
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
				if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Navigator's Compass" && ob.ChosenType != "" {
					tl.reasks++
					break
				}
			}
		}
		if len(act) > 0 && r.IntN(4) != 0 {
			a = act[r.IntN(len(act))]
		}
		return a
	})
}

// TestNavigatorsCompassLowers: the pool plays in lockstep, and the games ask
// the basic land type often, a second time on an already-chosen Compass too.
func TestNavigatorsCompassLowers(t *testing.T) {
	t.Parallel()
	var tl navCompassTally
	pol := func(seed uint64) Policy { return navCompassProbe(seed, &tl) }
	if div := lockstepRunPolicy(t, "navcompass", navCompassDecks, 120, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("%d basic land type asks, %d on a Compass that already chose", tl.asks, tl.reasks)
	if tl.asks < 40 || tl.reasks < 20 {
		t.Fatalf("%d basic land type asks (%d re-asks) over 120 seeds, want at least 40 (20): the pool does not exercise the Compass", tl.asks, tl.reasks)
	}
}

// TestNavigatorsCompassMutation: the lockstep catches the kernel applying a
// fixed type's record (Forest) whatever was chosen, and the kernel reusing a
// Compass's earlier answer instead of asking again.
func TestNavigatorsCompassMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag *bool
	}{
		{"fixed-type", &dirac.DebugChosenLandTypeFirst},
		{"reuse", &dirac.DebugChooseTypeReuse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := *tc.flag
			*tc.flag = true
			defer func() { *tc.flag = old }()
			var tl navCompassTally
			pol := func(seed uint64) Policy { return navCompassProbe(seed, &tl) }
			if div := lockstepRunPolicy(t, "navcompass", navCompassDecks, 120, false, nil, nil, pol); div == 0 {
				t.Fatalf("%s: no divergence in 120 seeds", tc.name)
			}
		})
	}
}
