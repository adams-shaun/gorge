package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two reads of the kernel's per-turn records played against gorge: Faller's
// Faithful's "if it wasn't dealt damage this turn" on a creature its enters
// trigger just destroyed (the record outlives the zone change), and Soulbright
// Seeker's third resolution of its own activated ability in a turn. The games
// are recorded by TestTurnRecReuseRecord (TURNREC_RECORD=1) and replayed by
// TestStdSetReprosLockstep; TestTurnRecReuseReprosReach requires each
// recorded game to reach the card's observable effect.

func turnRecReuseCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "fallers_faithful_destroyed_undamaged_target_draws_two",
			prefer: []string{"Faller's Faithful", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(8, "Faller's Faithful"), deckOf(16, "Swamp")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: fallersDrawsTwo,
		},
		{
			name:   "soulbright_seeker_third_resolution_adds_mana",
			prefer: []string{"Soulbright Seeker"},
			decks: [2][]string{
				append(deckOf(8, "Soulbright Seeker"), deckOf(16, "Mountain")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: seekerThirdResolution,
		},
	}
}

// fallersDrawsTwo: a creature went from the battlefield to the graveyard and,
// before the turn ended, two cards were drawn (the trigger's draw, not the
// turn's).
func fallersDrawsTwo(g *GorgeSide) bool {
	died, draws := false, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			died, draws = false, 0
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
				died = true
			}
		case died && e.Kind == events.Draw:
			draws++
			if draws >= 2 {
				return true
			}
		}
	}
	return false
}

// seekerThirdResolution: three abilities of one Soulbright Seeker resolved in
// one turn.
func seekerThirdResolution(g *GorgeSide) bool {
	n := map[state.ObjID]int{}
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			n = map[state.ObjID]int{}
		case e.Kind == events.Resolve:
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Ability == nil {
				continue
			}
			if s := g.E.G.Obj(o.Source); s != nil && s.Face() != nil && s.Face().Name == "Soulbright Seeker" {
				if n[o.Source]++; n[o.Source] >= 3 {
					return true
				}
			}
		}
	}
	return false
}

// turnRecPolicy is effectLongPolicy, except that an activation of a Seeker's
// ability is taken whenever it is offered.
func turnRecPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && c.prefer[0] == "Soulbright Seeker" {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Soulbright Seeker" {
					return o
				}
			}
		}
		return inner.Pick(p)
	})
}

// TestTurnRecReuseReprosReach replays the recorded games: no divergence,
// every scripted action offered, the effect reached.
func TestTurnRecReuseReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range turnRecReuseCases() {
		t.Run(c.name, func(t *testing.T) {
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
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}

// TestTurnRecReuseRecord plays the cases over seeds until one reaches its
// effect, and writes the repro. It runs only with TURNREC_RECORD=1.
func TestTurnRecReuseRecord(t *testing.T) {
	if os.Getenv("TURNREC_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range turnRecReuseCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, turnRecPolicy(m, c, seed))
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
