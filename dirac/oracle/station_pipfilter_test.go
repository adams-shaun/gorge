package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Station Planets with a coloured-fee filter (Evendo, Waking Haven and
// Uthros, Titanic Godcore): at 12 charge counters "{G}, {T}: Add {G} for each
// creature you control" ({U} per artifact) beside the land's free tap. The
// kernel plays it as a counters-gated one-pip fixed-colour filter (dirac
// filterTapMana, mana_filterpip.go): a paid census gap that gorge's prefix
// search activates only once the pool really pays the pip.
//
// Random games rarely reach 12 counters, so each case is a constructed
// matchup played by a policy that stations, plays lands and casts whenever it
// can, recorded once (PIPFILTER_RECORD=1) as a repro whose gorge log holds a
// pure multi-mana add of the land's colour: the filter's product, which
// nothing else in the deck makes. Replayed, both engines agree through the
// whole game.

type pipFilterCase struct {
	name, card, colour string
	decks              [2][]string
}

func pipFilterCases() []pipFilterCase {
	blank := deckOf(40, "Plains")
	// land-light decks whose four-mana spells (Giant Spider, Uthros Scanship) are
	// out of reach of the plain taps of three lands and within the filter's
	evendo := slices.Concat(deckOf(4, "Evendo, Waking Haven"), deckOf(10, "Grizzly Bears"), deckOf(4, "Wall of Wood"),
		deckOf(14, "Giant Spider"), deckOf(8, "Forest"))
	uthros := slices.Concat(deckOf(4, "Uthros, Titanic Godcore"), deckOf(6, "Coral Merfolk"), deckOf(4, "Golden Urn"), deckOf(4, "Shriekhorn"),
		deckOf(6, "Wurmwall Sweeper"), deckOf(8, "Uthros Scanship"), deckOf(8, "Island"))
	return []pipFilterCase{
		{name: "evendo_filter_pays_a_spell_with_a_green_fee", card: "Evendo, Waking Haven", colour: "G", decks: [2][]string{evendo, blank}},
		{name: "uthros_filter_pays_a_spell_with_a_blue_fee", card: "Uthros, Titanic Godcore", colour: "U", decks: [2][]string{uthros, blank}},
	}
}

// pipFilterPolicy stations, plays a land or casts whenever one is offered,
// else plays randomly (the answers to every ask are random too).
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

// pipFilterFired is a Check hook that scans gorge's log as it grows for the
// filter's product: a pure add of colour in an amount past one. at is the
// number of the decision (the Check call) that first saw it, 0 for none.
type pipFilterFired struct {
	colour  string
	seen    int
	checks  int
	at      int
	amount  int32
	charge  map[state.ObjID]int32 // charge counters per object, from the log
	most    int32                 // the most any object held
	checked func(*GorgeSide, Side) *Divergence
}

func (f *pipFilterFired) check(ref *GorgeSide, ker Side) *Divergence {
	f.checks++
	evs := ref.E.L.Events
	for ; f.seen < len(evs); f.seen++ {
		if e := &evs[f.seen]; e.Counter == "CHARGE" {
			if f.charge == nil {
				f.charge = map[state.ObjID]int32{}
			}
			f.charge[e.Obj] += e.Amount
			f.most = max(f.most, f.charge[e.Obj])
		}
		if e := &evs[f.seen]; e.Kind == events.ManaAdd && e.Amount >= 2 {
			println("DBGMANA", e.Counter, e.Amount, e.Text)
		}
		if e := &evs[f.seen]; f.at == 0 && e.Kind == events.ManaAdd && e.Counter == f.colour && e.Amount >= 2 && e.Text == "" {
			f.at, f.amount = f.checks, e.Amount
		}
	}
	if d := ref.E.Pending(); d != nil && d.Kind == decision.KPriority && ref.E.G.Active == 0 && ref.E.G.Step.IsMain() && len(ref.E.G.Stack) == 0 {
		g := ref.E.G
				var hand []string
		for _, id := range g.Zone(state.ZHand, 0) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Giant Spider" {
				hand = append(hand, o.Face().Name)
			}
		}
		if len(hand) > 0 {
			ev, lands, cr := 0, 0, 0
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				o := g.Obj(id)
				if o == nil || o.Face() == nil {
					continue
				}
				if o.Face().Name == "Evendo, Waking Haven" {
					ev += int(o.Counter("CHARGE"))
					if o.Tapped {
						ev += 1000
					}
				}
				if o.Face().Name == "Forest" {
					lands++
				}
				if o.Face().Name == "Grizzly Bears" || o.Face().Name == "Wall of Wood" {
					cr++
				}
			}
			cast := 0
			for _, op := range d.Options {
				if op.Kind == "cast" {
					cast++
				}
			}
			if ev > 0 {
				println("DBGPOS", f.checks, "hand", len(hand), "evendo charge", ev, "forests", lands, "creatures", cr, "cast options", cast)
			}
		}
	}
	if f.checked != nil {
		return f.checked(ref, ker)
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
	f := &pipFilterFired{colour: c.colour}
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
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom", MaxTurns: 40}
				res, f := pipFilterPlay(t, reg, c, r, pipFilterPolicy(seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if seed <= 12 {
					sp, cast := 0, 0
					for _, a := range res.Actions {
						switch a.Kind() {
						case ActSpecial:
							sp++
						case ActCast:
							cast++
						}
					}
					t.Logf("DBG seed %d: %d decisions, %d specials, %d casts, turns %d, most charge %d", seed, len(res.Actions), sp, cast, res.Turns, f.most)
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
