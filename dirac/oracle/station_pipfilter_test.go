package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Station Planets with a coloured-fee filter (Evendo, Waking Haven and
// Uthros, Titanic Godcore): at 12 charge counters "{G}, {T}: Add {G} for each
// creature you control" ({U} per artifact) beside the land's free tap. The
// kernel plays it as a counters-gated one-pip fixed-colour filter (dirac
// filterTapMana, mana_filterpip.go). gorge lists that granted ability only
// while the live pool pays its pip, so a game reaches it only with the colour
// floating at a decision; the lockstep never taps a land by hand, so here
// Crumbling Vestige's enters trigger floats it. On the autopay surface a
// plain cast the planner prices only through a prefix script is offered by
// neither engine (gorge.go's autopay vocabulary), so the games are played on
// the full surface, where gorge's payment window lists the filter.
//
// Random games rarely reach 12 counters, so each case is a constructed
// matchup played by a policy that stations, plays lands and casts its
// costliest spell whenever it can, recorded once (PIPFILTER_RECORD=1) as a
// repro whose gorge log holds the filter's product: a pure add of the land's
// colour in an amount past one, right after the Planet tapped. Replayed, both
// engines agree through the whole game. Each repro is also a row of
// TestStdSetReprosLockstep.

type pipFilterCase struct {
	name, card, colour string
	decks              [2][]string
}

func pipFilterCases() []pipFilterCase {
	blank := deckOf(40, "Plains")
	// land-light decks of cheap bodies and seven-mana spells, with Crumbling
	// Vestige, whose enters trigger adds one mana of any colour
	evendo := slices.Concat(deckOf(4, "Evendo, Waking Haven"), deckOf(14, "Grizzly Bears"), deckOf(4, "Wall of Wood"),
		deckOf(5, "Enormous Baloth"), deckOf(5, "Kalonian Behemoth"), deckOf(6, "Crumbling Vestige"), deckOf(2, "Forest"))
	uthros := slices.Concat(deckOf(4, "Uthros, Titanic Godcore"), deckOf(6, "Coral Merfolk"), deckOf(4, "Golden Urn"), deckOf(4, "Shriekhorn"),
		deckOf(4, "Wurmwall Sweeper"), deckOf(2, "Uthros Scanship"), deckOf(4, "Pinnacle Kill-Ship"), deckOf(6, "Crumbling Vestige"), deckOf(6, "Island"))
	return []pipFilterCase{
		{name: "evendo_filter_pays_a_spell_with_a_green_fee", card: "Evendo, Waking Haven", colour: "G", decks: [2][]string{evendo, blank}},
		{name: "uthros_filter_pays_a_spell_with_a_blue_fee", card: "Uthros, Titanic Godcore", colour: "U", decks: [2][]string{uthros, blank}},
	}
}

// pipFilterPolicy stations, plays a land or casts (the costliest spell, by
// gorge's card) whenever one is offered, else plays randomly (the answers to
// every ask are random too).
func pipFilterPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			for _, want := range []CanonKind{ActSpecial, ActPlayLand, ActCast} {
				var buf []Canon
				for _, a := range c.D.Options {
					if a.Kind() == want {
						buf = append(buf, a)
					}
				}
				if want == ActCast && len(buf) > 1 && c.G != nil {
					buf = costliest(c.G, buf)
				}
				if len(buf) > 0 {
					d := *c.D
					d.Options = buf
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
			}
		}
		return rnd.Pick(c)
	})
}

// costliest keeps the casts of casts whose card has the largest mana value.
func costliest(g *GorgeSide, casts []Canon) []Canon {
	mv := map[Ref]int32{} // lookup only
	for _, p := range g.E.G.AliveFrom(0) {
		for _, id := range g.E.G.Zone(state.ZHand, p) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
				mv[g.InstRef(id)] = o.Face().ManaValue()
			}
		}
	}
	best := int32(-1)
	for _, a := range casts {
		best = max(best, mv[a.Src()])
	}
	var out []Canon
	for _, a := range casts {
		if mv[a.Src()] == best {
			out = append(out, a)
		}
	}
	return out
}

// pipFilterFired is a Check hook that scans gorge's log as it grows for the
// filter's product: a pure add of colour in an amount past one, the next
// mana event after the Planet itself tapped (its free ability makes one). at
// is the number of the decision (the Check call) that first saw it, 0 for
// none.
type pipFilterFired struct {
	card, colour string
	seen         int
	checks       int
	tapped       bool // the last Tap in the log was the Planet's
	at           int
	amount       int32
}

func (f *pipFilterFired) check(ref *GorgeSide, ker Side) *Divergence {
	f.checks++
	evs := ref.E.L.Events
	for ; f.seen < len(evs); f.seen++ {
		e := &evs[f.seen]
		switch e.Kind {
		case events.Tap:
			o := ref.E.G.Obj(e.Obj)
			f.tapped = o != nil && o.Face() != nil && o.Face().Name == f.card
		case events.ManaAdd:
			if f.at == 0 && f.tapped && e.Counter == f.colour && e.Amount >= 2 && e.Text == "" {
				f.at, f.amount = f.checks, e.Amount
			}
			f.tapped = false
		}
	}
	return nil
}

func pipFilterPlay(t *testing.T, reg *cards.Registry, c pipFilterCase, r *Repro, pol Policy) (Result, *pipFilterFired) {
	t.Helper()
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	f := &pipFilterFired{card: c.card, colour: c.colour}
	rc := r.RunConfig()
	rc.Check = f.check
	rc.MaxDecisions = 4000
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
		rc.MaxDecisions = len(r.Actions) + 200
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), f
}

// TestPipFilterRecord plays the cases over seeds until one reaches the
// filter's product with the engines in agreement, and writes the repro. It
// runs only with PIPFILTER_RECORD=1; the games it writes are the committed
// repros.
func TestPipFilterRecord(t *testing.T) {
	if os.Getenv("PIPFILTER_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range pipFilterCases() {
		t.Run(c.card, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "full", Kernel: "custom", MaxTurns: 40}
				res, f := pipFilterPlay(t, reg, c, r, pipFilterPolicy(seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if f.at == 0 {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: the filter made %d at decision %d of %d", seed, f.amount, f.at, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s's filter", c.card)
		})
	}
}

// TestPipFilterReprosLockstep replays the recorded games: no divergence,
// every scripted action offered, and the filter's product made (a pure
// multi-mana add of the land's colour) -- the lockstep alone would accept a
// script that never reached it.
func TestPipFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range pipFilterCases() {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res, f := pipFilterPlay(t, reg, c, r, nil)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if f.at == 0 {
				t.Fatalf("the recorded game never made %s's filter product (%s x2+)", c.card, c.colour)
			}
			t.Logf("%s: the filter made %d %s at decision %d of %d", c.card, f.amount, c.colour, f.at, len(r.Actions))
		})
	}
}
