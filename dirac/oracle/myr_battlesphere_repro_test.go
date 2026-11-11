package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Myr Battlesphere's attack trigger, "you may tap X untapped Myr you control.
// If you do, it gets +X/+0 and deals X damage to the player it's attacking":
// tapXType<X/Myr> as the trigger's own cost. gorge poses one tap election
// (Min 0, Max the untapped Myr; the empty answer is the decline, no yes/no in
// front of it) and binds the count tapped as Count$xPaid; the kernel's pick is
// the literal tapXType's, one Myr per answer, closed by Done
// (dirac/cost_tapxvar.go). Two recorded games:
//
//   - the election answered with two Myr, then Done (X = 2);
//   - the election answered with Done at once (the decline).
//
// The games are recorded by TestMyrBattlesphereRecord (MYR_BATTLESPHERE_RECORD=1)
// and replayed by TestMyrBattlesphereReprosLockstep.
type myrBattlesphereCase struct {
	effectLongCase
	taps int // Myr tapped before Done is answered
}

func myrBattlesphereCases() []myrBattlesphereCase {
	decks := [2][]string{
		append(deckOf(4, "Myr Battlesphere"), deckOf(24, "Forest")...),
		deckOf(30, "Plains"),
	}
	return []myrBattlesphereCase{
		{
			effectLongCase: effectLongCase{
				name:    "myr_battlesphere_taps_two_myr",
				decks:   decks,
				surface: "autopay",
				reached: func(g *GorgeSide) bool { return costTaps(g) >= 2 },
			},
			taps: 2,
		},
		{
			effectLongCase: effectLongCase{
				name:    "myr_battlesphere_declines_the_tap",
				decks:   decks,
				surface: "autopay",
				reached: func(g *GorgeSide) bool { return battlesphereAttacked(g) && costTaps(g) == 0 },
			},
			taps: 0,
		},
	}
}

// battlesphereAttacked: Myr Battlesphere was declared as an attacker.
func battlesphereAttacked(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.DeclareAttackers {
			continue
		}
		for _, id := range e.IDs {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Myr Battlesphere" {
				return true
			}
		}
	}
	return false
}

// costTaps counts the permanents tapped as a cost.
func costTaps(g *GorgeSide) int {
	n := 0
	for i := range g.E.L.Events {
		if e := &g.E.L.Events[i]; e.Kind == events.Tap && e.Text == "tapped as a cost" {
			n++
		}
	}
	return n
}

// myrBattlespherePolicy plays lands, casts Battlesphere and attacks with it;
// a tap election gets c.taps picks, then Done; the random tail answers the rest.
func myrBattlespherePolicy(m *Matchup, c myrBattlesphereCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	picked := 0
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack && m.InstanceCard(int(o.Src())) == "Myr Battlesphere" {
					return o
				}
			}
		}
		if p.D.Kind == DChoose {
			var done, first Canon
			for _, o := range p.D.Options {
				if o.Kind() == ActDone {
					done = o
				} else if first == 0 {
					first = o
				}
			}
			if done != 0 && p.G != nil {
				if picked >= c.taps || first == 0 {
					picked = 0
					return done
				}
				picked++
				return first
			}
		}
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Myr Battlesphere" {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestMyrBattlesphereRecord plays the cases' policies over seeds until one
// reaches its condition, and writes the repro. It runs only with
// MYR_BATTLESPHERE_RECORD=1; the games it writes are the committed repros.
func TestMyrBattlesphereRecord(t *testing.T) {
	if os.Getenv("MYR_BATTLESPHERE_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range myrBattlesphereCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 40; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 40}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, myrBattlespherePolicy(m, c, seed))
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

// TestMyrBattlesphereReprosLockstep replays the recorded games: no
// divergence, every scripted action offered, the condition reached.
func TestMyrBattlesphereReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range myrBattlesphereCases() {
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
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("the kernel stopped: %q", res.Stall)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}
