package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Card.StrictlySelf presence-gate and TriggeredAttacker$CardPower carriers
// play identically in gorge and dirac: four decks over the fixture pool cover
// every newly lowered card, one seat at a time. The StrictlySelf side is the
// eminence/present-zone family (a presence read in the trigger's own zone adds
// no gate) and the graveyard-presence family; the power side is the Attacks
// pump family (SVar X = TriggeredAttacker$CardPower, X is the attacker's live
// power). gorge leads every seed, so the policy reads the card each cast
// option names and prioritises the newly lowered ones, then falls back to
// castFirstPolicy.
var strictlySelfCards = map[string]bool{
	"Arahbo, Roar of the World": true, "Arden Angel": true, "Edgar Markov": true,
	"Emergent Woodwurm": true, "Inalla, Archmage Ritualist": true,
	"Jocasta, Automaton Avenger": true, "Level Up": true, "Lothlórien Blade": true,
	"Master of Death": true, "Old One Eye": true, "Sidar Jabari of Zhalfir": true,
	"Skyclave Shade": true,
}

var strictlySelfDeckSets = []struct {
	name  string
	decks [2][]cardset.Entry
}{
	{
		name: "strictlyself-gw-vs-wub",
		decks: [2][]cardset.Entry{
			{
				{Name: "Arahbo, Roar of the World", Count: 2}, {Name: "Arden Angel", Count: 2},
				{Name: "Emergent Woodwurm", Count: 2},
				{Name: "Level Up", Count: 2}, {Name: "Lothlórien Blade", Count: 2},
				{Name: "Jocasta, Automaton Avenger", Count: 2}, {Name: "Old One Eye", Count: 2},
				{Name: "Plains", Count: 14}, {Name: "Forest", Count: 12},
			},
			{
				{Name: "Edgar Markov", Count: 4}, {Name: "Master of Death", Count: 2},
				{Name: "Sidar Jabari of Zhalfir", Count: 2}, {Name: "Skyclave Shade", Count: 2},
				{Name: "Thought Scour", Count: 4}, {Name: "Walking Corpse", Count: 4},
				{Name: "Plains", Count: 7}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 3},
			},
		},
	},
	{
		name: "strictlyself-ubr-vs-wub",
		decks: [2][]cardset.Entry{
			{
				{Name: "Inalla, Archmage Ritualist", Count: 3}, {Name: "Master of Death", Count: 2},
				{Name: "Skyclave Shade", Count: 2}, {Name: "Thought Scour", Count: 4},
				{Name: "Walking Corpse", Count: 4}, {Name: "Island", Count: 9},
				{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
			},
			{
				{Name: "Edgar Markov", Count: 4}, {Name: "Master of Death", Count: 2},
				{Name: "Sidar Jabari of Zhalfir", Count: 2}, {Name: "Skyclave Shade", Count: 2},
				{Name: "Thought Scour", Count: 4}, {Name: "Walking Corpse", Count: 4},
				{Name: "Plains", Count: 7}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 3},
			},
		},
	},
}

// strictlySelfPolicy casts a newly lowered card whenever one is offered, and
// otherwise plays castFirstPolicy. played counts the cast offers taken.
func strictlySelfPolicy(seed uint64, played map[string]int) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Face() == nil || !strictlySelfCards[ob.Face().Name] {
					continue
				}
				played[ob.Face().Name]++
				return a
			}
		}
		return base.Pick(c)
	})
}

func TestStrictlySelfLockstep(t *testing.T) {
	t.Parallel()
	played := map[string]int{}
	for _, ds := range strictlySelfDeckSets {
		if n := lockstepRunPolicy(t, "strictlyself", ds.decks, 300, true, nil, nil,
			func(seed uint64) Policy { return strictlySelfPolicy(seed, played) }); n != 0 {
			t.Fatalf("%s: %d divergent seeds", ds.name, n)
		}
	}
	names := make([]string, 0, len(strictlySelfCards))
	for name := range strictlySelfCards {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if played[name] == 0 {
			t.Errorf("%s was never cast: the pool does not reach it", name)
		}
	}
}
