package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Arcane Denial (Counter RememberTargets$, a delayed upkeep trigger over the
// countered spell's controller, Draw Upto$) is played in lockstep: seat 1
// counters seat 0's Grizzly Bears, and at the next upkeep seat 0, not the
// caster, is asked how many of up to two cards to draw while seat 1 draws
// its one. TestArcaneDenialReproResolves replays the recorded game and
// requires the draws to happen, since a lockstep entry alone would pass a
// script that never countered anything. TestArcaneDenialRecord records it
// (ARCANE_DENIAL_RECORD=1).

// arcaneDenialCases: the countered spell's controller draws both cards, or
// declines (the empty answer, legal at Min 0); the caster draws its one
// either way.
var arcaneDenialCases = []struct {
	name string
	want int // the cards seat 0 takes of the up to two
}{
	{"arcane_denial_draws_for_the_countered_spells_controller", 2},
	{"arcane_denial_controller_declines_to_draw", 0},
}

var arcaneDenialDecks = [2][]string{
	append(deckOf(5, "Grizzly Bears"), deckOf(19, "Forest")...),
	append(deckOf(7, "Arcane Denial"), deckOf(17, "Island")...),
}

// arcaneDenialPolicy casts Arcane Denial first, then the bears, plays lands
// and passes; the draw-up-to ask takes `want` cards then Done.
func arcaneDenialPolicy(m *Matchup, want int) Policy {
	prefer := []string{"Arcane Denial", "Grizzly Bears"}
	picks := 0
	return PolicyFunc(func(p *PickCtx) Canon {
		opts := p.D.Options
		kindOf := func(k CanonKind) []Canon {
			var out []Canon
			for _, o := range opts {
				if o.Kind() == k {
					out = append(out, o)
				}
			}
			return out
		}
		switch p.D.Kind {
		case DTarget:
			picks = 0
			if ts := kindOf(ActTarget); len(ts) > 0 {
				return ts[0]
			}
		case DAttackers, DBlockers:
			if d := kindOf(ActDone); len(d) > 0 {
				return d[0]
			}
		case DChoose:
			if d := kindOf(ActDone); len(d) > 0 {
				if picks >= want {
					picks = 0
					return d[0]
				}
				picks++
				for _, o := range opts {
					if o.Kind() != ActDone {
						return o
					}
				}
			}
		case DPriority:
			picks = 0
			for _, want := range prefer {
				for _, o := range kindOf(ActCast) {
					if m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			if l := kindOf(ActPlayLand); len(l) > 0 {
				return l[0]
			}
		}
		return opts[0]
	})
}

// arcaneDenialDraws counts, in the upkeep that follows the first Arcane
// Denial resolution, the cards each seat draws before the draw step.
func arcaneDenialDraws(evs []events.Event, g *state.Game) (hits [2]int, resolved bool) {
	i := 0
	for ; i < len(evs); i++ {
		e := &evs[i]
		if o := g.Obj(e.Obj); e.Kind == events.Resolve && o != nil && o.Face() != nil && o.Face().Name == "Arcane Denial" {
			resolved = true
			break
		}
	}
	if !resolved {
		return hits, false
	}
	inUpkeep := false
	for ; i < len(evs); i++ {
		e := &evs[i]
		switch {
		case e.Kind == events.StepChange && e.Step == state.StepUpkeep:
			inUpkeep = true
		case e.Kind == events.StepChange && e.Step == state.StepDraw:
			if inUpkeep {
				return hits, true
			}
		case e.Kind == events.Draw && inUpkeep && int(e.Player) < 2:
			hits[e.Player]++
		}
	}
	return hits, true
}

func arcaneDenialPlay(t *testing.T, reg *cards.Registry, r *Repro, set cardset.Set, pol Policy) (Result, [2]int, bool) {
	t.Helper()
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
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000
	}
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	var hits [2]int
	var resolved bool
	rc.Finish = func(g *GorgeSide) { hits, resolved = arcaneDenialDraws(g.E.L.Events, g.E.G) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits, resolved
}

func TestArcaneDenialReproResolves(t *testing.T) {
	reg := synthReg(t)
	for _, c := range arcaneDenialCases {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits, resolved := arcaneDenialPlay(t, reg, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !resolved {
				t.Fatal("Arcane Denial never resolved")
			}
			if hits[0] != c.want || hits[1] != 1 {
				t.Fatalf("upkeep draws = seat 0 %d, seat 1 %d, want %d for the countered spell's controller and 1 for the caster", hits[0], hits[1], c.want)
			}
		})
	}
}

func TestArcaneDenialRecord(t *testing.T) {
	if os.Getenv("ARCANE_DENIAL_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := charmRepeatSet(t, arcaneDenialDecks)
	for _, c := range arcaneDenialCases {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: arcaneDenialDecks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits, resolved := arcaneDenialPlay(t, reg, r, set, arcaneDenialPolicy(m, c.want))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !resolved || hits[0] != c.want || hits[1] != 1 {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatal("no seed resolved Arcane Denial with the upkeep draws")
		})
	}
}
