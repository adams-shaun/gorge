package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Alchemist's Vial's "{1}, {T}, Sacrifice: target creature can't attack or
// block this turn" (Pump KW$ HIDDEN CARDNAME can't attack or block.): the
// kernel's until-end-of-turn KwCantBlock grant beside an override record's
// CantAttack (the Animate HiddenKeywords$ pair). Two recorded games, each in
// seat 0's first main phase with its Grizzly Bears able to attack:
//
//   - the Vial targets seat 0's own Bears, which then cannot be declared as an
//     attacker (the attack half);
//   - the Vial targets seat 1's Runeclaw Bear, which then cannot block the
//     attacking Bears (the block half).
//
// The games are recorded by TestAlchemistsVialRecord (ALCHEMISTS_VIAL_RECORD=1)
// and replayed by TestAlchemistsVialReprosLockstep.
type alchemistsVialCase struct {
	effectLongCase
	target string // the creature the Vial is aimed at
}

func alchemistsVialCases() []alchemistsVialCase {
	decks := [2][]string{
		append(append(deckOf(8, "Alchemist's Vial"), deckOf(10, "Grizzly Bears")...), deckOf(22, "Forest")...),
		append(deckOf(14, "Runeclaw Bear"), deckOf(26, "Forest")...),
	}
	mk := func(name, target string) alchemistsVialCase {
		return alchemistsVialCase{
			effectLongCase: effectLongCase{name: name, decks: decks, surface: "autopay", reached: vialSacrificed},
			target:         target,
		}
	}
	return []alchemistsVialCase{
		mk("alchemists_vial_stops_own_attacker", "Grizzly Bears"),
		mk("alchemists_vial_stops_a_blocker", "Runeclaw Bear"),
	}
}

// vialSacrificed: Alchemist's Vial was sacrificed as a cost.
func vialSacrificed(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if !events.IsSacrifice(*e) {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Alchemist's Vial" {
			return true
		}
	}
	return false
}

// vialNamed counts the battlefield permanents of seat named n.
func vialNamed(g *GorgeSide, seat int, n string) int {
	c := 0
	for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(seat)) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == n {
			c++
		}
	}
	return c
}

func vialTurns(g *GorgeSide) int {
	n := 0
	for i := range g.E.L.Events {
		if g.E.L.Events[i].Kind == events.TurnChange {
			n++
		}
	}
	return n
}

// alchemistsVialPolicy plays lands, casts the creatures and Vials, attacks
// with every attacker offered, activates seat 0's Vial in its main phase once
// its Bears have been on the battlefield since an earlier turn (and seat 1's
// Runeclaw Bear is out), and aims it at c.target; the
// random tail answers the rest.
func alchemistsVialPolicy(m *Matchup, c alchemistsVialCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	bearsTurn := -1
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DAttackers {
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		if p.D.Kind == DTarget && p.G != nil {
			for _, a := range p.D.Options {
				if o, ok := p.G.GorgeOption(a); ok && a.Kind() == ActTarget {
					if ob := p.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == c.target {
						return a
					}
				}
			}
		}
		if p.D.Kind == DPriority && p.G != nil {
			g := p.G
			if bearsTurn < 0 && vialNamed(g, 0, "Grizzly Bears") > 0 {
				bearsTurn = vialTurns(g)
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			if p.D.Player == 0 && g.E.G.Active == 0 && g.E.G.Step == state.StepMain1 && bearsTurn >= 0 && vialTurns(g) > bearsTurn &&
				vialNamed(g, 1, "Runeclaw Bear") > 0 {
				for _, o := range p.D.Options {
					if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Alchemist's Vial" {
						return o
					}
				}
			}
			for _, want := range []string{"Grizzly Bears", "Runeclaw Bear", "Alchemist's Vial"} {
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

// TestAlchemistsVialRecord plays the cases' policies over seeds until one
// reaches its condition, and writes the repro. It runs only with
// ALCHEMISTS_VIAL_RECORD=1; the games it writes are the committed repros.
func TestAlchemistsVialRecord(t *testing.T) {
	if os.Getenv("ALCHEMISTS_VIAL_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range alchemistsVialCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 200; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 40}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, alchemistsVialPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v %s last=%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.LastSourced)
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

// TestAlchemistsVialReprosLockstep replays the recorded games: no divergence,
// every scripted action offered, the Vial sacrificed.
func TestAlchemistsVialReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range alchemistsVialCases() {
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
