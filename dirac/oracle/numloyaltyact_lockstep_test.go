package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// numLoyaltyActDeck is Oath of Teferi's "activate each planeswalker's loyalty
// abilities twice each turn" (the printed S:Mode$ NumLoyaltyAct) beside Jace
// Beleren's cheap +2, the pair that reaches a second activation: four Oaths,
// four Jaces, four bears for the Oath's exile-a-permanent enters trigger, and
// 28 lands of Oath's and Jace's colours.
var numLoyaltyActDeck = func() []string {
	var d []string
	for range 4 {
		d = append(d, "Oath of Teferi")
	}
	for range 4 {
		d = append(d, "Jace Beleren")
	}
	for range 4 {
		d = append(d, "Grizzly Bears")
	}
	for range 8 {
		d = append(d, "Plains")
	}
	for range 10 {
		d = append(d, "Island")
	}
	for range 10 {
		d = append(d, "Forest")
	}
	return d
}()

// numLoyaltyActDeckEntries is the deck's distinct cards, in deck order, each
// count one: Matchup only needs the card set, the 40-card lists carry the
// duplicates.
func numLoyaltyActDeckEntries(deck []string) []cardset.Entry {
	seen := map[string]bool{}
	var es []cardset.Entry
	for _, n := range deck {
		if !seen[n] {
			seen[n] = true
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
	}
	return es
}

// numLoyaltyActDouble reports whether some planeswalker activated two
// abilities in one turn. The event log is turn-bounded by TurnChange, so the
// check works after the game, when the walker may have left.
func numLoyaltyActDouble(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	turn := int32(-1)
	seen := map[[2]int32]int{}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TurnChange:
			turn++
		case events.AbilityPush:
			ob := g.E.G.Obj(ev.Obj)
			if ob == nil || ob.Face() == nil || !ob.Face().IsPlaneswalker() {
				continue
			}
			k := [2]int32{turn, int32(ev.Obj)}
			seen[k]++
			if seen[k] >= 2 {
				return true
			}
		}
	}
	return false
}

// numLoyaltyActPolicy casts Oath of Teferi whenever it can and otherwise
// activates a planeswalker whenever it can, so the games reach the second
// activation the static grants; every other choice is random.
func numLoyaltyActPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		var oath, act []Canon
		for _, a := range c.D.Options {
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			ob := c.G.E.G.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch a.Kind() {
			case ActCast:
				if ob.Face().Name == "Oath of Teferi" {
					oath = append(oath, a)
				}
			case ActActivate:
				if ob.Face().IsPlaneswalker() {
					act = append(act, a)
				}
			}
		}
		if len(oath) > 0 {
			return oath[0]
		}
		if len(act) > 0 {
			return act[0]
		}
		return rnd.Pick(c)
	})
}

// runNumLoyaltyActLockstep plays the Oath/Jace matchup; mutate ignores the
// static in the kernel, which must diverge once a second activation is offered.
func runNumLoyaltyActLockstep(t *testing.T, seeds uint64, mutate bool) (games, divs, reached int) {
	t.Helper()
	reg := synthReg(t)
	deck := numLoyaltyActDeck
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
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(numLoyaltyActDeckEntries(deck), numLoyaltyActDeckEntries(deck))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugNumLoyaltyActOff
	dirac.DebugNumLoyaltyActOff = mutate
	defer func() { dirac.DebugNumLoyaltyActOff = old }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"numloyaltyact-a", "numloyaltyact-b"}, decks, 1)
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
		hit := false
		res := LockstepLed(m, seed, g, k, numLoyaltyActPolicy(seed), RunConfig{
			Config: cfg, MaxDecisions: 3500, MaxTurns: 30,
			Finish: func(g *GorgeSide) { hit = numLoyaltyActDouble(g) },
		})
		CloseSide(k)
		games++
		if hit {
			reached++
		}
		if res.Div != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
			}
		}
		if !mutate {
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Fatal(err)
			}
		}
	}
	return games, divs, reached
}

// TestNumLoyaltyActLockstep: Oath of Teferi's Twice$ static matches gorge's
// loyalty-limit answer at every decision, and the games reach a second
// activation of one planeswalker in a turn (without the static that second
// activation is never offered).
func TestNumLoyaltyActLockstep(t *testing.T) {
	games, divs, reached := runNumLoyaltyActLockstep(t, 32, false)
	if divs != 0 {
		t.Fatalf("%d of %d games diverged", divs, games)
	}
	if reached == 0 {
		t.Fatalf("%d games: none reached a second loyalty activation, so the lockstep is vacuous", games)
	}
}

// TestNumLoyaltyActMutation: with the static ignored, the kernel withholds the
// second activation gorge offers, so at least one game must diverge.
func TestNumLoyaltyActMutation(t *testing.T) {
	games, divs, _ := runNumLoyaltyActLockstep(t, 32, true)
	if divs == 0 {
		t.Fatalf("%d games with the static ignored: no divergence, so the lockstep never depended on it", games)
	}
}

// TestNumLoyaltyActLockstepFourSeat runs the same Oath/Jace matchup at four
// seats through fourSeatSetPlay (the kernel's InitN path). The static is
// per-permanent and reads no seat, so the four-seat games must agree too; at
// least one game must reach a second activation, or the run is vacuous.
func TestNumLoyaltyActLockstepFourSeat(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	deck := numLoyaltyActDeck
	set := printedDelveSetIn(t, cardsDir, [2][]string{deck, deck})
	reached := 0
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck}, nil, nil,
		func(g *GorgeSide) {
			if numLoyaltyActDouble(g) {
				reached++
			}
		}, numLoyaltyActPolicy)
	const seeds = 12
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: unmodelled stall %q", seed, res.Stall)
		}
	}
	if reached == 0 {
		t.Fatalf("%d four-seat games: none reached a second loyalty activation, so the lockstep is vacuous", seeds)
	}
}
