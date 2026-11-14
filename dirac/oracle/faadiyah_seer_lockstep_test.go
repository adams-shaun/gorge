package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Fa'adiyah Seer's "{T}: Draw a card and reveal it. If it isn't a land card,
// discard it." lowers in lockstep: the draw remembers the drawn card
// (RememberDrawn$ True), the chained Discard Mode$ Defined
// DefinedCards$ Remembered discards it while it is still in hand
// (kop.OptDefined, no ask), and the node's Condition* gate skips the discard
// when the drawn card is a land. gorge's Draw ignores Reveal$ entirely
// (draw_params.go's drawKnownKeys omits it): the reveal is information only.

// faadiyahDeck is mono-green: Seers beside vanilla Bears, with enough Forests
// that the top of the library is usually a land (the no-discard half of the
// gate) but some draws turn a nonland up (the discard half).
func faadiyahDeck() []string {
	var d []string
	for range 4 {
		d = append(d, "Fa'adiyah Seer")
	}
	for range 8 {
		d = append(d, "Grizzly Bears")
	}
	for len(d) < 40 {
		d = append(d, "Forest")
	}
	return d
}

type faadiyahObs struct {
	acts     int // Fa'adiyah Seer activations gorge pushed
	discards int // discard events gorge emitted (only the Seer discards here)
}

// faadiyahTally reads gorge's log: the Seer's activations and its discards.
func (o *faadiyahObs) tally(g *GorgeSide) {
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.AbilityPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Fa'adiyah Seer" {
				o.acts++
			}
		}
		if events.IsDiscard(ev) {
			o.discards++
		}
	}
}

// faadiyahPolicy activates a Seer whenever it can (so the games reach the
// draw-reveal-discard chain), else plays a land, else casts, else random.
func faadiyahPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || seed%5 == 0 {
			return rnd.Pick(c)
		}
		var act, cast, land []Canon
		for _, o := range c.D.Options {
			switch {
			case o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Fa'adiyah Seer":
				act = append(act, o)
			case o.Kind() == ActCast:
				cast = append(cast, o)
			case o.Kind() == ActPlayLand:
				land = append(land, o)
			}
		}
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch {
		case len(act) > 0:
			return pick(act)
		case len(land) > 0:
			return pick(land)
		case len(cast) > 0:
			return pick(cast)
		}
		return rnd.Pick(c)
	})
}

// runFaadiyah plays the matchup; mutate makes the kernel's defined discard
// take nothing (the discard removed from the table: it must diverge).
func runFaadiyah(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *faadiyahObs) {
	t.Helper()
	obs = &faadiyahObs{}
	reg := synthReg(t)
	deck := faadiyahDeck()
	set := cardset.AllowUnverified(printedDelveSet(t, [2][]string{deck, deck}))
	for _, n := range deck {
		if _, ok := reg.Lookup(n); !ok {
			t.Fatalf("%s not in the corpus", n)
		}
	}
	if i, ok := set.Lookup("Fa'adiyah Seer"); !ok {
		t.Fatal("Fa'adiyah Seer is not in the table: it did not lower")
	} else if got := set.Status(i); got == cardset.StatusNotLowered {
		t.Fatalf("Fa'adiyah Seer status = %v, refusal = %q", got, set.Reason(i))
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
	noDiscard := dirac.DebugNoDefinedDiscard
	dirac.DebugNoDefinedDiscard = mutate
	defer func() { dirac.DebugNoDefinedDiscard = noDiscard }()
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
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"faadiyah-a", "faadiyah-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, faadiyahPolicy(m, seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20})
		obs.tally(g)
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, obs
}

// TestFaadiyahSeerLowers plays Fa'adiyah Seer games: the drawn nonland is
// discarded in both engines, the drawn land is not, and the games reach the
// activation.
func TestFaadiyahSeerLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, obs := runFaadiyah(t, 16, false)
	t.Logf("%d games, %d divergences, %d stalls, %d Seer activations, %d discards", games, divs, stalls, obs.acts, obs.discards)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.acts == 0 {
		t.Fatalf("no Fa'adiyah Seer was activated: the draw-reveal-discard chain was never reached")
	}
	if obs.discards == 0 {
		t.Fatalf("no card was discarded: the nonland arm of the gate never ran")
	}
}

// TestFaadiyahSeerMutation removes the kernel's defined discard from the
// table: gorge still discards the drawn nonland, so the pair diverges.
func TestFaadiyahSeerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runFaadiyah(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation removed the discard in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
