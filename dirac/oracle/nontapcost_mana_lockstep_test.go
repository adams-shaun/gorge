package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// Mana abilities whose cost goes beyond {T}, a sacrifice of the source and a
// plain fixed mana: a sacrifice of another permanent.
//
//   - Phyrexian Altar's "Sacrifice a creature: Add one mana of any color" (no
//     {T}, so gorge's census leaves it an unrelaxable gap) and Gilded Goose's
//     "{T}, Sacrifice a Food: Add one mana of any color";
//   - Phyrexian Tower's second ability, "{T}, Sacrifice a creature: Add {B}{B}",
//     beside its plain "{T}: Add {C}".
//
// A play only their mana pays is found by gorge's prefix script, which
// activates the source (a second ability answers its ability ask), asks which
// permanent to sacrifice (only with two or more candidates, battlefield
// order), then the colour of an any-colour one. The kernel models them as
// sources of the tap-another search (dirac/mana_tapanother.go).
//
// Player 0's lands make no red (or black), so the spells that need it are paid
// only by the sources under test. The table is built from the decks' own cards
// (printedDelveSet), as a lowered card with no evidence has no pool yet.
type nonTapCostCase struct {
	name   string
	deck   [2][]cardset.Entry
	spells []string // the spells only the sources pay, cast first
	cast   []string // every other card to cast
	source string   // the permanent whose mana the spells need
	// victims: the cards whose sacrifice the games must see (the source's
	// cost sacrifices them; none: the source sacrifices nothing)
	victims []string
	// multiCtr: the games must see the source remove two or more counters in
	// one activation (Runaway Steam-Kin's three +1/+1)
	multiCtr bool
}

