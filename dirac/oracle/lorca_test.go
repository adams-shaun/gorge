package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveyard FacesDilemma lockstep: Gabriel Lorca (TriggerZones$ Graveyard, an
// optional {B} ChangeZone back to hand) in both decks. Charming Scoundrel's
// discard and the self-mill put him in the graveyard; the charms are the
// dilemmas (the Caretaker and Seven of Nine add the opponent's). The graveyard
// trigger must fire once per dilemma of its owner, never for the other player's.
var klorcaDecks = [2][]cardset.Entry{
	{
		{Name: "Gabriel Lorca, Mirror Menace", Count: 6}, {Name: "Seven of Nine", Count: 2}, {Name: "Abrade", Count: 4},
		{Name: "Charming Scoundrel", Count: 6}, {Name: "Thought Scour", Count: 3}, {Name: "Tome Scour", Count: 3},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "The Caretaker", Count: 6}, {Name: "Gabriel Lorca, Mirror Menace", Count: 2}, {Name: "Seven of Nine", Count: 2},
		{Name: "Abrade", Count: 4}, {Name: "Charming Scoundrel", Count: 6},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4},
	},
}

const klorcaName = "Gabriel Lorca, Mirror Menace"

// klorcaObs is what the games reached, read from the gorge side.
type klorcaObs struct {
	trig     int // graveyard triggers seen on the stack (distinct stack objects)
	returned int // Lorca moves graveyard -> hand
	oppFaced int // dilemmas a player faced while only the OTHER player's graveyard held Lorca
	twice    int // windows with Lorca in the graveyard and two dilemmas faced this turn by its owner
}

func klorcaPolicy(seed uint64, obs *klorcaObs) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	prev := map[state.ObjID]state.Zone{}
	var faced [2]int32
	var turn int32
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			gs := g.E.G
			if gs.Turn != turn {
				turn, faced = gs.Turn, [2]int32{}
			}
			var inGrave [2]bool
			for p := range gs.Players {
				for _, id := range gs.Zone(state.ZGraveyard, state.PlayerID(p)) {
					if o := gs.Obj(id); o != nil && o.Face() != nil && o.Face().Name == klorcaName {
						inGrave[p] = true
					}
				}
			}
			for p := 0; p < 2; p++ {
				if d := gs.Players[p].DilemmasFacedThisTurn; d > faced[p] {
					if inGrave[1-p] && !inGrave[p] {
						obs.oppFaced += int(d - faced[p])
					}
					faced[p] = d
					if inGrave[p] && d >= 2 {
						obs.twice++
					}
				}
			}
			for _, id := range gs.Stack {
				if o := gs.Obj(id); o != nil && o.Source != 0 && !seen[id] {
					if s := gs.Obj(o.Source); s != nil && s.Face() != nil && s.Face().Name == klorcaName && s.Zone == state.ZGraveyard {
						seen[id] = true
						obs.trig++
					}
				}
			}
			for p := range gs.Players {
				for _, z := range []state.Zone{state.ZGraveyard, state.ZHand} {
					for _, id := range gs.Zone(z, state.PlayerID(p)) {
						if o := gs.Obj(id); o != nil && o.Face() != nil && o.Face().Name == klorcaName {
							if prev[id] == state.ZGraveyard && z == state.ZHand {
								obs.returned++
							}
							prev[id] = z
						}
					}
				}
			}
		}
		return rnd.Pick(c)
	})
}

func runKlorca(t *testing.T, seeds int, mutate bool) (games, divs int, obs *klorcaObs) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dilemma")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugDilemmaNoGrave
	dirac.DebugDilemmaNoGrave = mutate
	defer func() { dirac.DebugDilemmaNoGrave = old }()
	obs = &klorcaObs{}
	var decks [2][]*cards.Card
	for s, d := range klorcaDecks {
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
	tbl, _, _, err := set.Matchup(klorcaDecks[0], klorcaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"klorca-a", "klorca-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, klorcaPolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if !mutate && res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
	}
	return games, divs, obs
}

func TestLorcaLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, obs := runKlorca(t, 256, false)
	t.Logf("%d games, %d divergences, %d graveyard triggers, %d returned to hand, %d opponent dilemmas with only the other graveyard holding Lorca, %d second dilemmas with Lorca in the graveyard",
		games, divs, obs.trig, obs.returned, obs.oppFaced, obs.twice)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if obs.trig < 200 {
		t.Errorf("%d graveyard triggers, want >= 200", obs.trig)
	}
	if obs.returned < 50 {
		t.Errorf("%d returns to hand, want >= 50", obs.returned)
	}
	if obs.oppFaced < 50 {
		t.Errorf("%d opponent dilemmas, want >= 50", obs.oppFaced)
	}
	if obs.twice < 20 {
		t.Errorf("%d turns with two dilemmas, want >= 20", obs.twice)
	}
}

// TestLorcaMutation makes the kernel's dilemma walk skip the graveyard: gorge
// queues the trigger the kernel never does, so the games must diverge.
func TestLorcaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runKlorca(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation skipped the graveyard walk in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
