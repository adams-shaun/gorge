package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// Twitching Doll's "{T}: Add one mana of any color. Put a nest counter on
// this" played decision by decision against gorge (CR 605.3b: the rider
// resolves with the mana). gorge's planner defers the source, so a red cast
// only its mana funds needs a scripted prefix, which the autopay surface drops
// (Config.GorgeAutopaySurface); the game is recorded on the full surface,
// where the payment taps it. TestStdSetReprosLockstep replays the repro;
// TestManaRiderReprosReach requires each recorded game to put a nest counter
// on a Doll, since a lockstep entry alone would pass a script that never
// tapped one.
//
// The games are recorded by TestManaRiderRecord (MANA_RIDER_RECORD=1) on the
// std-dsk pool with manaRiderPolicy: both seats prefer the case's cards and
// otherwise pick at random from the seed's stream, never sacrificing a Doll.
func manaRiderCases() []effectLongCase {
	a := append(deckOf(8, "Twitching Doll"), append(deckOf(4, "Clockwork Percussionist"), append(deckOf(4, "Ragged Playmate"), deckOf(12, "Forest")...)...)...)
	b := append(deckOf(4, "Piggy Bank"), deckOf(20, "Mountain")...)
	return []effectLongCase{
		{
			name:    "twitching_doll_mana_puts_a_nest_counter",
			prefer:  []string{"Clockwork Percussionist", "Ragged Playmate", "Twitching Doll", "Piggy Bank"},
			decks:   [2][]string{a, b},
			surface: "full",
			reached: nestCounterOnDoll,
		},
	}
}

// nestCounterOnDoll: a nest counter was put on a Twitching Doll (only its
// mana ability's rider puts one).
func nestCounterOnDoll(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.CounterChange && e.Counter == "NEST" && e.Amount > 0 {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Twitching Doll" {
				return true
			}
		}
	}
	return false
}

// manaRiderPolicy is effectLongPolicy, except that a Twitching Doll's
// sacrifice ability is never activated, so the Dolls stay to make mana.
func manaRiderPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Twitching Doll" {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestManaRiderReprosReach replays the recorded games: no divergence, every
// scripted action offered, a nest counter put.
func TestManaRiderReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	for _, c := range manaRiderCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatal("the recorded game never puts a nest counter on a Twitching Doll")
			}
		})
	}
}

// TestManaRiderRecord plays the cases' policies over seeds until one reaches
// the rider, and writes the repro. It runs only with MANA_RIDER_RECORD=1; the
// games it writes are the committed repros.
func TestManaRiderRecord(t *testing.T) {
	if os.Getenv("MANA_RIDER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	for _, c := range manaRiderCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-dsk"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, manaRiderPolicy(m, c, seed))
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
