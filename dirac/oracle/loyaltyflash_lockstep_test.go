package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// loyaltyFlashDeck carries the four lowered loyalty-timing grants (the
// CastWithFlash Activated.Loyalty statics, kop.StatLoyaltyFlash): The
// Wandering Emperor's printed Card.Self+ThisTurnEntered line, Jace's
// Machinations' EOT Effect over the Jace token its Empower makes, and Teferi,
// Temporal Archmage's and Teferi's Talent's emblems; four bears for the
// Emperor's exile, and 20 lands of the deck's colours.
var loyaltyFlashDeck = func() []string {
	var d []string
	for _, n := range []string{"The Wandering Emperor", "Jace's Machinations", "Teferi, Temporal Archmage", "Teferi's Talent", "Grizzly Bears"} {
		for range 4 {
			d = append(d, n)
		}
	}
	for range 10 {
		d = append(d, "Plains")
	}
	for range 10 {
		d = append(d, "Island")
	}
	return d
}()

// loyaltyFlashReached reports whether some planeswalker activated a loyalty
// ability outside a main phase: only the grant times it there.
func loyaltyFlashReached(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	step := state.Step(0)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.AbilityPush:
			if step == state.StepMain1 || step == state.StepMain2 {
				continue
			}
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().IsPlaneswalker() {
				return true
			}
		}
	}
	return false
}

// loyaltyFlashPolicy casts the Emperor and Machinations whenever it can and
// otherwise activates a planeswalker whenever it can; every other choice is
// random.
func loyaltyFlashPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		var cast, act []Canon
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
				if n := ob.Face().Name; n == "The Wandering Emperor" || n == "Jace's Machinations" {
					cast = append(cast, a)
				}
			case ActActivate:
				if ob.Face().IsPlaneswalker() {
					act = append(act, a)
				}
			}
		}
		if len(cast) > 0 {
			return cast[0]
		}
		if len(act) > 0 {
			return act[0]
		}
		return rnd.Pick(c)
	})
}

// runLoyaltyFlashLockstep plays the deck against itself; mutate ignores the
// grant in the kernel, which must diverge once gorge offers an instant-speed
// loyalty activation.
func runLoyaltyFlashLockstep(t *testing.T, seeds uint64, mutate bool) (games, divs, reached int) {
	t.Helper()
	reg := synthReg(t)
	deck := loyaltyFlashDeck
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
	old := dirac.DebugLoyaltyFlashOff
	dirac.DebugLoyaltyFlashOff = mutate
	defer func() { dirac.DebugLoyaltyFlashOff = old }()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"loyaltyflash-a", "loyaltyflash-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, loyaltyFlashPolicy(seed), RunConfig{
			Config: cfg, MaxDecisions: 3500, MaxTurns: 30,
			Finish: func(g *GorgeSide) { hit = loyaltyFlashReached(g) },
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

// TestLoyaltyFlashLockstep: the four lowered loyalty-timing carriers match
// gorge's loyaltyAtInstantSpeed at every decision, and the games reach a
// loyalty activation outside a main phase.
func TestLoyaltyFlashLockstep(t *testing.T) {
	games, divs, reached := runLoyaltyFlashLockstep(t, 24, false)
	if divs != 0 {
		t.Fatalf("%d of %d games diverged", divs, games)
	}
	if reached == 0 {
		t.Fatalf("%d games: none reached an instant-speed loyalty activation, so the lockstep is vacuous", games)
	}
	t.Logf("%d games, %d reached an instant-speed loyalty activation", games, reached)
}

// TestLoyaltyFlashMutation: with the grant ignored, the kernel withholds the
// instant-speed activation gorge offers, so at least one game must diverge.
func TestLoyaltyFlashMutation(t *testing.T) {
	games, divs, _ := runLoyaltyFlashLockstep(t, 24, true)
	if divs == 0 {
		t.Fatalf("%d games with the grant ignored: no divergence, so the lockstep never depended on it", games)
	}
}
