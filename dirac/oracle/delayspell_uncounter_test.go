package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "The next spell you cast this turn can't be countered" (Mistrise Village,
// Theorist's Proxy): gorge's Static$ True delayed SpellCast resolves its
// Effect at the cast, and a counter of that spell is stopped (CR 101.2). The
// recorded games play on the Standard pools; TestStdSetReprosLockstep
// replays them, and TestDelaySpellUncounterReach requires each to reach a
// stopped counter and to diverge with the kernel's record ignored.
// The games are recorded by TestDelaySpellUncounterRecord
// (DELAYSPELL_UNCOUNTER_RECORD=1).
type uncounterCase struct {
	effectLongCase
	pool      string
	registrar string // the card whose activation registers
	counter   string // the opponent's counterspell
}

func uncounterCases() []uncounterCase {
	return []uncounterCase{
		{
			effectLongCase: effectLongCase{
				name:  "theorists_proxy_next_spell_survives_countersculpt",
				decks: [2][]string{deckOf(10, "Theorist's Proxy", deckOf(14, "Island")...), deckOf(10, "Countersculpt", deckOf(14, "Island")...)},
				// both seats prefer their spells; the case's policy adds the
				// activation and the counter of the opponent's spell
				prefer:  []string{"Theorist's Proxy"},
				surface: "autopay",
				reached: counterStopped,
			},
			pool: "std-fra", registrar: "Theorist's Proxy", counter: "Countersculpt",
		},
		{
			effectLongCase: effectLongCase{
				name: "mistrise_village_next_spell_survives_dispelling_exhale",
				decks: [2][]string{
					deckOf(8, "Mistrise Village", deckOf(8, "Highspire Bell-Ringer", deckOf(4, "Forest", deckOf(8, "Island")...)...)...),
					deckOf(10, "Dispelling Exhale", deckOf(14, "Island")...),
				},
				prefer:  []string{"Highspire Bell-Ringer"},
				surface: "full", // the autopay planner never prices the Exhale's optional behold,
				reached: counterStopped,
			},
			pool: "std-tdm", registrar: "Mistrise Village", counter: "Dispelling Exhale",
		},
	}
}

// counterStopped: gorge's counter of a spell was stopped by a replacement
// (counterStopped's "counter prevented" Note on a stack object).
func counterStopped(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.Note && e.Text == "counter prevented" {
			return true
		}
	}
	return false
}

// uncounterPolicy: with no registration of its own live, the registrar's
// activation first; then a spell the case prefers; the counterspell while an
// opponent's spell is on the stack; a land; else the seed's random pick.
func uncounterPolicy(m *Matchup, c uncounterCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G == nil || p.G.fr == nil || p.G.fr.d == nil {
			return rnd.Pick(p)
		}
		g := p.G.E.G
		actor := state.PlayerID(p.G.fr.d.Player)
		live := false
		for i := range g.Delayed {
			if dt := &g.Delayed[i]; dt.EventMode == "SpellCast" && dt.Controller == actor {
				live = true
			}
		}
		oppSpell := false
		for _, id := range g.Stack {
			if o := g.Obj(id); o != nil && o.Controller != actor && o.Zone == state.ZStack {
				oppSpell = true
			}
		}
		pick := func(kind CanonKind, name string) (Canon, bool) {
			for _, o := range p.D.Options {
				if o.Kind() == kind && m.InstanceCard(int(o.Src())) == name {
					return o, true
				}
			}
			var none Canon
			return none, false
		}
		if oppSpell {
			if o, ok := pick(ActCast, c.counter); ok {
				return o
			}
		}
		if !live {
			if o, ok := pick(ActActivate, c.registrar); ok {
				return o
			}
		}
		if live || len(g.Stack) == 0 {
			for _, want := range c.prefer {
				if o, ok := pick(ActCast, want); ok {
					return o
				}
			}
		}
		for _, o := range p.D.Options {
			if o.Kind() == ActPlayLand {
				return o
			}
		}
		return rnd.Pick(p)
	})
}

// TestDelaySpellUncounterReach replays the recorded games: no divergence,
// every scripted action offered, a counter stopped; with the kernel's fired
// records ignored, the same script diverges.
func TestDelaySpellUncounterReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range uncounterCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := cardsettest.Synth(t, c.pool)
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never stops a counter")
			}
			dirac.DebugUncounterDelayOff = true
			defer func() { dirac.DebugUncounterDelayOff = false }()
			res, _ = effectLongPlay(t, reg, c.effectLongCase, r, set, &ScriptPolicy{Actions: r.Actions})
			if res.Div == nil {
				t.Fatal("ignoring the fired records did not make the recorded game diverge")
			}
		})
	}
}

// TestDelaySpellUncounterRecord plays the cases' policies over seeds until
// one stops a counter, and writes the repro. It runs only with
// DELAYSPELL_UNCOUNTER_RECORD=1; the games it writes are the committed
// repros.
func TestDelaySpellUncounterRecord(t *testing.T) {
	if os.Getenv("DELAYSPELL_UNCOUNTER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range uncounterCases() {
		t.Run(c.name, func(t *testing.T) {
			set := cardsettest.Synth(t, c.pool)
			for seed := uint64(1); seed <= 300; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth " + c.pool}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, uncounterPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
				}
				if res.Stall != "" {
					t.Logf("seed %d: stalled: %s", seed, res.Stall)
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
