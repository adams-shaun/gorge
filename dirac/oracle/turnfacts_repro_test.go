package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two games on the per-player this-turn facts the kernel keeps (a coin flip,
// an Exhaust activation; dirac/static_turnbits.go):
//
//   - Edgar, King of Figaro beside Viashino Sandswimmer: the first flip each turn is
//     heads with no rng draw, a second flip in the turn draws it.
//   - Elvish Refueler activating its own Exhaust ability on two turns: the
//     Activations static waives the used once record once per turn.
//
// The games are recorded by TestTurnFactsRecord (TURNFACTS_RECORD=1) and
// replayed by TestStdSetReprosLockstep.
type turnFactsCase struct {
	effectLongCase
	cast []string // cast in this order of preference
	act  string   // the card whose abilities are activated
}

func turnFactsCases() []turnFactsCase {
	return []turnFactsCase{
		{
			effectLongCase: effectLongCase{
				name:    "edgar_first_flip_each_turn_is_heads",
				decks:   [2][]string{deckOf(4, "Edgar, King of Figaro", deckOf(8, "Viashino Sandswimmer", append(deckOf(10, "Island"), deckOf(10, "Mountain")...)...)...), deckOf(30, "Plains")},
				surface: "autopay",
				reached: edgarTwoFlipTurn,
			},
			cast: []string{"Viashino Sandswimmer", "Edgar, King of Figaro"},
			act:  "Viashino Sandswimmer",
		},
		{
			effectLongCase: effectLongCase{
				name:    "refueler_waives_its_used_exhaust_on_a_later_turn",
				decks:   [2][]string{append(deckOf(4, "Elvish Refueler"), deckOf(20, "Forest")...), deckOf(24, "Plains")},
				surface: "autopay",
				reached: func(g *GorgeSide) bool { return pushCount(g, "Elvish Refueler") >= 2 },
			},
			cast: []string{"Elvish Refueler"},
			act:  "Elvish Refueler",
		},
	}
}

// pushCount: how many abilities of the named permanent were pushed.
func pushCount(g *GorgeSide, name string) int {
	n := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.AbilityPush {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}

// edgarTwoFlipTurn: Edgar entered the battlefield and some turn after it
// holds two coin flips (the first forced heads, the second drawn).
func edgarTwoFlipTurn(g *GorgeSide) bool {
	in, flips := false, 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Edgar, King of Figaro" {
				in = true
			}
		case e.Kind == events.TurnChange:
			flips = 0
		case in && e.Kind == events.Note && len(e.Text) >= len(events.FlipNotePrefix) && e.Text[:len(events.FlipNotePrefix)] == events.FlipNotePrefix:
			if flips++; flips >= 2 {
				return true
			}
		}
	}
	return false
}

// turnFactsPolicy plays lands, casts the case's cards, then activates the
// case's card while it is offered; the random tail answers everything else.
func turnFactsPolicy(m *Matchup, c turnFactsCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose && len(p.D.Options) > 0 {
			return p.D.Options[0]
		}
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, card := range c.cast {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == card {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.act {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestTurnFactsRecord plays the cases' policies over seeds until one reaches
// its condition, and writes the repro. It runs only with TURNFACTS_RECORD=1;
// the games it writes are the committed repros.
func TestTurnFactsRecord(t *testing.T) {
	if os.Getenv("TURNFACTS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range turnFactsCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 300; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 100}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, turnFactsPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}

// TestTurnFactsReprosReach replays the recorded games: no divergence, every
// scripted action offered, the condition reached.
func TestTurnFactsReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range turnFactsCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, "custom", r)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}
