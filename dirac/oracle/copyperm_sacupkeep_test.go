package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CopyPermanent AtEOT$ SacrificeUpkeep with AddStaticAbilities$ played
// decision by decision against gorge: Firion, Wild Rose Warrior's "Whenever a
// nontoken Equipment you control enters, create a token that's a copy of it,
// except it has 'This Equipment's equip abilities cost {2} less to activate.'
// Sacrifice that token at the beginning of the next upkeep." The token's equip
// is offered for free beside two tapped-out cards (kop.CpSacAtUpkeep, the
// kernel's dlyUpkeep record, and the copy exception's gated equip reduction),
// and the token is sacrificed at the next upkeep of either player.
// TestStdSetReprosLockstep replays the repro; TestCopypermSacupkeepReprosReach
// requires the recorded game to reach the shape it names.
//
// The game is recorded by TestCopypermSacupkeepRecord
// (COPYPERM_SACUPKEEP_RECORD=1) on the std-fin pool.
const (
	firionCard  = "Firion, Wild Rose Warrior"
	masamune    = "Excalibur II"
	sacupkeepFn = "firion_copy_equip_free_then_sacrificed_at_upkeep"
)

func sacupkeepCase() effectLongCase {
	return effectLongCase{
		name: sacupkeepFn,
		decks: [2][]string{
			append(append(deckOf(6, firionCard), deckOf(8, masamune)...), deckOf(10, "Mountain")...),
			deckOf(24, "Mountain"),
		},
		surface: "autopay",
		reached: firionTokenEquippedThenSacrificed,
	}
}

// firionTokenEquippedThenSacrificed: a token copy of Excalibur II was made,
// equipped to a creature (its equip paid without the printed {2}: the Attach
// is the token's), and left the battlefield at an upkeep step.
func firionTokenEquippedThenSacrificed(g *GorgeSide) bool {
	step := state.Step(0)
	var tok state.ObjID
	equipped, upkeepLeave := false, false
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.StepChange:
			step = e.Step
		case events.CopyToken:
			tok = 0
		case events.Attach:
			if o := g.E.G.Obj(e.Obj); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == masamune {
				equipped = true
				tok = e.Obj
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield && tok != 0 && e.Obj == tok && step == state.StepUpkeep && equipped {
				upkeepLeave = true
			}
		}
	}
	return equipped && upkeepLeave
}

// sacupkeepPolicy casts Firion, then Excalibur II, equips Excalibur II token
// whenever the activation is offered, plays lands, and otherwise picks at
// random from the seed's own stream.
func sacupkeepPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() != ActActivate || p.G == nil {
					continue
				}
				if op, ok := p.G.GorgeOption(o); ok && op.Obj != 0 {
					if ob := p.G.E.G.Obj(op.Obj); ob != nil && ob.IsToken {
						return o
					}
				}
			}
			for _, want := range []string{firionCard, masamune} {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestCopypermSacupkeepReprosReach replays the recorded game: no divergence,
// every scripted action offered, the shape reached.
func TestCopypermSacupkeepReprosReach(t *testing.T) {
	reg := synthReg(t)
	c := sacupkeepCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "std-fin", r)
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, set, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never reaches the shape")
	}
}

// TestCopypermSacupkeepRecord plays the case's policy over seeds until one
// reaches its shape, and writes the repro. It runs only with
// COPYPERM_SACUPKEEP_RECORD=1.
func TestCopypermSacupkeepRecord(t *testing.T) {
	if os.Getenv("COPYPERM_SACUPKEEP_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := sacupkeepCase()
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "synth std-fin"}
		set := reproSet(t, "std-fin", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, sacupkeepPolicy(m, seed))
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
	t.Fatal("no seed reached the shape")
}
