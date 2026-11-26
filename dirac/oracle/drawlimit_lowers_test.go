package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The DrawLimit$ lockstep: Narset, Parter of Veils ("each opponent can't draw
// more than one card each turn") and Spirit of the Labyrinth ("each player
// can't draw more than one card each turn") lower to StatCantDraw with the
// per-turn cap in Static.A (kernel static_cantdraw.go playerBanned, gorge
// drawForbidden's CardsDrawnThisTurn compare). Both decks run Divination and
// Night's Whisper, so a second card in a turn is often refused.
var kdrawlimDecks = [2][]cardset.Entry{
	{
		{Name: "Island", Count: 9}, {Name: "Plains", Count: 7},
		{Name: "Narset, Parter of Veils", Count: 4}, {Name: "Spirit of the Labyrinth", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 4},
	},
	{
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
		{Name: "Narset, Parter of Veils", Count: 4}, {Name: "Divination", Count: 4},
		{Name: "Night's Whisper", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Walking Corpse", Count: 4},
	},
}

// kdrawlimRun plays seeds with pol, failing on each divergence or unmodelled
// stall (unless mutated: the first divergence is returned instead). It
// returns how many games had a DrawLimit$ static on the battlefield while a
// player had already drawn a card that turn, on gorge's side.
func kdrawlimRun(t *testing.T, pol func(uint64) Policy, n uint64, mutated bool) (*Divergence, int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("drawlimit")
	if err != nil {
		t.Fatal(err)
	}
	kd := kdrawlimDecks
	var decks [2][]*cards.Card
	for s, d := range kd {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kd[0], kd[1])
	if err != nil {
		t.Fatal(err)
	}
	capped := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"dl-a", "dl-b"}, decks, 1)
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
		seen := false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if seen {
				return nil
			}
			e := ref.E
			limit := false
			for p := range e.G.Players {
				for _, id := range e.G.Zone(state.ZBattlefield, e.G.Players[p].ID) {
					if o := e.G.Obj(id); o != nil && o.Face() != nil &&
						(o.Face().Name == "Narset, Parter of Veils" || o.Face().Name == "Spirit of the Labyrinth") {
						limit = true
					}
				}
			}
			if !limit {
				return nil
			}
			for p := range e.G.Players {
				if e.CardsDrawnThisTurn(e.G.Players[p].ID) >= 1 {
					seen = true
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if seen {
			capped++
		}
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil && mutated {
			return d, capped
		} else if d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil, capped
}

// TestDrawLimitLowers: random and explore policies, no divergence and no
// unmodelled stall, and the cap is live on gorge's side (a DrawLimit$ static
// on the battlefield after a draw that turn) in some games.
func TestDrawLimitLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	kdrawlimRun(t, pkPolicy, 40, false)
	_, capped := kdrawlimRun(t, ExplorePolicy, 48, false)
	t.Logf("%d explore games had the draw cap live", capped)
	if capped == 0 {
		t.Error("the DrawLimit$ cap was never live on gorge's side")
	}
}

// TestDrawLimitMutation: with the cap ignored (dirac.DebugDrawLimitOff) the
// explore run diverges, so TestDrawLimitLowers depends on the kernel reading
// DrawLimit$ and does not pass vacuously.
func TestDrawLimitMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDrawLimitOff = true
	defer func() { dirac.DebugDrawLimitOff = false }()
	if d, _ := kdrawlimRun(t, ExplorePolicy, 48, true); d == nil {
		t.Error("no divergence without the DrawLimit$ cap")
	}
}
