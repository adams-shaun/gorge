package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Card.StrictlySelf presence-gate and TriggeredAttacker$CardPower carriers
// play identically in gorge and dirac: four decks over the fixture pool cover
// every newly lowered card, one seat at a time. The StrictlySelf side is the
// eminence/present-zone family (a presence read in the trigger's own zone is
// the source's live zone) and the graveyard-presence family; the power side is
// the Attacks pump family (SVar X = TriggeredAttacker$CardPower, X is the
// attacker's live power). The commander matchup below puts Edgar Markov in a command
// zone, so the eminence trigger's PresentZone$ Command branch fires too. gorge
// leads every seed, so the policy reads the card each cast option names and
// prioritises the newly lowered ones, then falls back to castFirstPolicy.
var strictlySelfCards = map[string]bool{
	"Arahbo, Roar of the World": true, "Arden Angel": true, "Charforger": true,
	"Edgar Markov": true, "Emergent Woodwurm": true, "Genesis": true,
	"Gigapede": true, "Inalla, Archmage Ritualist": true,
	"Jocasta, Automaton Avenger": true, "Level Up": true, "Lothlórien Blade": true,
	"Master of Death": true, "Old One Eye": true, "Pyre Zombie": true,
	"Sidar Jabari of Zhalfir": true, "Skyclave Shade": true,
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
				{Name: "Emergent Woodwurm", Count: 2}, {Name: "Genesis", Count: 2},
				{Name: "Gigapede", Count: 2},
				{Name: "Level Up", Count: 2}, {Name: "Lothlórien Blade", Count: 2},
				{Name: "Jocasta, Automaton Avenger", Count: 2}, {Name: "Old One Eye", Count: 2},
				{Name: "Plains", Count: 12}, {Name: "Forest", Count: 10},
			},
			{
				{Name: "Edgar Markov", Count: 4}, {Name: "Charforger", Count: 2},
				{Name: "Master of Death", Count: 2}, {Name: "Pyre Zombie", Count: 2},
				{Name: "Sidar Jabari of Zhalfir", Count: 2}, {Name: "Skyclave Shade", Count: 2},
				{Name: "Thought Scour", Count: 4}, {Name: "Walking Corpse", Count: 4},
				{Name: "Plains", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 4},
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

// strictlySelfCommanderDecks is a mirror Commander pair: Edgar Markov is each
// seat's commander (first deck entry) and further Edgar copies in the deck are
// the Vampire spells the policy casts, so the Command-zone eminence trigger runs
// while the commander stays in the command zone. Eminence's token is created
// without a cost or a copy of an entering object, so the game exercises the
// Command presence branch alone.
var strictlySelfCommanderDecks = [2][]cardset.Entry{
	{
		{Name: "Edgar Markov", Count: 7},
		{Name: "Walking Corpse", Count: 8}, {Name: "Thought Scour", Count: 5},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Edgar Markov", Count: 7},
		{Name: "Walking Corpse", Count: 8}, {Name: "Thought Scour", Count: 5},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5},
		{Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
	},
}

// isCommandZoneCast reports whether a is a cast of the commander from the
// command zone. The commander policy keeps it there so the Command presence
// branch is the one under test.
func isCommandZoneCast(c *PickCtx, a Canon) bool {
	if a.Kind() != ActCast || c.G == nil {
		return false
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	ob := c.G.E.G.Obj(o.Obj)
	return ob != nil && ob.Zone == state.ZCommand
}

// strictlySelfCommanderPolicy casts the newly lowered non-commander cards (an
// Edgar Markov cast from hand is the Vampire spell its command-zone twin
// watches), then falls back to castFirstPolicy but never casts the commander
// from the command zone.
func strictlySelfCommanderPolicy(seed uint64, vampireCasts *int) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast || isCommandZoneCast(c, a) {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Face() == nil {
					continue
				}
				name := ob.Face().Name
				if strictlySelfCards[name] {
					if name == "Edgar Markov" {
						*vampireCasts++
					}
					return a
				}
			}
		}
		a := base.Pick(c)
		if isCommandZoneCast(c, a) {
			for _, o := range c.D.Options {
				if !isCommandZoneCast(c, o) {
					return o
				}
			}
		}
		return a
	})
}

// TestStrictlySelfCommanderLockstep plays the Command-zone half: Edgar Markov
// sits in each seat's command zone while another Edgar (a Vampire spell) enters,
// so the eminence intervening-if reads PresentZone$ Command at both trigger time
// and resolution. The policy keeps the commander in the command zone, so the
// trigger fires only through that branch.
func TestStrictlySelfCommanderLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("strictlyself")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range strictlySelfCommanderDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("commander deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(strictlySelfCommanderDecks[0], strictlySelfCommanderDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	vampireCasts := 0
	for seed := uint64(1); seed <= 200; seed++ {
		m, err := NewMatchup(reg, [2]string{"strictlyself-cmd-a", "strictlyself-cmd-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		m.Commander, m.Commanders = true, [2]int{0, 0}
		lead, err := NewGorgeSide(m, seed, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for s := 0; s < 2; s++ {
			pl := &lead.E.G.Players[s]
			if len(pl.Commanders) != 1 || lead.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand {
				t.Fatalf("seed %d seat %d: commander %v is not in the command zone", seed, s, pl.Commanders)
			}
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, strictlySelfCommanderPolicy(seed, &vampireCasts),
			RunConfig{Config: Config{}, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
	}
	if vampireCasts == 0 {
		t.Fatal("no Edgar Markov cast from hand while the commander was in the command zone: the pool does not reach the Command presence branch")
	}
	t.Logf("commander: %d Vampire spells cast under a command-zone Edgar Markov", vampireCasts)
}
