package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Ajani Steadfast's -7 (Effect, Duration$ Permanent): "You get an emblem with
// 'If a source would deal damage to you or a planeswalker you control, prevent
// all but 1 of that damage.'" gorge registers the line for the rest of the
// game; the kernel's emblem carries it as a static all-but-N prevention
// (dirac/repl_staticprevent_emblem.go). Seat 0 plays Ajani Steadfast and
// activates +1 until the -7 is offered; seat 1 plays Shocks at seat 0's face.
// Every Shock after the -7 deals 1, not 2.

// ajaniEmblemObs is what the games reached: the -7 activations, and the
// damage events of exactly 1 to seat 0 after one (a Shock deals 2).
type ajaniEmblemObs struct {
	ultimates int
	reduced   int
}

func ajaniEmblemDecks() [2][]string {
	return [2][]string{
		append(deckOf(8, "Ajani Steadfast"), deckOf(16, "Plains")...),
		append(deckOf(12, "Shock"), deckOf(12, "Mountain")...),
	}
}

// ajaniEmblemPolicy: seat 0 casts Ajani, plays lands and activates the -7 when
// offered, else the +1; seat 1 casts its Shocks and aims every target at
// seat 0. at records the log length when the -7 is picked.
func ajaniEmblemPolicy(m *Matchup, seed uint64, at *int) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		pick := func(opts []Canon) Canon {
			d := *c.D
			d.Options = opts
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch c.D.Kind {
		case DTarget:
			if c.D.Player == 1 {
				for _, o := range c.D.Options {
					if o.Kind() == ActTarget && o.Tgt() == RefPlayer {
						return o
					}
				}
			}
		case DPriority:
			var ult, plus, cast, land []Canon
			for _, o := range c.D.Options {
				switch {
				case o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Ajani Steadfast" && o.Mode() == 2:
					ult = append(ult, o)
				case o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Ajani Steadfast" && o.Mode() == 0:
					plus = append(plus, o)
				case o.Kind() == ActCast:
					cast = append(cast, o)
				case o.Kind() == ActPlayLand:
					land = append(land, o)
				}
			}
			switch {
			case len(ult) > 0:
				if c.G != nil && *at == 0 {
					*at = len(c.G.E.L.Events)
				}
				return ult[0]
			case len(land) > 0:
				return pick(land)
			case len(plus) > 0:
				return plus[0]
			case len(cast) > 0:
				return pick(cast)
			}
		}
		return rnd.Pick(c)
	})
}

// runAjaniEmblem plays the matchup over seeds; mutate leaves the emblem's
// prevention out of the kernel (the mutation must diverge).
func runAjaniEmblem(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *ajaniEmblemObs) {
	t.Helper()
	reg := synthReg(t)
	deck := ajaniEmblemDecks()
	set := cardset.AllowUnverified(printedDelveSet(t, deck))
	old := dirac.DebugNoEmblemPrev
	dirac.DebugNoEmblemPrev = mutate
	defer func() { dirac.DebugNoEmblemPrev = old }()
	obs = &ajaniEmblemObs{}
	var decks [2][]*cards.Card
	var entries [2][]cardset.Entry
	for s := range decks {
		for _, n := range deck[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ajaniemblem-a", "ajaniemblem-b"}, decks, 1)
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
		at := 0
		res := LockstepLed(m, seed, g, k, ajaniEmblemPolicy(m, seed, &at), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		if at != 0 {
			obs.ultimates++
			for _, e := range g.E.L.Events[at:] {
				if e.Kind == events.Damage && e.Obj == 0 && e.Player == 0 && e.Amount == 1 {
					obs.reduced++
				}
			}
		}
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

// TestAjaniEmblemLockstep plays Ajani Steadfast games to the -7: no divergence
// and no stall, and some game deals seat 0 damage reduced to 1 after it.
func TestAjaniEmblemLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, obs := runAjaniEmblem(t, 60, false)
	t.Logf("%d games, %d divergences, %d stalls, %d -7 activations, %d damage events reduced to 1", games, divs, stalls, obs.ultimates, obs.reduced)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.ultimates == 0 || obs.reduced == 0 {
		t.Fatalf("%d -7 activations, %d reduced damage events: the emblem's prevention was never reached", obs.ultimates, obs.reduced)
	}
}

// TestAjaniEmblemMutation: with the emblem's prevention never collected, the
// kernel deals the full damage gorge reduces, so some game diverges.
func TestAjaniEmblemMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runAjaniEmblem(t, 60, true)
	if divs == 0 {
		t.Fatalf("the mutation did not diverge in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
