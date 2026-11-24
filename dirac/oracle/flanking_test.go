package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// K:Flanking (CR 702.25a): whenever a flanking creature becomes blocked by a
// creature without flanking, the blocker gets -1/-1 until end of turn.
// Lowered from gorge's expansion trigger (AttackerBlockedByCreature,
// ValidBlocker$ Creature.withoutFlanking, kop.W7WithoutFlanking), played
// decision by decision against gorge over the flanking pool: the two decks
// field flanking attackers and blockers beside plain creatures, so the word
// has both answers (a flanking blocker takes no penalty, CR 702.25a).

var flankDecks = [2][]cardset.Entry{
	{
		{Name: "Benalish Cavalry", Count: 4}, {Name: "Zhalfirin Knight", Count: 4}, {Name: "Femeref Knight", Count: 4},
		{Name: "Elite Vanguard", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Fallen Askari", Count: 4}, {Name: "Shadow Rider", Count: 4}, {Name: "Bogardan Lancer", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Scathe Zombies", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

var flankCarriers = map[string]bool{
	"Benalish Cavalry": true, "Zhalfirin Knight": true, "Femeref Knight": true,
	"Fallen Askari": true, "Shadow Rider": true, "Bogardan Lancer": true,
}

// flankScan counts the flanking triggers that fired (the carriers' only
// trigger is the expansion's) and the creatures that cast.
func flankScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && flankCarriers[ob.Face().Name] {
				n["flanking"]++
			}
		case events.DeclareBlockers:
			n["blocks"]++
		}
	}
}

func runFlanking(t *testing.T, seeds uint64) (games int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("flanking")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range flankDecks[s] {
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
	tbl, _, _, err := set.Matchup(flankDecks[0], flankDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fl-a", "fl-b"}, decks, 1)
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
		flankScan(gs, branch)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestFlankingLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runFlanking(t, 200)
	t.Logf("%d games; %v", games, branch)
	for _, want := range []string{"flanking", "blocks"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
}
