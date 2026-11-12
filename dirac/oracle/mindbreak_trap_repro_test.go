package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mindbreak Trap: "if an opponent cast three or more spells this turn, you
// may pay {0} rather than pay this spell's mana cost; exile any number of
// target spells". kgen lowers the free alternative cost's gate
// (PlayerCountOpponents$ConditionGE3 SpellsCastThisTurn, two-seat: the one
// opponent's cast tally) and the stack-to-exile move over an up-to slot whose
// Count$ValidStack Card bound never binds (lower_stackmove.go,
// value_oppcondition.go). The Trap seat has no lands, so the free cost is
// its only way to cast; the other seat casts 0-cost Ornithopters and, with
// each Ornithopter on the stack, responds with Opt while it has an Island,
// so the Trap, free from the third spell of a turn, meets two spells. Two recorded games:
//
//   - both spells picked, then the done-only ask (the bound never binds);
//   - one spell picked, then Done.
//
// The games are recorded by TestMindbreakTrapRecord (MINDBREAK_TRAP_RECORD=1)
// and replayed by TestMindbreakTrapReprosLockstep.
type mindbreakTrapCase struct {
	effectLongCase
	picks int // targets picked before Done (0: every candidate)
}

func mindbreakTrapCases() []mindbreakTrapCase {
	decks := [2][]string{
		deckOf(30, "Mindbreak Trap"),
		append(append(deckOf(14, "Ornithopter"), deckOf(6, "Opt")...), deckOf(10, "Island")...),
	}
	return []mindbreakTrapCase{
		{
			effectLongCase: effectLongCase{
				name:    "mindbreak_trap_free_exiles_every_spell",
				decks:   decks,
				surface: "autopay",
				reached: func(g *GorgeSide) bool { return trapDepth(g) >= 2 && trapExiled(g) == trapDepth(g) },
			},
		},
		{
			effectLongCase: effectLongCase{
				name:    "mindbreak_trap_free_exiles_one_spell",
				decks:   decks,
				surface: "autopay",
				reached: func(g *GorgeSide) bool { return trapDepth(g) >= 2 && trapExiled(g) == 1 },
			},
			picks: 1,
		},
	}
}

// trapExiled counts the spells other than Mindbreak Trap moved from the stack
// to exile.
func trapExiled(g *GorgeSide) int {
	n := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZExile && sideObjName(g, e.Obj) != "Mindbreak Trap" {
			n++
		}
	}
	return n
}

// trapDepth is the number of spells on the stack as the first Mindbreak Trap
// was put there (no ability uses the stack in these games), or 0.
func trapDepth(g *GorgeSide) int {
	depth := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.PutOnStack && sideObjName(g, e.Obj) == "Mindbreak Trap":
			return depth
		case e.Kind == events.PutOnStack:
			depth++
		case e.Kind == events.MoveZone && e.From == state.ZStack:
			depth--
		}
	}
	return 0
}

// mindbreakTrapPolicy casts every offered Ornithopter, and Opt in response
// to the Ornithopter it just cast; it casts Mindbreak Trap once, when it is first offered; its target ask gets
// c.picks picks (0: every candidate), then Done. The random tail answers the
// rest.
func mindbreakTrapPolicy(m *Matchup, c mindbreakTrapCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	cast, picked, orn := false, 0, false
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DTarget {
			var done, first Canon
			for _, o := range p.D.Options {
				if o.Kind() == ActDone {
					done = o
				} else if o.Kind() == ActTarget && first == 0 {
					first = o
				}
			}
			if done != 0 && (first == 0 || c.picks > 0 && picked >= c.picks) {
				return done
			}
			if first != 0 {
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
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Ornithopter" {
					orn = true
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Opt" && orn {
					orn = false
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Mindbreak Trap" && !cast && p.G != nil && len(p.G.E.G.Stack) >= 2 {
					cast = true
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestMindbreakTrapRecord plays the cases' policies over seeds until one
// reaches its condition, and writes the repro. It runs only with
// MINDBREAK_TRAP_RECORD=1; the games it writes are the committed repros.
func TestMindbreakTrapRecord(t *testing.T) {
	if os.Getenv("MINDBREAK_TRAP_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range mindbreakTrapCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 40; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", MaxTurns: 16}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, mindbreakTrapPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					t.Fatalf("seed %d: the kernel stopped: %q", seed, res.Stall)
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

// TestMindbreakTrapReprosLockstep replays the recorded games: no divergence,
// every scripted action offered, the condition reached.
func TestMindbreakTrapReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range mindbreakTrapCases() {
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
