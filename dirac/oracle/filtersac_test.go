package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// Omni-Cheese Pizza's "{1}, {T}, Sacrifice this artifact: Add one mana of any
// color" played decision by decision against gorge: a filter whose activation
// cost sacrifices its source (CR 602.2b, 605.1a). A black spell the lands
// cannot make ({B}) is payable only through the Pizza: a Plains funds its {1},
// the Pizza makes {B} and leaves the battlefield. gorge's planner censuses the
// Pizza as a paid gap, so the cast is a scripted prefix the autopay surface
// drops; the game is recorded on the full surface (station_pipfilter_test.go).
//
// std-tmt holds no Sacrificed-mode watcher, so the watcher is Super Shredder
// ("Whenever another permanent leaves the battlefield, put a +1/+1 counter on
// it"): the Pizza's sacrifice with a Shredder in play is the second Pizza's
// (the first one pays for the Shredder itself). TestFilterSacReprosReach
// requires each recorded game to sacrifice a Pizza with a Shredder in play and
// the Shredder to take the counter, since a lockstep entry alone would pass a
// script that never touched a Pizza. The Pizza's other, activated (non-mana)
// sacrifice ability is never taken by hand.
//
// The game is recorded by TestFilterSacRecord (FILTER_SAC_RECORD=1) on the
// std-tmt pool and replayed by TestStdSetReprosLockstep.
func filterSacCases() []effectLongCase {
	a := append(deckOf(5, "Omni-Cheese Pizza"), append(deckOf(1, "Super Shredder"), append(deckOf(3, "Squirrelanoids"),
		append(deckOf(3, "Dream Beavers"), deckOf(12, "Plains")...)...)...)...)
	return []effectLongCase{
		{
			name:    "filter_sacrifices_its_source_to_pay_a_spell",
			prefer:  []string{"Super Shredder", "Omni-Cheese Pizza", "Squirrelanoids", "Dream Beavers"},
			decks:   [2][]string{a, deckOf(24, "Plains")},
			surface: "full",
			reached: pizzaSacrificedBesideShredder,
		},
	}
}

// pizzaSacrificedBesideShredder: a Pizza was sacrificed (only its mana
// ability does so in this policy) while a Super Shredder was on the
// battlefield, and the Shredder then took a +1/+1 counter.
func pizzaSacrificedBesideShredder(g *GorgeSide) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	evs := g.E.L.Events
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.MoveZone || !events.IsSacrifice(*e) || name(e) != "Omni-Cheese Pizza" {
			continue
		}
		for j := i + 1; j < len(evs); j++ {
			f := &evs[j]
			if f.Kind == events.CounterChange && f.Counter == "P1P1" && f.Amount > 0 && name(f) == "Super Shredder" {
				return true
			}
		}
	}
	return false
}

// filterSacPolicy is effectLongPolicy, except that a Pizza's activated
// abilities are never taken by hand, so every sacrifice is the mana ability's.
func filterSacPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Omni-Cheese Pizza" {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// TestFilterSacReprosReach replays the recorded game: no divergence, every
// scripted action offered, a Pizza sacrificed beside a Shredder that counts it.
func TestFilterSacReprosReach(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range filterSacCases() {
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
				t.Fatal("the recorded game never sacrifices a Pizza beside a Super Shredder that takes the counter")
			}
		})
	}
}

// TestFilterSacRecord plays the case's policy over seeds until one reaches
// the sacrifice, and writes the repro. It runs only with FILTER_SAC_RECORD=1;
// the game it writes is the committed repro.
func TestFilterSacRecord(t *testing.T) {
	if os.Getenv("FILTER_SAC_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range filterSacCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-tmt"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, filterSacPolicy(m, c, seed))
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
