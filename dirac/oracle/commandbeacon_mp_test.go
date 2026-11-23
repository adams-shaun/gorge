package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestFourSeatCommandBeaconLockstep plays four-seat Commander pods of Command
// Beacon decks on the edh table, dirac against gorge. Beacon's second ability
// ("{T}, Sacrifice: put your commander into your hand from the command zone")
// is a stack object (CR 602.2, 405.1) whose resolution asks its controller to
// pick the commander (effHiddenPick over the command zone) and then moves it
// to hand under the CR 903.9b ask (command zone or hand). kfuzz -kernel edh
// -seats 4 -cmd-mini 100 -seed 1 pair 22 diverged at decision 91, projection
// NStack: the kernel lowered the pick to nothing (no seat has a commander),
// kept no stack item and posed no ask where gorge kept one and posed both.
// No pod diverges, and across them a Beacon ability resolved with a commander
// in the command zone.
func TestFourSeatCommandBeaconLockstep(t *testing.T) {
	set := edhSet(t)
	base, _ := testCorpus(t)
	c := &Corpus{Reg: base.Reg, decks: map[string][]*cards.Card{}}
	cmdrs := []string{"Brimaz, Blight of Oreskos", "Jean-Luc Picard, Luminary", "Brimaz, Blight of Oreskos", "Jean-Luc Picard, Luminary"}
	names := make([]string, len(cmdrs))
	files := map[string]deck.File{}
	for i, cmd := range cmdrs {
		names[i] = "beacon-" + string(rune('a'+i))
		files[names[i]] = deck.File{
			Name: names[i], Format: "commander", Commander: cmd,
			Cards: []deck.Entry{{Name: cmd, Count: 1}, {Name: "Command Beacon", Count: 30},
				{Name: "Plains", Count: 25}, {Name: "Swamp", Count: 25}, {Name: "Island", Count: 19}},
		}
	}
	c.AddDecks(files)
	resolved := 0
	for seed := uint64(1); seed <= 6; seed++ {
		m, tbl, err := SetMatchupN(set, c, names, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 100}
		rc.Finish = func(g *GorgeSide) {
			for i := range g.E.L.Events {
				e := &g.E.L.Events[i]
				if e.Kind == events.AbilityPush {
					if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Command Beacon" {
						resolved++
					}
				}
			}
		}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, AffordableTargetPolicy(RandomPolicy(seed)), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		t.Logf("seed %d: %d decisions, stall %q", seed, res.Decisions, res.Stall)
	}
	t.Logf("%d Command Beacon activations", resolved)
	if resolved < 3 {
		t.Fatalf("%d Command Beacon activations: the pick went unexercised", resolved)
	}
}
