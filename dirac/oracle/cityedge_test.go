package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The City on the Edge of Forever lockstep: chapter I phases out every
// artifact and creature (CR 702.26a, one at a time, an Aura or Equipment
// attached to one going with it, 702.26g), then creates a 0/1 Human whose
// static says permanents can't phase in; the phased-out permanents stay out
// through their controllers' untap steps until the token is gone (Shock
// kills it in the second matchup), and chapter III sacrifices a creature
// token for 5 life. Savannah Lions, Grizzly Bears, Bonesplitter and
// Pacifism sit on both sides of the table. The expected answer at every
// decision is gorge's.
var kcityedgeDecks = [][2][]cardset.Entry{{
	{
		{Name: "The City on the Edge of Forever", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Bonesplitter", Count: 4},
		{Name: "Pacifism", Count: 2}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "The City on the Edge of Forever", Count: 4}, {Name: "Savannah Lions", Count: 8}, {Name: "Bonesplitter", Count: 4},
		{Name: "Pacifism", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
}, {
	{
		{Name: "The City on the Edge of Forever", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Bonesplitter", Count: 4},
		{Name: "Pacifism", Count: 2}, {Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 10}, {Name: "Bonesplitter", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}}

// cityedgePolicy plays a land, else casts or activates whenever it can, else
// plays at random, so the games reach the Saga and the Equipment's equip.
func cityedgePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kcityedgeTally counts, on gorge's side, what the games reach.
type kcityedgeTally struct {
	games, direct, indirect, in, held, sacGames int
}

// kcityedgeCheck tallies phase changes, and the phased-out permanents that
// are still out at the first decision of their controller's turn (the untap
// step's phase-in did not happen, the token holds it).
func kcityedgeCheck(tal *kcityedgeTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	type st struct{ out, ind bool }
	prev := map[state.ObjID]st{}
	turn := int32(-1)
	return func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		if ref != cur {
			cur, prev, turn = ref, map[state.ObjID]st{}, -1
			tal.games++
		}
		newTurn := int32(g.Turn) != turn
		turn = int32(g.Turn)
		for i := range g.Objs {
			o := &g.Objs[i]
			now := st{o.PhasedOut && o.Zone == state.ZBattlefield, o.PhasedOutIndirect}
			was := prev[o.ID]
			switch {
			case now.out && !was.out && now.ind:
				tal.indirect++
			case now.out && !was.out:
				tal.direct++
			case !now.out && was.out:
				tal.in++
			}
			if newTurn && now.out && was.out && !now.ind && o.Controller == g.Active {
				tal.held++
			}
			prev[o.ID] = now
		}
		return nil
	}
}

// runCityedge plays the matchups for seeds 1..seeds and returns the games and
// divergences; mutate stops the Saga's phase-out.
func runCityedge(t *testing.T, seeds int, mutate bool, tal *kcityedgeTally) (divs int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cityedge")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPhasesNone
	dirac.DebugPhasesNone = mutate
	defer func() { dirac.DebugPhasesNone = old }()
	for mi, md := range kcityedgeDecks {
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
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcityedge-a", "kcityedge-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, cityedgePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: kcityedgeCheck(tal)})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[max(d.At-12, 0):d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return divs
}

func TestCityOnTheEdgeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kcityedgeTally
	divs := runCityedge(t, 200, false, &tal)
	t.Logf("%d games, %d divergences: %d direct phase-outs, %d indirect, %d phase-ins, %d held out through their controller's untap", tal.games, divs, tal.direct, tal.indirect, tal.in, tal.held)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tal.direct < 100 || tal.indirect < 10 || tal.held < 20 || tal.in < 10 {
		t.Errorf("direct %d (want >= 100), indirect %d (>= 10), held out %d (>= 20), phase-ins %d (>= 10)", tal.direct, tal.indirect, tal.held, tal.in)
	}
}

// TestCityOnTheEdgeMutation stops the phase-out: the games must diverge. A
// green mutation (no divergence) means the lockstep never reaches chapter I.
func TestCityOnTheEdgeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kcityedgeTally
	divs := runCityedge(t, 40, true, &tal)
	if divs == 0 {
		t.Fatalf("mutation phased nothing out in %d games with no divergence", tal.games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, tal.games)
}
