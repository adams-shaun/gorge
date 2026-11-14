package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Ghitu Slinger's echo ({2}{R}) is offered on the payer's POTENTIAL mana
// (rules respay.go resPayNeedsMana over potential.go), whose walk counts
// every mana ability of every untapped source -- including a bounded rider
// source (Hickory Woodlot's "{T}, Remove a depletion counter: Add {G}{G}",
// a Deplete rider). The kernel's offer gate used to require the census tap
// flag, whose exactly-{T} test a Tap+SubCounter cost fails, so on a board
// whose non-Woodlot sources fall short of the echo (a Mountain and an
// Island make two units of the three the echo costs) gorge offered pay and
// decline while the kernel offered decline only. The deck below reaches
// those boards in its first echoes: a Slinger cast on curve taps its
// controller's lands, and the echo asks the next upkeep.

// echoWoodlotDeck is Ghitu Slingers beside Mountains (the echo's {R} pip),
// Hickory Woodlots (the bounded rider) and Islands.
func echoWoodlotDeck() []string {
	var d []string
	for range 8 {
		d = append(d, "Ghitu Slinger")
	}
	for range 8 {
		d = append(d, "Mountain")
	}
	for range 8 {
		d = append(d, "Hickory Woodlot")
	}
	for len(d) < 40 {
		d = append(d, "Island")
	}
	return d
}

// echoWoodlotObs records what the games reached: how often the echo's pay
// arm was on offer (non-vacuity), and how often the policy paid one.
type echoWoodlotObs struct {
	pays   int
	paid   int
	sacs   int
	slings int // Slingers cast (the games reach the echoes)
}

// echoWoodlotPolicy plays lands and casts Slingers on curve, pays an echo
// whenever its pay arm is offered (exercising the Woodlot funding) and
// counts the asks.
func echoWoodlotPolicy(seed uint64, obs *echoWoodlotObs) Policy {
	rnd := RandomPolicy(seed)
	payCode, okPay := canon.ChooseCode("trigger_cost_pay")
	declineCode, okDecline := canon.ChooseCode("trigger_cost_decline")
	payKind, declineKind := canon.ActChoose + canon.CanonKind(payCode), canon.ActChoose + canon.CanonKind(declineCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		if okPay && okDecline {
			var pay, decline Canon
			hasPay, hasDecline := false, false
			for _, o := range c.D.Options {
				switch o.Kind() {
				case payKind:
					pay, hasPay = o, true
					obs.pays++
				case declineKind:
					decline, hasDecline = o, true
				}
			}
			switch {
			case hasPay:
				obs.paid++
				return pay
			case hasDecline:
				return decline
			}
		}
		if c.D.Kind != DPriority || seed%5 == 0 {
			return rnd.Pick(c)
		}
		var land, cast []Canon
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActPlayLand:
				land = append(land, o)
			case ActCast:
				cast = append(cast, o)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(land) > 0:
			return pick(land)
		case len(cast) > 0:
			if c.G != nil {
				obs.slings += len(cast)
			}
			return pick(cast)
		}
		return rnd.Pick(c)
	})
}

// TestGhituSlingerEchoWoodlotLockstep plays the Ghitu Slinger + Hickory
// Woodlot mirror: the echo's pay arm is offered in both engines wherever it
// is offered in one, the elected pay funding it through the Woodlot's
// counter-removal arm when the other sources fall short.
func TestGhituSlingerEchoWoodlotLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	deck := echoWoodlotDeck()
	set := cardset.AllowUnverified(printedDelveSet(t, [2][]string{deck, deck}))
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range deck {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(deck), entries(deck))
	if err != nil {
		t.Fatal(err)
	}
	obs := &echoWoodlotObs{}
	const seeds = 16
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"echo-woodlot-a", "echo-woodlot-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, echoWoodlotPolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d echo pay arms offered, %d paid, %d sacrifices, %d Slinger cast asks", seeds, obs.pays, obs.paid, obs.sacs, obs.slings)
	if obs.pays == 0 {
		t.Fatalf("no echo pay arm was offered in %d games: the ask was never reached", seeds)
	}
}