var nonTapCostCases = []nonTapCostCase{
	{
		name: "any-colour",
		deck: [2][]cardset.Entry{
			{
				{Name: "Phyrexian Altar", Count: 4}, {Name: "Gilded Goose", Count: 4}, {Name: "Grizzly Bears", Count: 8},
				{Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Raging Goblin", "Hill Giant"},
		cast:   []string{"Phyrexian Altar", "Gilded Goose", "Grizzly Bears"},
		source: "Phyrexian Altar",
		// creatures only: Gilded Goose's Food is no creature, so a game counts
		// only the Altar's sacrifices
		victims: []string{"Grizzly Bears", "Raging Goblin", "Hill Giant", "Gilded Goose"},
	},
	{
		// Phyrexian Altar beside Abundant Growth's granted "{T}: Add one mana
		// of any color" on a Forest: gorge's one prefix search activates both,
		// the Altar asking the creature, then the colour (grant search,
		// dirac/mana_grantsacx.go)
		name: "altar-grant",
		deck: [2][]cardset.Entry{
			{
				{Name: "Phyrexian Altar", Count: 2}, {Name: "Abundant Growth", Count: 6}, {Name: "Grizzly Bears", Count: 8},
				{Name: "Furnace Whelp", Count: 8}, {Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:  []string{"Furnace Whelp"},
		cast:    []string{"Phyrexian Altar", "Abundant Growth", "Grizzly Bears"},
		source:  "Phyrexian Altar",
		victims: []string{"Grizzly Bears", "Furnace Whelp"},
	},
	{
		// Gilded Goose beside Abundant Growth: its Food sacrifice, then the
		// colour, in the grant search
		name: "goose-grant",
		deck: [2][]cardset.Entry{
			{
				{Name: "Gilded Goose", Count: 6}, {Name: "Abundant Growth", Count: 6}, {Name: "Raging Goblin", Count: 8},
				{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:  []string{"Raging Goblin"},
		cast:    []string{"Gilded Goose", "Abundant Growth", "Grizzly Bears"},
		source:  "Gilded Goose",
		victims: []string{"Food Token"},
	},
	{
		name: "tower",
		deck: [2][]cardset.Entry{
			{
				{Name: "Phyrexian Tower", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Black Knight", Count: 8},
				{Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:  []string{"Black Knight"},
		cast:    []string{"Grizzly Bears"},
		source:  "Phyrexian Tower",
		victims: []string{"Grizzly Bears", "Black Knight"},
	},
	{
		// Gilded Goose under Cryptolith Rite's granted "{T}: Add one mana of
		// any color": the holder's ask lists its printed Food sacrifice
		// first, then the grant (dirac/mana_grantsacx.go printedXFirst)
		name: "goose-rite",
		deck: [2][]cardset.Entry{
			{
				{Name: "Gilded Goose", Count: 6}, {Name: "Cryptolith Rite", Count: 4}, {Name: "Raging Goblin", Count: 8},
				{Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:  []string{"Raging Goblin"},
		cast:    []string{"Gilded Goose", "Cryptolith Rite", "Grizzly Bears"},
		source:  "Gilded Goose",
		victims: []string{"Food Token"},
	},
	{
		// Phyrexian Tower beside Abundant Growth: the grant search's
		// candidate asks the Tower's ability first (its plain {C}), then the
		// creature to sacrifice (dirac/mana_grantsacx.go sacXPlain)
		name: "tower-grant",
		deck: [2][]cardset.Entry{
			{
				{Name: "Phyrexian Tower", Count: 6}, {Name: "Abundant Growth", Count: 6}, {Name: "Grizzly Bears", Count: 8},
				{Name: "Black Knight", Count: 8}, {Name: "Forest", Count: 12},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:  []string{"Black Knight"},
		cast:    []string{"Abundant Growth", "Grizzly Bears"},
		source:  "Phyrexian Tower",
		victims: []string{"Grizzly Bears", "Black Knight"},
	},
	{
		// Urza's "Tap an untapped artifact you control: Add {U}" (no {T}), with
		// Ornithopters to tap and two Islands, so Phantom Warrior's {U}{U} wants it
		name: "urza",
		deck: [2][]cardset.Entry{
			{
				{Name: "Urza, Lord High Artificer", Count: 2}, {Name: "Ornithopter", Count: 8}, {Name: "Phantom Warrior", Count: 8},
				{Name: "Grizzly Bears", Count: 6}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 12},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Phantom Warrior"},
		cast:   []string{"Ornithopter", "Urza, Lord High Artificer", "Grizzly Bears"},
		source: "Urza, Lord High Artificer",
	},
	{
		// Urza beside Abundant Growth: the no-{T} tap-one source is a grant
		// search candidate (dirac/mana_granttapx.go), its fixed {U} asked no
		// colour
		name: "urza-grant",
		deck: [2][]cardset.Entry{
			{
				{Name: "Urza, Lord High Artificer", Count: 4}, {Name: "Abundant Growth", Count: 4}, {Name: "Ornithopter", Count: 4},
				{Name: "Stormchaser Mage", Count: 8}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 16},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Stormchaser Mage"},
		cast:   []string{"Ornithopter", "Urza, Lord High Artificer", "Abundant Growth"},
		source: "Urza, Lord High Artificer",
	},
	{
		// Relic of Legends' second ability, "Tap an untapped legendary creature
		// you control: Add one mana of any color", beside its {T}: Furnace
		// Whelp's {R}{R} is paid by the two
		name: "relic",
		deck: [2][]cardset.Entry{
			{
				{Name: "Relic of Legends", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Furnace Whelp", Count: 8},
				{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 12},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Furnace Whelp"},
		cast:   []string{"Isamaru, Hound of Konda", "Relic of Legends", "Grizzly Bears"},
		source: "Relic of Legends",
	},
	{
		// Relic of Legends beside Abundant Growth: its second ability's
		// colour ask in the grant search, the play's colours first
		// (dirac/mana_granttapx.go tapXBranch)
		name: "relic-grant",
		deck: [2][]cardset.Entry{
			{
				{Name: "Relic of Legends", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Abundant Growth", Count: 4},
				{Name: "Furnace Whelp", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 12},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Furnace Whelp"},
		cast:   []string{"Isamaru, Hound of Konda", "Relic of Legends", "Abundant Growth"},
		source: "Relic of Legends",
	},
	{
		// Seton, Krosan Protector's "Tap an untapped Druid you control: Add {G}":
		// a Druid itself, so its own tap is a candidate beside the Mystics'
		// (no {T} in its cost); Overrun's {G}{G}{G} wants it with two Forests
		name: "seton",
		deck: [2][]cardset.Entry{
			{
				{Name: "Seton, Krosan Protector", Count: 2}, {Name: "Elvish Mystic", Count: 6}, {Name: "Overrun", Count: 8},
				{Name: "Forest", Count: 4}, {Name: "Mountain", Count: 20},
			},
			{{Name: "Island", Count: 40}},
		},
		spells: []string{"Overrun"},
		cast:   []string{"Elvish Mystic", "Seton, Krosan Protector"},
		source: "Seton, Krosan Protector",
	},
	{
		// Pentad Prism's "Remove a charge counter from this: Add one mana of
		// any color" and Runaway Steam-Kin's "Remove three +1/+1 counters from
		// this: Add {R}{R}{R}", neither costing {T} nor limited to once a
		// turn: gorge's census leaves each a repeatable, unrelaxable gap its
		// prefix search activates, so one source may pay several activations
		// (Pentad Prism spends its two charge counters in one game) and one
		// activation removes more than one counter (Runaway's three). The
		// lands make no red, so the red spells are paid only by these two;
		// Runaway's counters come from the red spells Pentad's charge counters
		// pay, and its three-counter activation is the last red source.
		name: "counter",
		deck: [2][]cardset.Entry{
			{
				{Name: "Pentad Prism", Count: 4}, {Name: "Runaway Steam-Kin", Count: 4}, {Name: "Raging Goblin", Count: 8},
				{Name: "Hill Giant", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
			},
			{{Name: "Island", Count: 40}},
		},
		spells:   []string{"Raging Goblin", "Hill Giant"},
		cast:     []string{"Pentad Prism", "Runaway Steam-Kin"},
		source:   "Runaway Steam-Kin",
		multiCtr: true,
	},
}

func (c *nonTapCostCase) names() [2][]string {
	var out [2][]string
	for i, d := range c.deck {
		for _, e := range d {
			out[i] = append(out[i], e.Name)
		}
	}
	return out
}

func TestNonTapCostManaLockstep(t *testing.T) {
	for i := range nonTapCostCases {
		c := &nonTapCostCases[i]
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			tl := c.run(t, 24, true, 30)
			t.Logf("games %d: spells cast %d (beside %s %d), sacrifices %d", tl.games, tl.paid, c.source, tl.src, tl.sacs)
			if tl.src == 0 || len(c.victims) != 0 && tl.sacs == 0 {
				t.Fatalf("spells cast beside %s %d, sacrifices %d: the activation was not exercised", c.source, tl.src, tl.sacs)
			}
			if c.multiCtr && tl.bigCtr == 0 {
				t.Fatalf("no activation of %s removed two or more counters: the multi-counter cost was not exercised", c.source)
			}
		})
	}
}

// TestNonTapCostManaLockstepMutation hides the tap-another and sacrifice-another sources in the
// kernel only: the spells they pay are then gorge's alone, and lockstep
// diverges.
func TestNonTapCostManaLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapMana = true
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoTapMana, dirac.DebugNoCtrMana = false, false }()
	for i := range nonTapCostCases {
		c := &nonTapCostCases[i]
		if tl := c.run(t, 24, false, 30); tl.divs == 0 {
			t.Fatalf("%s: no seed diverged without the tap-another sources: the scenario never needs their mana", c.name)
		}
	}
}

// canonNames maps each option string to the first card it names.
func canonNames(m *Matchup, opts []string) []string {
	var out []string
	for _, o := range opts {
		if n := firstObjectCard(m, o); n != "" {
			out = append(out, n)
		}
	}
	return out
}

type nonTapCostTally struct {
	games, divs, paid, sacs, src, bigCtr int
}

func (c *nonTapCostCase) run(t *testing.T, seeds uint64, failOnDiv bool, maxTurns int32) nonTapCostTally {
	t.Helper()
	reg := synthReg(t)
	set := cardset.Raw(printedDelveSet(t, c.names()))
	decks, tbl := mccoyBoard(t, reg, set, c.deck)
	var tl nonTapCostTally
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"ntc-a", "ntc-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &nonTapCostScenario{t: t, tl: &tl, c: c}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: maxTurns, Check: sc.check})
		CloseSide(k)
		tl.games++
		if res.Div != nil {
			tl.divs++
			if failOnDiv {
				t.Fatalf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v (%v)\nker %v (%v)\nimplicates %v enablers %v",
					c.name, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
					res.Div.RefOptions, canonNames(m, res.Div.RefOptions), res.Div.KerOptions, canonNames(m, res.Div.KerOptions),
					res.Div.Implicates(m), res.Div.Enablers)
			}
			return tl
		}
	}
	return tl
}

type nonTapCostScenario struct {
	t    *testing.T
	tl   *nonTapCostTally
	c    *nonTapCostCase
	ref  *GorgeSide
	evAt int
}

func (p *nonTapCostScenario) Pick(c *PickCtx) Canon {
	return mulliganDonePick(p.priority, c)
}

func (p *nonTapCostScenario) priority(c *PickCtx) Canon {
	g := c.G.E.G
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
		return a
	}
	// the spells first: only the sources' mana pays them
	for _, name := range p.c.spells {
		if a, ok := reproHandCastAction(c, g, 0, name); ok {
			p.tl.paid++
			if controlsBattlefieldNamed(g, 0, p.c.source) {
				p.tl.src++
			}
			return a
		}
	}
	for _, name := range p.c.cast {
		if a, ok := reproHandCastAction(c, g, 0, name); ok {
			return a
		}
	}
	return pickKind(c, ActPass)
}

// check counts the sacrifices of the case's victims in gorge's event log.
func (p *nonTapCostScenario) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.CounterChange && e.Amount <= -2 && p.c.multiCtr && sideObjName(ref, e.Obj) == p.c.source {
			p.tl.bigCtr++
			continue
		}
		if e.Kind != events.MoveZone || !events.IsSacrifice(*e) {
			continue
		}
		if slices.Contains(p.c.victims, sideObjName(ref, e.Obj)) {
			p.tl.sacs++
		}
	}
	p.evAt = len(evs)
	return nil
}
