package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated
// exilefd pool, whose carriers are Digs into exile with ExileFaceDown$ True
// and WithMayLook$ True (Outrageous Robbery, Decadent Dragon's Expensive Taste,
// Black Cat, Cunning Thief). gorge stamps the move exiled_with_face_down_maylook
// (effects/zone_change.go:1286-1335), the fold sets Object.FaceDown
// (events/apply_zone.go:222-240), and the lockstep projection carries FaceDown
// on an exiled object (FFaceDown). The remembered cards are then cast or played
// through the Effect's MayPlay.

var kefdDecks = [2][]cardset.Entry{
	{
		{Name: "Outrageous Robbery", Count: 2}, {Name: "Decadent Dragon", Count: 3}, {Name: "Black Cat, Cunning Thief", Count: 3},
		{Name: "Walking Corpse", Count: 5}, {Name: "Goblin Piker", Count: 4}, {Name: "Disfigure", Count: 2},
		{Name: "Swamp", Count: 11}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Outrageous Robbery", Count: 1}, {Name: "Decadent Dragon", Count: 3}, {Name: "Black Cat, Cunning Thief", Count: 3},
		{Name: "Walking Corpse", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 9},
	},
}

// kefdScan counts, in gorge's event log, the face-down may-look exiles and the
// casts / land plays of a card that was exiled that way.
func kefdScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	down := map[state.ObjID]bool{}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.MoveZone {
			continue
		}
		if ev.To == state.ZExile && ev.Counter == "exiled_with_face_down_maylook" {
			down[ev.Obj] = true
			n["exiled"]++
			continue
		}
		if ev.From == state.ZExile {
			if down[ev.Obj] && (ev.To == state.ZStack || ev.To == state.ZBattlefield) {
				n["played"]++
			}
			delete(down, ev.Obj)
		}
	}
}

// runExilefd plays the seeded games; mutate sets DebugExileFaceUp and stops
// at the first divergence.
func runExilefd(t *testing.T, seeds uint64, mutate bool) (games, divs int, branch map[string]int, stalls map[string]int, by map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exilefd")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kefdDecks {
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
	tbl, _, _, err := set.Matchup(kefdDecks[0], kefdDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &dirac.ExileFDStats{}
	dirac.DebugExileFDStats = st
	dirac.DebugExileFaceUp = mutate
	defer func() { dirac.DebugExileFDStats, dirac.DebugExileFaceUp = nil, false }()
	played := map[string]int{}
	branch, stalls = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kefd-a", "kefd-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		kefdScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	by = map[string]int{}
	for ci := range tbl.Cards {
		if ci < len(st.By) && st.By[ci] > 0 {
			// a double-faced card's table name is "face:<name>#<n>" (Decadent Dragon's Adventure)
			nm := strings.TrimPrefix(tbl.Cards[ci].Name, "face:")
			if i := strings.Index(nm, "#"); i >= 0 {
				nm = nm[:i]
			}
			by[nm] += st.By[ci]
		}
	}
	return
}

func TestExilefdLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, branch, stalls, by := runExilefd(t, 256, false)
	t.Logf("%d games, %d divergences; gorge exiled face down %d, played from there %d; kernel exiled %v; kernel stalls %v",
		games, divs, branch["exiled"], branch["played"], by, stalls)
	if len(stalls) != 0 {
		t.Errorf("kernel stalls %v", stalls)
	}
	if branch["exiled"] < 20 {
		t.Errorf("face-down exiles %d, want at least 20", branch["exiled"])
	}
	for _, c := range []string{"Outrageous Robbery", "Decadent Dragon", "Black Cat, Cunning Thief"} {
		if by[c] < 5 {
			t.Errorf("%s face-down exiles %d, want at least 5", c, by[c])
		}
	}
	if branch["played"] < 5 {
		t.Errorf("casts or plays of a face-down exiled card %d, want at least 5", branch["played"])
	}
}

// TestExilefdMutation leaves the Dig's cards face up in the kernel
// (DebugExileFaceUp): the projection loses FFaceDown on every such card, so
// the lockstep must diverge.
func TestExilefdMutation(t *testing.T) {
	games, divs, _, _, _ := runExilefd(t, 128, true)
	if divs == 0 {
		t.Fatalf("DebugExileFaceUp mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
