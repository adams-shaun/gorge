package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// Two cards whose second and third activate-only-once abilities the kernel
// now keeps apart (pfOnceGame plus the mechOnce2/3 arena masks):
//
//   - Draconautics Engineer: two Exhaust abilities.
//   - Loot, the Pathfinder: three Exhaust abilities, the first a mana ability.
//
// The games are recorded by TestOnceAbilityRecord (ONCEABILITY_RECORD=1) and
// replayed by TestStdSetReprosLockstep.
type onceAbilityCase struct {
	effectLongCase
}

func onceAbilityCases() []onceAbilityCase {
	return []onceAbilityCase{
		{
			effectLongCase: effectLongCase{
				name:    "draconautics_two_exhaust_abilities",
				prefer:  []string{"Draconautics Engineer"},
				decks:   [2][]string{append(deckOf(4, "Draconautics Engineer"), deckOf(20, "Mountain")...), deckOf(24, "Plains")},
				surface: "autopay",
				reached: func(g *GorgeSide) bool {
					return pushedAbility(g, "Draconautics Engineer", 0) && pushedAbility(g, "Draconautics Engineer", 1)
				},
			},
		},
		{
			effectLongCase: effectLongCase{
				name:    "loot_three_exhaust_abilities",
				prefer:  []string{"Loot, the Pathfinder"},
				decks:   [2][]string{append(deckOf(1, "Loot, the Pathfinder"), append(deckOf(1, "Island"), append(deckOf(1, "Mountain"), deckOf(20, "Forest")...)...)...), deckOf(30, "Plains")},
				surface: "autopay",
				reached: func(g *GorgeSide) bool {
					return pushedAbility(g, "Loot, the Pathfinder", 1) && pushedAbility(g, "Loot, the Pathfinder", 2)
				},
			},
		},
	}
}

// pushedAbility: the named permanent's activated ability at index idx was
// pushed (AbilityPush's Amount is the printed ability index).
func pushedAbility(g *GorgeSide, name string, idx int32) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.AbilityPush || e.Amount != idx {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// onceAbilityPolicy plays lands, casts the case's card, then takes its
// activations while they are offered; the random tail answers everything else.
func onceAbilityPolicy(m *Matchup, c onceAbilityCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	card := c.prefer[0]
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
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == card {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == card {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestOnceAbilityRecord plays the cases' policies over seeds until one
// reaches every ability, and writes the repro. It runs only with
// ONCEABILITY_RECORD=1; the games it writes are the committed repros.
func TestOnceAbilityRecord(t *testing.T) {
	if os.Getenv("ONCEABILITY_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dft")
	for _, c := range onceAbilityCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: false, Surface: c.surface, Kernel: "std-dft", MaxTurns: 100}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, onceAbilityPolicy(m, c, seed))
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

// TestOnceAbilityReprosReach replays the recorded games: no divergence, every
// scripted action offered, and the per-ability activations reached.
func TestOnceAbilityReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dft")
	for _, c := range onceAbilityCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
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
