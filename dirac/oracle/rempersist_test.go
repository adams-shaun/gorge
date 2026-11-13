package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The remembered-persist lockstep: Journey to Nowhere and
// Mesmeric Fiend, whose leaves trigger reads what their enters trigger
// remembered (CardDef.RemPersist, dirac/zone_rememberpersist.go). Shock
// kills the Fiend and Disenchant destroys the Journey, sometimes in response
// to the enters trigger. Counters from gorge's event log:
//
//  1. Journey exiles a creature, 2. its leaves trigger returns it,
//  3. the Fiend exiles a hand card, 4. its leaves trigger returns it,
//  5. a source left the battlefield with its enters trigger still on the
//     stack (the card it then takes stays exiled),
//  6. Hoarding Dragon (the dies form) dies and its leaves
//     trigger puts the searched artifact into its owner's hand.
//
// Journey and the Fiend play in separate matchups: gorge's Defined$
// Remembered falls back to the trigger's event capture (the leaving source
// itself) when the source remembered nothing (its contract "Trigger
// remembered readers intentionally differ"), so a Fiend whose enters
// trigger took nothing, exiled by a Journey, returns ITSELF to its owner's
// hand from its leaves trigger. Forge reads only the host's remembered list
// and the rules return only the card the Fiend exiled, which the kernel
// follows (zone_rememberpersist.go); the interaction is a gorge defect,
// so no matchup here has a Journey beside a
// Fiend.
var rpDecks = [][2][]cardset.Entry{
	{ // the Fiend, Shock to kill it (sometimes in response to its trigger)
		{
			{Name: "Mesmeric Fiend", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Walking Corpse", Count: 6},
			{Name: "Swamp", Count: 10}, {Name: "Plains", Count: 10},
		},
		{
			{Name: "Shock", Count: 8}, {Name: "Disenchant", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Savannah Lions", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
		},
	},
	{ // Hoarding Dragon, Murder to kill it (sometimes in response to its trigger)
		{
			{Name: "Hoarding Dragon", Count: 8}, {Name: "Mind Stone", Count: 4}, {Name: "Sol Ring", Count: 4},
			{Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 16},
		},
		{
			{Name: "Murder", Count: 10}, {Name: "Walking Corpse", Count: 14}, {Name: "Swamp", Count: 16},
		},
	},
	{ // Journey, Disenchant to destroy it (sometimes in response to its trigger)
		{
			{Name: "Journey to Nowhere", Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Walking Corpse", Count: 6},
			{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Disenchant", Count: 8}, {Name: "Shock", Count: 2}, {Name: "Hill Giant", Count: 6}, {Name: "Savannah Lions", Count: 4},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
		},
	},
}

type rpTally struct {
	evAt                             int
	jExile, jReturn, fExile, fReturn int
	early                            int
	dragonReturn                     int                  // an artifact from exile to hand: Hoarding Dragon's dies trigger
	etbOn                            map[state.ObjID]bool // a source whose enters trigger is on the stack
}

func (p *rpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.TriggerPush {
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && int(e.Amount) < len(o.Face().Triggers) && (o.Face().Name == "Journey to Nowhere" || o.Face().Name == "Mesmeric Fiend") {
				p.etbOn[e.Obj] = e.Amount == 0
			}
			continue
		}
		if e.Kind == events.Choose && p.etbOn[e.Obj] {
			p.etbOn[e.Obj] = false // the enters trigger's remember: it resolved
		}
		if e.Kind != events.MoveZone {
			continue
		}
		switch {
		case e.From == state.ZBattlefield && e.To == state.ZExile:
			p.jExile++ // the pool's only battlefield exile is Journey's
		case e.From == state.ZExile && e.To == state.ZBattlefield:
			p.jReturn++
		case e.From == state.ZHand && e.To == state.ZExile:
			p.fExile++
		case e.From == state.ZExile && e.To == state.ZHand:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().IsArtifact() {
				p.dragonReturn++
			} else {
				p.fReturn++
			}
		}
		if e.From == state.ZBattlefield {
			if n := chName(g, e.Obj); n == "Journey to Nowhere" || n == "Mesmeric Fiend" {
				if p.etbOn[e.Obj] {
					p.early++
				}
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// rpRun plays the matchups for seeds seeds each. With mutated set a
// divergence is returned instead of failing the test.
func rpRun(t *testing.T, seeds int, mutated bool) (p *rpTally, games int, diverged bool) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("rempersist")
	if err != nil {
		t.Fatal(err)
	}
	p = &rpTally{etbOn: map[state.ObjID]bool{}}
	var matchups [][2][]cardset.Entry
	for _, d := range rpDecks {
		matchups = append(matchups, d, [2][]cardset.Entry{d[1], d[0]})
	}
	for mi, md := range matchups {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"rp-a", "rp-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed) // plays at random among its plays
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt = 0
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if mutated {
				if res.Div != nil {
					return p, games, true
				}
				continue
			}
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return p, games, diverged
}

func TestRempersistLockstep(t *testing.T) {
	t.Parallel()
	p, games, _ := rpRun(t, 60, false)
	t.Logf("%d games: Journey exiled %d, returned %d; Fiend exiled %d, returned %d; sources gone with their trigger on the stack %d; Hoarding Dragon returns %d",
		games, p.jExile, p.jReturn, p.fExile, p.fReturn, p.early, p.dragonReturn)
	for _, c := range []struct {
		name string
		n    int
	}{{"Journey exiles", p.jExile}, {"Journey returns", p.jReturn}, {"Fiend exiles", p.fExile}, {"Fiend returns", p.fReturn}, {"early leaves", p.early}, {"Hoarding Dragon dies-trigger returns", p.dragonReturn}} {
		want := 5
		if c.name == "Hoarding Dragon dies-trigger returns" {
			want = 3
		}
		if c.n < want {
			t.Errorf("%d %s, want at least %d", c.n, c.name, want)
		}
	}
}

// TestRempersistMutation: with the kept remembered set dropped, the dragon's
// dies trigger (and every leaves trigger) finds nothing to return, which
// gorge's source-held set still returns.
func TestRempersistMutation(t *testing.T) {
	dirac.DebugRemPersistNoKeep = true
	defer func() { dirac.DebugRemPersistNoKeep = false }()
	if _, _, diverged := rpRun(t, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with the remembered set not kept")
	}
}
