package oracle

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Dream Thrush in lockstep over the dreamthrush fixture pool: "{T}: Target
// land becomes the basic land type of your choice until end of turn." gorge
// asks over the five basic land types on every activation and the Animate |
// Types$ ChosenType | RemoveLandTypes$ True is a CR 305.7 land-subtype setter:
// the land's land types become the chosen one, it loses the abilities its
// rules text generates and keeps the intrinsic mana ability of the type it was
// set to (CR 305.6). The kernel lowers it to OpChooseType B =
// kop.ChooseTypeBasicLand and an OvChosenType override over five kop.Override
// SetLandSub records (act_choosebasicland.go, static_landtypesetter.go). Each deck holds
// colours only a set land can pay for (Savannah Lions without a Plains).
var dreamThrushDecks = [2][]cardset.Entry{
	{
		{Name: "Dream Thrush", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Dream Thrush", Count: 6}, {Name: "Raging Goblin", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Savannah Lions", Count: 2},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3},
	},
}

// dreamThrushTally counts the basic land type asks gorge posed and the
// activations the probe took.
type dreamThrushTally struct{ asks, acts int }

// dreamThrushProbe wraps castFirstPolicy: a Dream Thrush activation gorge
// offers is taken three times in four; a basic land type ask is counted.
func dreamThrushProbe(seed uint64, tl *dreamThrushTally, names ...string) Policy {
	base := castFirstPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0xd7e5))
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
			case o.Kind == "ability" && slices.Contains(names, mbcOptionName(c, o)):
				act = append(act, x)
			}
		}
		if basic == 5 {
			tl.asks++
		}
		if len(act) > 0 && r.IntN(4) != 0 {
			tl.acts++
			a = act[r.IntN(len(act))]
		}
		return a
	})
}

// TestDreamThrushLowers: the pool plays in lockstep, and the games ask the
// basic land type often.
func TestDreamThrushLowers(t *testing.T) {
	t.Parallel()
	var tl dreamThrushTally
	pol := func(seed uint64) Policy { return dreamThrushProbe(seed, &tl, "Dream Thrush") }
	if div := lockstepRunPolicy(t, "dreamthrush", dreamThrushDecks, 120, true, nil, nil, pol); div != 0 {
		return // each divergence is already a test error
	}
	t.Logf("%d basic land type asks, %d Thrush activations taken", tl.asks, tl.acts)
	if tl.asks < 40 {
		t.Fatalf("%d basic land type asks over 120 seeds, want at least 40: the pool does not exercise the Thrush", tl.asks)
	}
}

// TestDreamThrushMutation: the lockstep catches the kernel applying a fixed
// type's record (Forest) whatever was chosen, and the kernel taking the set
// land keep its printed mana abilities beside the intrinsic one.
func TestDreamThrushMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag *bool
	}{
		{"fixed-type", &dirac.DebugChosenLandTypeFirst},
		{"own-mana", &dirac.DebugLandSetOwnMana},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := *tc.flag
			*tc.flag = true
			defer func() { *tc.flag = old }()
			var tl dreamThrushTally
			pol := func(seed uint64) Policy { return dreamThrushProbe(seed, &tl, "Dream Thrush") }
			if div := lockstepRunPolicy(t, "dreamthrush", dreamThrushDecks, 120, false, nil, nil, pol); div == 0 {
				t.Fatalf("%s: no divergence in 120 seeds", tc.name)
			}
		})
	}
}

// chosenLandFamilyDecks: the other cards of Dream Thrush's spelling -- the
// creature and artifact abilities (Moonbow Illusionist's return-a-land cost,
// Tideshaper Mystic's own-turn limit, the "you control" targets, Unstable
// Frontier as its own target, a land whose own ability is not the plain
// shape) and the spells (Jinx, Shimmering Mirage).
var chosenLandFamilyDecks = [2][]cardset.Entry{
	{
		{Name: "Jinx", Count: 3}, {Name: "Shimmering Mirage", Count: 3}, {Name: "Unstable Frontier", Count: 3},
		{Name: "Tideshaper Mystic", Count: 3}, {Name: "Moonbow Illusionist", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 2}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Grixis Illusionist", Count: 3}, {Name: "Pixie Illusionist", Count: 3}, {Name: "Reef Shaman", Count: 3},
		{Name: "Sea Snidd", Count: 2}, {Name: "Mystic Compass", Count: 3},
		{Name: "Raging Goblin", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 5},
	},
}

// TestChosenLandTypeFamilyLowers: the other cards of the same spelling play in
// lockstep too.
func TestChosenLandTypeFamilyLowers(t *testing.T) {
	t.Parallel()
	var tl dreamThrushTally
	names := []string{"Tideshaper Mystic", "Moonbow Illusionist", "Unstable Frontier", "Grixis Illusionist", "Pixie Illusionist", "Reef Shaman", "Sea Snidd", "Mystic Compass"}
	pol := func(seed uint64) Policy { return dreamThrushProbe(seed, &tl, names...) }
	if div := lockstepRunPolicy(t, "dreamthrush", chosenLandFamilyDecks, 120, true, nil, nil, pol); div != 0 {
		return
	}
	t.Logf("%d basic land type asks, %d activations taken", tl.asks, tl.acts)
	if tl.asks < 40 {
		t.Fatalf("%d basic land type asks over 120 seeds, want at least 40", tl.asks)
	}
}
