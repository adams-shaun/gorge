package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// cumScenario is a game from the cumulativecost fixture pool with name on
// player 0's battlefield carrying age age counters, advanced to the pay-or-
// decline ask of its next cumulative upkeep (the counter makes it age+1).
type cumScenario struct {
	sc   *scen
	src  Obj
	id   map[string]CardID
	k    *prep
	ask  Decision
	cost kop.ManaCost
}

func newCumScenario(t *testing.T, name string, age int, setup func(sc *scen, id map[string]CardID)) *cumScenario {
	t.Helper()
	set := cardsettest.Set(t, "cumulativecost")
	var deck []cardset.Entry
	for _, n := range []string{"Arctic Nishoba", "Dystopia", "Phobian Phantasm", "Phyrexian Soulgorger", "Polar Kraken", "Vexing Sphinx", "Yavimaya Ants"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	deck = append(deck, cardset.Entry{Name: "Swamp", Count: 1}, cardset.Entry{Name: "Island", Count: 1},
		cardset.Entry{Name: "Forest", Count: 1}, cardset.Entry{Name: "Hill Giant", Count: 10})
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	for _, n := range []string{name, "Swamp", "Island", "Forest", "Hill Giant"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s is not in the cumulativecost table", n)
		}
	}
	sc := newScenTableLib(t, tbl, id["Hill Giant"], Config{NoElide: true})
	s := sc.s
	c := &cumScenario{sc: sc, id: id, k: sc.k}
	c.src = sc.perm(id[name], 0)
	if !s.perms[sc.slot(c.src)].setCtrN(s, int(kop.CtrAge), age) {
		t.Fatal("precondition: the age counters did not take")
	}
	if setup != nil {
		setup(sc, id)
	}
	// pass to player 0's next upkeep; the trigger resolves into the ask
	for i := 0; i < 600 && !s.over && s.res.ask != raGenPay && s.res.ask != raGenPayPip; i++ {
		d := s.Step()
		if s.res.ask == raGenPay || s.res.ask == raGenPayPip {
			break
		}
		if d.Kind != DkPriority {
			done := false // an attack or block declaration: declare none; a trigger order: the first
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Kind() == AkDone {
					s.Apply(d.Options[j])
					done = true
					break
				}
			}
			if !done && d.Kind == DkTriggerOrder {
				s.Apply(d.Options[0])
				done = true
			}
			if !done {
				t.Fatalf("%s: decision kind %d before the cumulative upkeep ask (turn %d step %d)", name, d.Kind, s.turn, s.step)
			}
			continue
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if s.over {
		return c // a stalled game: the caller reads s.g.stalled
	}
	if s.res.ask != raGenPay && s.res.ask != raGenPayPip {
		t.Fatalf("%s: never reached the cumulative upkeep ask (turn %d step %d)", name, s.turn, s.step)
	}
	if !sc.onBF(c.src) || s.perms[sc.slot(c.src)].ctrN(s, int(kop.CtrAge)) != age+1 {
		t.Fatalf("%s: on battlefield %v with %d age counters, want %d", name, sc.onBF(c.src), s.perms[sc.slot(c.src)].ctrN(s, int(kop.CtrAge)), age+1)
	}
	c.ask = s.Step()
	return c
}

func (c *cumScenario) payCost() kop.ManaCost { return c.sc.s.resPayCost(c.k) }

func (c *cumScenario) answer(pay bool) {
	m := uint8(0)
	if pay {
		m = 1
	}
	c.sc.s.Apply(MakeAction(AkChoose, 0, 0, m))
}

// TestCumulativeUpkeepColouredAndLifeCosts: a cumulative upkeep paid with
// coloured or hybrid mana, life, a sacrifice or a discard asks gorge's one
// pay-or-decline over the printed cost times the age counters (CR 702.24a;
// gorge scaleCost, actionRemaining); a decline sacrifices the source.
func TestCumulativeUpkeepColouredAndLifeCosts(t *testing.T) {
	swamps := func(n int) func(*scen, map[string]CardID) {
		return func(sc *scen, id map[string]CardID) {
			for i := 0; i < n; i++ {
				sc.perm(id["Swamp"], 0)
			}
		}
	}

	t.Run("Phobian Phantasm asks BB with two age counters", func(t *testing.T) {
		for _, pay := range []bool{true, false} {
			c := newCumScenario(t, "Phobian Phantasm", 1, swamps(2))
			s := c.sc.s
			cost := c.payCost()
			if c.ask.Sub != ChGenPay || s.g.pickZ != kop.PayMana || s.g.pickF&pfNoPay != 0 || cost.Pips[kop.B] != 2 || cost.Generic != 0 {
				t.Fatalf("ask sub %d kind %d flags %#x cost %+v, want the pay-or-decline over {B}{B}", c.ask.Sub, s.g.pickZ, s.g.pickF, cost)
			}
			c.answer(pay)
			if got := c.sc.onBF(c.src); got != pay {
				t.Fatalf("pay=%v: source on the battlefield = %v", pay, got)
			}
			if pay && s.pl[0].pool[kop.B] != 0 {
				t.Fatalf("paid: %d black mana left floating", s.pl[0].pool[kop.B])
			}
		}
	})

	t.Run("one Swamp cannot pay BB", func(t *testing.T) {
		c := newCumScenario(t, "Phobian Phantasm", 1, swamps(1))
		if c.sc.s.g.pickF&pfNoPay == 0 {
			t.Fatalf("pickF %#x: the unaffordable {B}{B} is offered as a payment", c.sc.s.g.pickF)
		}
	})

	t.Run("Yavimaya Ants asks GGGG with two age counters", func(t *testing.T) {
		c := newCumScenario(t, "Yavimaya Ants", 1, nil)
		if cost := c.payCost(); cost.Pips[kop.G] != 4 {
			t.Fatalf("cost %+v, want {G}{G}{G}{G}", cost)
		}
	})

	t.Run("Arctic Nishoba announces each hybrid pip", func(t *testing.T) {
		c := newCumScenario(t, "Arctic Nishoba", 1, func(sc *scen, id map[string]CardID) {
			sc.perm(id["Forest"], 0)
			sc.perm(id["Forest"], 0)
		})
		s := c.sc.s
		for i := 0; i < 2; i++ {
			if s.res.ask != raGenPayPip || c.ask.Sub != ChGenPip || c.ask.N != 2 {
				t.Fatalf("pip %d: ask %d sub %d with %d options, want a two-face announcement", i, s.res.ask, c.ask.Sub, c.ask.N)
			}
			var green Action
			for j := 0; j < int(c.ask.N); j++ {
				if c.ask.Options[j].Mode() == uint8(kop.G) {
					green = c.ask.Options[j]
				}
			}
			if green.Mode() != uint8(kop.G) {
				t.Fatalf("pip %d: the announcement does not offer G: %v", i, c.ask.Options[:c.ask.N])
			}
			s.Apply(green)
			c.ask = s.Step()
		}
		if s.res.ask != raGenPay || c.ask.Sub != ChGenPay {
			t.Fatalf("after two pips: ask %d sub %d, want the pay-or-decline", s.res.ask, c.ask.Sub)
		}
		if cost := c.payCost(); cost.Pips[kop.G] != 2 || cost.Hyb[0] != 0 || s.g.pickF&pfNoPay != 0 {
			t.Fatalf("cost %+v flags %#x, want {G}{G} offered", cost, s.g.pickF)
		}
	})

	t.Run("a third hybrid pip stalls", func(t *testing.T) {
		c := newCumScenario(t, "Arctic Nishoba", 2, nil)
		if s := c.sc.s; !s.over || s.g.stalled != StallCumulativeCost {
			t.Fatalf("over %v stalled %d, want the cumulative cost capacity stall %d", s.over, s.g.stalled, StallCumulativeCost)
		}
	})

	t.Run("Dystopia asks 3 life with three age counters", func(t *testing.T) {
		for _, pay := range []bool{true, false} {
			c := newCumScenario(t, "Dystopia", 2, nil)
			s := c.sc.s
			if c.ask.Sub != ChGenPay || s.g.pickZ != kop.PayLife || s.g.pickLeft != 3 || s.g.pickF&pfNoPay != 0 {
				t.Fatalf("ask sub %d kind %d amount %d flags %#x, want the pay-or-decline over 3 life", c.ask.Sub, s.g.pickZ, s.g.pickLeft, s.g.pickF)
			}
			life := s.pl[0].life
			c.answer(pay)
			if got := c.sc.onBF(c.src); got != pay {
				t.Fatalf("pay=%v: source on the battlefield = %v", pay, got)
			}
			want := life
			if pay {
				want -= 3
			}
			if s.pl[0].life != want {
				t.Fatalf("pay=%v: life %d, want %d", pay, s.pl[0].life, want)
			}
		}
	})

	t.Run("Polar Kraken asks 2 lands with two age counters", func(t *testing.T) {
		c := newCumScenario(t, "Polar Kraken", 1, func(sc *scen, id map[string]CardID) {
			for i := 0; i < 3; i++ {
				sc.perm(id["Island"], 0)
			}
		})
		s := c.sc.s
		if c.ask.Sub != ChGenPay || s.g.pickZ != kop.PaySac || s.g.pickLeft != 2 || s.g.pickF&pfNoPay != 0 {
			t.Fatalf("ask sub %d kind %d amount %d flags %#x, want the pay-or-decline over 2 sacrificed lands", c.ask.Sub, s.g.pickZ, s.g.pickLeft, s.g.pickF)
		}
		c.answer(false)
		if c.sc.onBF(c.src) {
			t.Fatal("declined: the Kraken is still on the battlefield")
		}
	})

	t.Run("Phyrexian Soulgorger asks 2 creatures", func(t *testing.T) {
		c := newCumScenario(t, "Phyrexian Soulgorger", 1, func(sc *scen, id map[string]CardID) {
			sc.perm(id["Hill Giant"], 0)
		})
		s := c.sc.s
		if c.ask.Sub != ChGenPay || s.g.pickZ != kop.PaySac || s.g.pickLeft != 2 || s.g.pickF&pfNoPay != 0 {
			t.Fatalf("ask sub %d kind %d amount %d flags %#x, want the pay-or-decline over 2 sacrificed creatures (the source counts)", c.ask.Sub, s.g.pickZ, s.g.pickLeft, s.g.pickF)
		}
	})

	t.Run("Vexing Sphinx asks 2 cards with two age counters", func(t *testing.T) {
		c := newCumScenario(t, "Vexing Sphinx", 1, func(sc *scen, id map[string]CardID) {
			sc.hand(id["Hill Giant"], 0)
			sc.hand(id["Hill Giant"], 0)
		})
		s := c.sc.s
		if c.ask.Sub != ChGenPay || s.g.pickZ != kop.PayDiscard || s.g.pickLeft != 2 || s.g.pickF&pfNoPay != 0 {
			t.Fatalf("ask sub %d kind %d amount %d flags %#x, want the pay-or-decline over 2 discarded cards", c.ask.Sub, s.g.pickZ, s.g.pickLeft, s.g.pickF)
		}
	})
}
