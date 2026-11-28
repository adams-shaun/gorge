package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Any player may pay" (UnlessPayer$ Player) in a four-seat pod: gorge
// (effects/unless.go unlessPayerTargetsFor) asks every living player in
// AliveFrom(0) seat order -- seat 0 first, whoever controls the resolving
// object -- until one pays, and skips a player who has left the game (CR
// 800.4a, 800.4f). The kernel walks the same seats
// (dirac/cost_anyplayerpays.go). The pod lockstep pins that both engines ask
// the same seats in the same order.
//
// Every unless ask in these decks is an any-player ask: Browbeat and Book
// Burning (any player may take damage; the damage costs seats their lives, so
// seats leave while the game goes on), Rhystic Shield (any player may pay
// {2}), Carrion Rats (any player may exile a card from their graveyard, "if a
// player does") and Dash Hopes (any player may pay 5 life, "if a player
// does"). Dash Hopes is a one-of: with more, one can counter another whose
// cast trigger is still on the stack, where the kernel's counter takes the
// wrong stack entry (dirac/act_counter.go execCounter), which is not the walk
// this test pins.

var anyPlayerPaysPodDecks = [4][]cardset.Entry{
	{{Name: "Browbeat", Count: 10}, {Name: "Mountain", Count: 14}},
	{{Name: "Rhystic Shield", Count: 8}, {Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 12}},
	{{Name: "Dash Hopes", Count: 1}, {Name: "Carrion Rats", Count: 8}, {Name: "Swamp", Count: 14}},
	{{Name: "Book Burning", Count: 10}, {Name: "Mountain", Count: 14}},
}

// anyPlayerPaysTally counts the unless asks gorge posed, by the seat asked.
type anyPlayerPaysTally struct {
	asked, paid [MaxSeats]int
	// seat0Gone counts the asks posed after seat 0 had left the game;
	// departed counts asks posed to a seat that had left (CR 800.4f: none).
	seat0Gone, departed int
}

// tallyPolicy wraps p, counting each unless ask the leading gorge side poses
// and the pay answers p gives it.
func (y *anyPlayerPaysTally) tallyPolicy(p Policy) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		a := p.Pick(c)
		d := c.G.Pending()
		if d == nil || d.ResumeKind != "unless_pay" {
			return a
		}
		y.asked[d.Player]++
		if c.G.E.G.Players[d.Player].Lost {
			y.departed++
		}
		if c.G.E.G.Players[state.PlayerID(0)].Lost {
			y.seat0Gone++
		}
		for i, o := range c.D.Options {
			if o == a && len(d.Options) == len(c.D.Options) && d.Options[i].Mode == decision.ModeUnlessPay {
				y.paid[d.Player]++
			}
		}
		return a
	})
}

// TestFourSeatAnyPlayerPaysLockstep plays the pod over seeds in lockstep: no
// game diverges or stalls, and across them seats 2 and 3 are asked (and pay),
// and asks are posed after seat 0 has left the game.
func TestFourSeatAnyPlayerPaysLockstep(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 4)
	gd := make([][]*cards.Card, 4)
	md := make([][]cardset.Entry, 4)
	var all []string
	for s, d := range anyPlayerPaysPodDecks {
		names[s] = "any-player-pays-pod"
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
	var y anyPlayerPaysTally
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
		pol := y.tallyPolicy(castFirstPolicy(seed))
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
			break
		}
		if t.Failed() {
			break
		}
	}
	t.Logf("%+v", y)
	if y.departed != 0 {
		t.Errorf("gorge asked a departed seat %d time(s) (CR 800.4f)", y.departed)
	}
	if y.asked[2] == 0 || y.asked[3] == 0 {
		t.Fatalf("the pods never asked seats 2 and 3: %+v", y)
	}
	if y.paid[2]+y.paid[3] == 0 {
		t.Fatalf("neither seat 2 nor seat 3 ever paid: %+v", y)
	}
	if y.seat0Gone == 0 {
		t.Fatalf("no unless ask was posed after seat 0 had left the game: %+v", y)
	}
}
