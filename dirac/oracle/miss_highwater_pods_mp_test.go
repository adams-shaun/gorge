package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Miss Highwater in a four-seat pod: a player who took the contract (the
// damaged player discarded their hand, drew seven and got a contract counter)
// and later loses the game while the game goes on is the case the card was
// marked two-seat only for: in two seats the loss ends the game, so the
// Effect's LosesGame trigger (copies of the loser's artifacts and creatures)
// never resolves. Gorge arms the comma-listed Effect trigger
// ("LoseGameTrigger,ExileEffect") and, after the loss, copies the loser's
// artifacts and creatures; the kernel carries the same Effect through the
// armed contract-counter put (dirac/kgen contractLossEffect,
// dirac/ctr_losscopy.go). The pod lockstep pins that both engines make the
// copies and agree decision for decision.
//
// The class is read off play only (a contracted seat loses while the game goes
// on), never off how gorge spells a registration or a diagnostic Note: those
// change with no change in play, and the test must not care.
//
// The pod: seat 0 plays Miss Highwater; seats 1-3 play short decks of
// creatures and a Manalith, so a seat that takes the contract (it draws seven)
// runs out of library soon after, and the game goes on past the loss.

var missHighwaterPodDecks = [4][]cardset.Entry{
	{{Name: "Miss Highwater", Count: 10}, {Name: "Dark Ritual", Count: 4}, {Name: "Swamp", Count: 16}},
	{{Name: "Savannah Lions", Count: 4}, {Name: "Manalith", Count: 2}, {Name: "Plains", Count: 8}},
	{{Name: "Grizzly Bears", Count: 4}, {Name: "Manalith", Count: 2}, {Name: "Forest", Count: 8}},
	{{Name: "Raging Goblin", Count: 4}, {Name: "Gray Ogre", Count: 3}, {Name: "Mountain", Count: 7}},
}

// missHighwaterPodTally counts what the games showed, read off gorge's logs.
type missHighwaterPodTally struct {
	games, contracts, contractLosses, copies, goneOn int
}

// missHighwaterPodTallyGame adds one finished game's events to y.
func missHighwaterPodTallyGame(g *GorgeSide, y *missHighwaterPodTally) {
	var contract [MaxSeats]int
	lossSeen := false
	lost := 0 // seats out so far, this loss included
	for i := range g.E.L.Events {
		switch e := &g.E.L.Events[i]; e.Kind {
		case events.PlayerCounterChange:
			if e.Counter == "Contract" && int(e.Player) < MaxSeats {
				contract[e.Player] += int(e.Amount)
				y.contracts++
			}
		case events.PlayerLost:
			lost++
			if int(e.Player) < MaxSeats && contract[e.Player] > 0 {
				y.contractLosses++
				lossSeen = true
				if lost < MaxSeats-1 {
					y.goneOn++ // two seats or more still stand after it: the game goes on
				}
			}
		case events.CopyToken, events.TokenCreate:
			if lossSeen {
				y.copies++
			}
		}
	}
	y.games++
}

// TestFourSeatMissHighwaterPodsLockstep plays the pod over seeds in lockstep:
// no game diverges or stalls, and across them a seat that took the contract
// leaves the game while it goes on and tokens appear after it.
func TestFourSeatMissHighwaterPodsLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 4)
	gd := make([][]*cards.Card, 4)
	md := make([][]cardset.Entry, 4)
	var all []string
	for s, d := range missHighwaterPodDecks {
		names[s] = "highwater-pod"
		md[s] = d
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			all = append(all, e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, [2][]string{all, nil}))
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	var y missHighwaterPodTally
	for seed := uint64(1); seed <= 40; seed++ {
		cfg := Config{}
		m, err := NewMatchupN(reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		missHighwaterPodTallyGame(g, &y)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
			break
		}
	}
	t.Logf("%+v", y)
	// the class: seats that took the contract (the counter and the Effect
	// come from one resolution) left a game that went on, and the Effect's
	// LosesGame trigger copied the loser's artifacts and creatures
	if y.contractLosses < 3 || y.goneOn < 3 {
		t.Fatalf("the pods never showed the armed class: %+v", y)
	}
	if y.copies == 0 {
		t.Fatalf("gorge made no tokens after a contracted loss: the LosesGame Effect trigger did not fire: %+v", y)
	}
}
