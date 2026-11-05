package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Peace in Our Time lockstep: its beginning-of-combat trigger is
// OptionalDecider$ TriggeredPlayer, so on each player's turn that player
// (not the enchantment's controller) answers the CR 603.5 yes/no; a yes draws
// a card for them, bars their creatures from attacking this turn and puts a
// peace counter on the enchantment. Both engines must pose the ask to the
// same seat with the same options, and a yes must stop the same attackers.

var peaceDeck = []cardset.Entry{
	{Name: "Peace in Our Time", Count: 6}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 3},
	{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Mountain", Count: 10},
}

type peaceTally struct {
	asks      int // optional-trigger asks posed for the enchantment
	askedOpp  int // asks whose seat is not the enchantment's controller
	askedSelf int // asks whose seat is its controller
	yes, no   int
	wrongSeat int   // asks posed to a seat that is not the active player
	attackYes int   // attack declarations offering an attacker to a player who said yes earlier this turn: must stay 0
	attackOK  int   // attack declarations offering an attacker to a player who did not say yes this turn
	yesTurn   int32 // the last turn on which the active player answered yes (-1: none)
}

func peacePolicy(seed uint64, t *peaceTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		} else {
			a = rnd.Pick(c)
		}
		if c.G != nil {
			peaceCount(c, a, t)
		}
		return a
	})
}

func peaceCount(c *PickCtx, a Canon, t *peaceTally) {
	pd := c.G.Pending()
	if pd == nil {
		return
	}
	g := c.G.E.G
	switch {
	case pd.Kind == decision.KTriggerOptional && pd.ResumeKind == "optional":
		src := g.Obj(pd.Source)
		if src == nil || src.Face() == nil || src.Face().Name != "Peace in Our Time" {
			return
		}
		t.asks++
		if state.PlayerID(pd.Player) == src.Controller {
			t.askedSelf++
		} else {
			t.askedOpp++
		}
		if state.PlayerID(pd.Player) != g.Active {
			t.wrongSeat++
		}
		if opt, ok := c.G.GorgeOption(a); ok {
			switch opt.Kind {
			case "yes":
				t.yes++
				t.yesTurn = g.Turn
			case "no":
				t.no++
			}
		}
	case pd.Kind == decision.KAttackers:
		offered := false
		for _, o := range pd.Options {
			if o.Kind == "attacker" {
				offered = true
				break
			}
		}
		switch {
		case !offered:
		case t.yesTurn == g.Turn:
			t.attackYes++ // the can't-attack effect must leave no attacker to declare
		default:
			t.attackOK++
		}
	}
}

func peaceSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, [2][]cardset.Entry) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("peaceinourtime")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = peaceDeck
		for _, e := range peaceDeck {
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
	return reg, set, decks, ents
}

// peaceRun plays seeds 1..n; the first divergence is logged.
func peaceRun(t *testing.T, n uint64, tally *peaceTally, stopOnDiv bool) (div, stalls int) {
	t.Helper()
	reg, set, decks, ents := peaceSetup(t)
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"peace-a", "peace-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.yesTurn = -1
		res := Lockstep(m, seed, k, peacePolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if div == 1 {
				t.Logf("seed %d diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if stopOnDiv {
				return div, stalls
			}
		}
	}
	return div, stalls
}

func TestPeaceInOurTimeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tally := &peaceTally{}
	div, _ := peaceRun(t, 200, tally, false)
	if div != 0 {
		t.Errorf("%d divergent games", div)
	}
	t.Logf("%+v", *tally)
	if tally.asks < 40 {
		t.Errorf("%d Peace asks, want at least 40", tally.asks)
	}
	if tally.askedOpp < 10 || tally.askedSelf < 10 {
		t.Errorf("asks to the opponent %d, to the controller %d, want at least 10 of each", tally.askedOpp, tally.askedSelf)
	}
	if tally.wrongSeat != 0 {
		t.Errorf("%d asks went to a seat that was not the active player", tally.wrongSeat)
	}
	if tally.attackYes != 0 || tally.attackOK < 20 {
		t.Errorf("attackers offered after a yes: %d (want 0); after a no or no ask: %d (want at least 20)", tally.attackYes, tally.attackOK)
	}
	if tally.yes < 10 || tally.no < 10 {
		t.Errorf("answers yes %d no %d, want at least 10 of each", tally.yes, tally.no)
	}
}

// TestPeaceInOurTimeMutation makes the controller the decider again and
// requires a divergence.
func TestPeaceInOurTimeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugOptDeciderCtrl = true
	defer func() { dirac.DebugOptDeciderCtrl = false }()
	if div, _ := peaceRun(t, 200, &peaceTally{}, true); div == 0 {
		t.Errorf("mutation: no divergence in 200 games")
	}
}
