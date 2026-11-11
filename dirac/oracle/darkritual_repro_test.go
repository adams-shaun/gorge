package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dark Ritual ("Add {B}{B}{B}", CR 106.4) is a spell, not a mana ability
// (CR 605.1a): it resolves from the stack and puts three black into its
// caster's pool, which a Vampire Nighthawk ({1}{B}{B}) cast in the same step
// then spends. The recorded game casts the Ritual from a Swamp's {B} and the
// Nighthawk from the floating mana; it is replayed in lockstep by
// TestStdSetReprosLockstep and checked for that shape by TestDarkRitualReach.
// TestDarkRitualRecord (DARK_RITUAL_RECORD=1) writes it.
const darkRitualRepro = "dark_ritual_floating_mana_casts_a_creature"

func darkRitualCase() effectLongCase {
	return effectLongCase{
		name:   darkRitualRepro,
		prefer: []string{"Vampire Nighthawk", "Dark Ritual"},
		decks: [2][]string{
			append(append(deckOf(8, "Swamp"), deckOf(8, "Dark Ritual")...), deckOf(8, "Vampire Nighthawk")...),
			deckOf(24, "Plains"),
		},
		surface: "full",
		reached: darkRitualSpent,
	}
}

// darkRitualSpent: a Dark Ritual resolved and its three {B} were added, then a
// Vampire Nighthawk entered the battlefield before the step changed (the pool
// empties as a step ends, CR 500.4), with no second black source tapped in
// between: the Nighthawk's {1}{B}{B} was paid from the Ritual's mana.
func darkRitualSpent(g *GorgeSide) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	resolved, black := false, 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.Resolve && name(e) == "Dark Ritual":
			resolved, black = true, 0
		case !resolved:
		case e.Kind == events.StepChange:
			resolved = false
		case e.Kind == events.ManaAdd && e.Player == 0 && e.Amount > 0 && e.Counter == "B":
			black += int(e.Amount)
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && name(e) == "Vampire Nighthawk":
			if black == 3 {
				return true
			}
			resolved = false
		}
	}
	return false
}

func darkRitualPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand && m.InstanceCard(int(o.Src())) == "Swamp" {
					return o
				}
			}
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestDarkRitualReach replays the recorded game: no divergence, every
// scripted action offered, the Nighthawk paid from the Ritual's three {B}.
func TestDarkRitualReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := darkRitualCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never pays a Nighthawk with a Dark Ritual's mana")
	}
}

// TestDarkRitualRecord plays over seeds until one reaches the shape and writes
// the repro. It runs only with DARK_RITUAL_RECORD=1.
func TestDarkRitualRecord(t *testing.T) {
	if os.Getenv("DARK_RITUAL_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := darkRitualCase()
	for seed := uint64(1); seed <= 2000; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, darkRitualPolicy(m, c, seed))
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
	t.Fatal("no seed paid a Nighthawk with a Dark Ritual's mana")
}
