package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The blind face-down exile (CR 406.3), played decision by decision against
// gorge over the exilefacedown pool: Bomat Courier's and Connecting the Dots'
// attack-trigger Dig, Induced Amnesia's remembering hand exile and the return
// when it dies, Triumph of Saint Katherine's own exile and its six. The
// projection compares FFaceDown on every exiled object.

var exileFDDecks = [2][]cardset.Entry{
	{
		{Name: "Bomat Courier", Count: 4}, {Name: "Connecting the Dots", Count: 3}, {Name: "Triumph of Saint Katherine", Count: 3},
		{Name: "Raging Goblin", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Disenchant", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Induced Amnesia", Count: 4}, {Name: "Doom Blade", Count: 4}, {Name: "Scathe Zombies", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Unsummon", Count: 4}, {Name: "Bomat Courier", Count: 3},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 8},
	},
}

// exileFDScan counts the cards gorge exiled face down (the blind marker) and
// those that came back out of exile.
func exileFDScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.MoveZone {
			continue
		}
		switch {
		case ev.To == state.ZExile && ev.Counter == "exiled_with_face_down":
			n["faceDown"]++
			if ev.From == state.ZHand {
				n["fromHand"]++
			}
		case ev.From == state.ZExile:
			n["leftExile"]++
		}
	}
}

func TestExileFaceDownLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "exilefacedown", exileFDDecks, 200, exileFDScan)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"faceDown", "fromHand", "leftExile"} {
		if branch[want] < 20 {
			t.Errorf("%s %d, want at least 20", want, branch[want])
		}
	}
}

// exileFDDiverged plays the pool's games without reporting and returns how
// many diverged (the mutation test's measure).
func exileFDDiverged(t *testing.T, seeds uint64) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exilefacedown")
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card
	for s, d := range exileFDDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(exileFDDecks[0], exileFDDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"efd-a", "efd-b"}, cs, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			div++
		}
	}
	return div
}

// TestExileFaceDownMutation: with the face-down mark skipped, or the
// departed source's link dropped, some seed diverges.
func TestExileFaceDownMutation(t *testing.T) {
	// not parallel: it flips dirac.DebugExileFaceUp and dirac.DebugExwNoLeft
	dirac.DebugExileFaceUp = true
	n := exileFDDiverged(t, 60)
	dirac.DebugExileFaceUp = false
	if n == 0 {
		t.Error("no seed diverged with the face-down mark skipped")
	}
	dirac.DebugExwNoLeft = true
	n = exileFDDiverged(t, 60)
	dirac.DebugExwNoLeft = false
	if n == 0 {
		t.Error("no seed diverged with the departed source's link dropped")
	}
}
