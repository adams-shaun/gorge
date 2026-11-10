package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Stagger lockstep: Lightning, Army of One's first-strike hit marks the
// player it damaged, so until the Effect's controller's next turn the damage
// that player takes is doubled (Duration$ UntilYourNextTurn, kop.ExtDmgMark).
// TestStdSetReprosLockstep replays the recorded game against gorge;
// TestStaggerReproReaches requires it to span the whole mark: Lightning's hit,
// a Dwarven Castle Guard's doubled regular-step hit after it, and then
// Lightning's own hit in a later turn of its controller at its undoubled
// three, since the mark ended before that turn.
//
// The game is recorded by TestStaggerRecord (STAGGER_RECORD=1) on the std-fin
// synth pool.

const staggerRepro = "stagger_doubles_then_expires"

func staggerCase() effectLongCase {
	return effectLongCase{
		name:   staggerRepro,
		prefer: []string{"Lightning, Army of One", "Dwarven Castle Guard"},
		decks: [2][]string{
			append(deckOf(8, "Lightning, Army of One"), append(deckOf(8, "Dwarven Castle Guard"), append(deckOf(12, "Mountain"), deckOf(12, "Plains")...)...)...),
			deckOf(40, "Plains"),
		},
		surface: "autopay",
		reached: staggerSpansExpiry,
	}
}

// staggerSpansExpiry: the second player took three (Lightning's hit), later
// four from a two-power creature (doubled), and after at least two more turn
// changes three again (Lightning's hit once the mark has ended).
func staggerSpansExpiry(g *GorgeSide) bool {
	stage, turns := 0, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange && stage == 2:
			turns++
		case e.Kind == events.Damage && e.Player == 1:
			loss := e.Amount // the damage dealt to the second player
			if loss < 0 {
				loss = -loss
			}
			switch {
			case stage == 0 && loss == 3:
				stage = 1
			case stage == 1 && loss == 4:
				stage = 2
			case stage == 2 && turns >= 2 && loss == 3:
				return true
			}
		}
	}
	return false
}

// staggerPolicy casts the case's cards, plays lands, and declares every
// attacker it is offered; anything else is the seed's random pick.
func staggerPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	base := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		return base.Pick(p)
	})
}

func TestStaggerReproReaches(t *testing.T) {
	reg := synthReg(t)
	c := staggerCase()
	set := cardsettest.Synth(t, "std-fin")
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
		t.Fatal("the recorded game never reaches a doubled hit and the expiry")
	}
}

func TestStaggerRecord(t *testing.T) {
	if os.Getenv("STAGGER_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := staggerCase()
	set := cardsettest.Synth(t, "std-fin")
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
			Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "synth std-fin"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, staggerPolicy(m, c, seed))
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
	t.Fatal("no seed reached the doubled hit and the expiry")
}
