package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC api-params-2 spellings, each played in lockstep over its fixture
// pool:
//
//   - faces (flip cards, CR 710): Orochi Eggwatcher and Nezumi Graverobber
//     flip by SetState Mode$ Flip (kop.FaceFlip); the Graverobber's flip
//     reads Count$ValidGraveyard Card.OwnedBy TargetedController
//     (kop.CtrlTgtOwner). Fixture dirac/cardset/pools/fixtures/mbcflip.
//   - param:trigger.AtLeastOneInstance: Matoc, Lavamancer's DamageDealtOnce
//     trigger fires only when a recipient was dealt exactly 2 of a source's
//     damage (Shock, Grizzly Bears and its own "any other target" ping pass;
//     Lightning Bolt and Hill Giant do not). Fixture mbcmatoc.
//
// Each subtest asserts the route it is about ran in gorge (the flip, the
// trigger), so a lockstep that never reached it cannot pass.

// ap2Case is one spelling's lockstep: its fixture, decks and the gorge-side
// tally that proves the route ran.
type ap2Case struct {
	fixture string
	decks   []fixtureDecks
	watch   map[string]bool // cards the policy casts and activates first
	seeds   int
	// tally reads one finished game's gorge events into counts by name.
	tally func(g *GorgeSide, evs []events.Event, counts map[string]int)
	// want is the least count of each tallied name over every game.
	want map[string]int
}

func TestMBCApiParams2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, tc := range []struct {
		name string
		c    ap2Case
	}{
		{"faces", ap2Flip},
		{"AtLeastOneInstance", ap2Matoc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counts := runAP2(t, tc.c)
			t.Logf("tally %v", counts)
			for k, n := range tc.c.want {
				if counts[k] < n {
					t.Errorf("%s: %d, want at least %d (the route did not run)", k, counts[k], n)
				}
			}
		})
	}
}

var ap2Flip = ap2Case{
	fixture: "mbcflip",
	decks: []fixtureDecks{{
		{
			{Name: "Orochi Eggwatcher", Count: 4}, {Name: "Nezumi Graverobber", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 3},
		},
		{
			{Name: "Orochi Eggwatcher", Count: 4}, {Name: "Nezumi Graverobber", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4},
			{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 3},
		},
	}},
	watch: map[string]bool{"Orochi Eggwatcher": true, "Nezumi Graverobber": true, "Llanowar Elves": true, "Grizzly Bears": true},
	seeds: 60,
	tally: func(g *GorgeSide, evs []events.Event, counts map[string]int) {
		for i := range evs {
			e := &evs[i]
			if e.Kind != events.FlipFace || e.Text == "Transformed" || e.Amount != 1 {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Card != nil && o.Card.AlternateMode == "Flip" {
				counts["flip "+o.Card.Faces[0].Name]++
			}
		}
	},
	want: map[string]int{"flip Orochi Eggwatcher": 10, "flip Nezumi Graverobber": 40},
}

var ap2Matoc = ap2Case{
	fixture: "mbcmatoc",
	decks: []fixtureDecks{{
		{
			{Name: "Matoc, Lavamancer", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Mountain", Count: 14}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Matoc, Lavamancer", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Mountain", Count: 14}, {Name: "Forest", Count: 6},
		},
	}},
	watch: map[string]bool{"Matoc, Lavamancer": true, "Shock": true, "Lightning Bolt": true},
	seeds: 60,
	tally: func(g *GorgeSide, evs []events.Event, counts map[string]int) {
		matoc := func(id state.ObjID) bool {
			o := g.E.G.Obj(id)
			return o != nil && o.Card != nil && o.Card.Faces[0].Name == "Matoc, Lavamancer"
		}
		onField := false // a Matoc on the battlefield as the damage is dealt (approximated by any)
		for i := range evs {
			e := &evs[i]
			switch {
			case e.Kind == events.TriggerPush && matoc(e.Obj):
				counts["trigger"]++
			case e.Kind == events.AbilityPush && matoc(e.Obj):
				counts["ping"]++
			case e.Kind == events.MoveZone && e.To == state.ZBattlefield && matoc(e.Obj):
				onField = true
			case e.Kind == events.Damage && e.Amount > 0 && e.Amount != 2 && onField:
				counts["not-2 damage"]++
			}
		}
	},
	want: map[string]int{"trigger": 20, "ping": 5, "not-2 damage": 20},
}

// ap2Policy plays randomly but, three times in four, takes a cast or an
// activation of a watched card the moment one is offered with an empty stack.
func ap2Policy(seed uint64, watch map[string]bool) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x61703270)) // "ap2p"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority && len(c.G.E.G.Stack) == 0 && prng.IntN(4) != 0 {
			for _, a := range c.D.Options {
				if (a.Kind() == ActCast || a.Kind() == ActActivate) && watch[refCardName(c.G, a.Src())] {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runAP2 plays seeds 1..c.seeds of c's matchups in lockstep, failing on a
// divergence or a kernel stall, and returns c.tally summed over the games.
func runAP2(t *testing.T, c ap2Case) map[string]int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(c.fixture)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	divs := 0
	for seed := uint64(1); seed <= uint64(c.seeds) && divs < 3; seed++ {
		md := c.decks[int(seed/2)%len(c.decks)]
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				cd, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], cd)
				}
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ap2-a", "ap2-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, ap2Policy(seed, c.watch), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %s %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Detail, d.Diff, d.RefOptions, d.KerOptions)
		}
		c.tally(g, g.E.L.Events, counts)
	}
	return counts
}
