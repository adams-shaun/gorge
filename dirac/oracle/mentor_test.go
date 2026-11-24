package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// K:Mentor (CR 702.134): whenever the creature attacks, put a +1/+1 counter
// on target attacking creature with lesser power. Lowered from gorge's
// expansion (an Attacks self-trigger, a targeted PutCounter carrying Mentor$,
// kgen mentorPutCounter: the target filter reads power < the source's power),
// played decision by decision against gorge over the mentor pool. The decks
// field Mentor carriers of every power beside plain creatures, so the target
// offer has empty, one-card and many-card answers. No other card in the pool
// puts a +1/+1 counter, so each CounterChange is a Mentor resolution.

var mentorMatchups = [][2][]cardset.Entry{
	{
		{
			{Name: "Goblin Banneret", Count: 3}, {Name: "Boros Challenger", Count: 3}, {Name: "Sunhome Stalwart", Count: 3},
			{Name: "Blade Instructor", Count: 3}, {Name: "Tajic, Legion's Edge", Count: 3}, {Name: "Hammer Dropper", Count: 3},
			{Name: "Barging Sergeant", Count: 3}, {Name: "Truefire Captain", Count: 3},
			{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
		},
		{
			{Name: "Aurelia, Exemplar of Justice", Count: 4}, {Name: "Wojek Bodyguard", Count: 4}, {Name: "Light of the Legion", Count: 4},
			{Name: "Parhelion Patrol", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
		},
	},
	{
		{
			{Name: "Felisa, Fang of Silverquill", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Scathe Zombies", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
		},
		{
			{Name: "Tributary Instructor", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Elite Vanguard", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
		},
	},
}

var mentorCarrierNames = map[string]bool{
	"Goblin Banneret": true, "Boros Challenger": true, "Sunhome Stalwart": true, "Blade Instructor": true,
	"Tajic, Legion's Edge": true, "Hammer Dropper": true, "Barging Sergeant": true, "Truefire Captain": true,
	"Aurelia, Exemplar of Justice": true, "Wojek Bodyguard": true, "Light of the Legion": true,
	"Parhelion Patrol": true, "Felisa, Fang of Silverquill": true, "Tributary Instructor": true,
}

// mentorScan counts the Mentor triggers that went on the stack and the
// +1/+1 counters they put.
func mentorScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && mentorCarrierNames[ob.Face().Name] {
				n["triggers"]++
			}
		case events.CounterChange:
			n["counters"]++
		}
	}
}

func TestMentorLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mentor")
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	games := 0
	for mi, mu := range mentorMatchups {
		var decks [2][]*cards.Card
		for s := range decks {
			for _, e := range mu[s] {
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
		tbl, _, _, err := set.Matchup(mu[0], mu[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 200; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mentor-a", "mentor-b"}, decks, 1)
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
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
			CloseSide(k)
			games++
			mentorScan(gs, branch)
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Fatalf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games; %v; plays %v", games, branch, played)
	for _, want := range []string{"triggers", "counters"} {
		if branch[want] < 100 {
			t.Errorf("%s %d, want at least 100", want, branch[want])
		}
	}
	for n := range mentorCarrierNames {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}
