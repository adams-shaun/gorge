package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Relentless Assault (DB$ AddPhase | ExtraPhase$ Combat | FollowedBy$ Main2,
// behind ConditionPhases$ Main1,Main2): "After this main phase, there is an
// additional combat phase followed by an additional main phase" (CR 500.8).
// Cast in the first main phase the extra combat and extra main come before the
// ordinary combat; cast in the second the extra main is the last before the end
// step. The two repros are the recorded games; the replay requires the grant
// and the walk that follows it, not only a clean lockstep.

const relentlessAssaultName = "Relentless Assault"

type relentlessAssaultCase struct {
	repro string
	main  state.Step // the main phase the spell is cast in
	steps []state.Step
	// extraMainCast: a Raging Goblin is cast in the additional main phase,
	// before the ordinary combat it then attacks in. That priority decision
	// exists only if the kernel walks into that main phase, so the lockstep
	// compares the walk there, not only the extra combat.
	extraMainCast bool
}

var relentlessAssaultCases = []relentlessAssaultCase{
	// Main1, extra combat, extra main, the ordinary combat, Main2, End
	{"relentless_assault_cast_precombat_runs_before_the_ordinary_combat", state.StepMain1,
		[]state.Step{state.StepBeginCombat, state.StepEndCombat, state.StepMain2, state.StepBeginCombat, state.StepEndCombat, state.StepMain2, state.StepEnd}, true},
	// Main2, extra combat, extra main, End
	{"relentless_assault_cast_postcombat_adds_a_last_main_phase", state.StepMain2,
		[]state.Step{state.StepBeginCombat, state.StepEndCombat, state.StepMain2, state.StepEnd}, false},
}

func relentlessAssaultDecks() [2][]string {
	return [2][]string{
		append(append(deckOf(6, relentlessAssaultName), deckOf(6, "Raging Goblin")...), deckOf(28, "Mountain")...),
		deckOf(40, "Forest"),
	}
}

// relentlessAssaultReached: the card's grant was made in the case's main phase
// (the extra combat, FollowedBy$ the second main phase), and the steps that
// followed it in that turn, restricted to the combat ends, the second main
// phase and the end step, are exactly want. With extraMainCast a Raging Goblin
// was also put on the stack in the additional main phase (the third of those
// steps). Any of the game's grants may be the one that does.
func relentlessAssaultReached(g *GorgeSide, c relentlessAssaultCase) bool {
	ev := g.E.L.Events
	for i, e := range ev {
		if e.Kind != events.ExtraPhase || e.Amount <= 0 || e.Step != c.main || len(e.IDs) != 2 ||
			state.Step(e.IDs[0]) != state.StepBeginCombat || state.Step(e.IDs[1]) != state.StepMain2 {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Face().Name != relentlessAssaultName {
			continue
		}
		var got []state.Step
		cast := false
		for _, s := range ev[i+1:] {
			if s.Kind == events.PutOnStack && len(got) == 3 && isGoblin(g, s.Obj) {
				cast = true
			}
			if s.Kind != events.StepChange {
				continue
			}
			switch s.Step {
			case state.StepBeginCombat, state.StepEndCombat, state.StepMain2, state.StepEnd:
				got = append(got, s.Step)
			}
			if s.Step == state.StepEnd {
				break
			}
		}
		if len(got) >= len(c.steps) && (cast || !c.extraMainCast) && slices.Equal(got[:len(c.steps)], c.steps) {
			return true
		}
	}
	return false
}

// relentlessAssaultPolicy plays a land, casts Relentless Assault in the
// case's main phase, else a Raging Goblin in the first main phase (with
// extraMainCast only the first two, the others in a second main phase of a
// turn with a grant), and attacks with every creature.
func relentlessAssaultPolicy(m *Matchup, c relentlessAssaultCase) Policy {
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DPriority:
			if p.G != nil {
				step := p.G.E.G.Step
				goblin := step == state.StepMain1 && (!c.extraMainCast || goblinsCast(p.G) < 2) ||
					c.extraMainCast && step == state.StepMain2 && grantedThisTurn(p.G)
				for _, want := range [...]bool{true, false} { // Relentless Assault first
					for _, o := range p.D.Options {
						if o.Kind() != ActCast {
							continue
						}
						name := m.InstanceCard(int(o.Src()))
						if want && name == relentlessAssaultName && step == c.main || !want && name == "Raging Goblin" && goblin {
							return o
						}
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPass {
					return o
				}
			}
		case DAttackers:
			for _, o := range p.D.Options {
				if o.Kind() == ActAttack {
					return o
				}
			}
		}
		return p.D.Options[0]
	})
}

func isGoblin(g *GorgeSide, id state.ObjID) bool {
	o := g.E.G.Obj(id)
	return o != nil && o.Face() != nil && o.Face().Name == "Raging Goblin"
}

// goblinsCast counts the Raging Goblins cast this game; with extraMainCast
// the policy casts two in first main phases and keeps the rest in hand for an
// additional main phase.
func goblinsCast(g *GorgeSide) int {
	n := 0
	for _, e := range g.E.L.Events {
		if e.Kind == events.PutOnStack && isGoblin(g, e.Obj) {
			n++
		}
	}
	return n
}

// grantedThisTurn: an extra phase was granted this turn (the second main
// phase is then the additional one or follows it), so the policy keeps a
// Raging Goblin for it.
func grantedThisTurn(g *GorgeSide) bool {
	ev := g.E.L.Events
	for i := len(ev) - 1; i >= 0 && ev[i].Kind != events.TurnChange; i-- {
		if ev[i].Kind == events.ExtraPhase && ev[i].Amount > 0 {
			return true
		}
	}
	return false
}

// TestRelentlessAssaultReproRecord plays the policy until a game reaches each
// case's turn shape and writes the repros. It runs only with
// RELENTLESS_ASSAULT_RECORD=1.
func TestRelentlessAssaultReproRecord(t *testing.T) {
	if os.Getenv("RELENTLESS_ASSAULT_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range relentlessAssaultCases {
		recorded := false
		for seed := uint64(1); seed <= 100 && !recorded; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{c.repro + "_0", c.repro + "_1"}, Cards: relentlessAssaultDecks(),
				Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			set := reproSet(t, "custom", r)
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: true, PassElision: true}
			k, err := SetSide(set, tbl, cfg)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			reached := false
			rc := RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20}
			rc.Finish = func(g *GorgeSide) { reached = relentlessAssaultReached(g, c) }
			res := Lockstep(m, r.Seed, k, relentlessAssaultPolicy(m, c), rc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("%s seed %d: divergence at %d: %s %s %v", c.repro, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if !reached {
				continue
			}
			r.setActions(res.Actions)
			if err := r.Write(filepath.Join("testdata", "repro", c.repro+".repro.json")); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: seed %d, %d decisions", c.repro, seed, len(res.Actions))
			recorded = true
		}
		if !recorded {
			t.Fatalf("%s: no seed reached the shape", c.repro)
		}
	}
}

// TestRelentlessAssaultReproLockstep replays each recorded game on its own
// table: no divergence, nothing unmodelled, and the grant and the walk after
// it are the ones the case names (so the script did cast the spell there).
func TestRelentlessAssaultReproLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range relentlessAssaultCases {
		t.Run(c.repro, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.repro+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			set := reproSet(t, "custom", r)
			tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions = len(r.Actions) + 200
			reached := false
			rc.Finish = func(g *GorgeSide) { reached = relentlessAssaultReached(g, c) }
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			script := &ScriptPolicy{Actions: r.Actions}
			res := Lockstep(m, r.Seed, k, script, rc)
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.StallUnmodelled {
				t.Fatalf("dirac stopped at a rule it does not model: %s", res.Stall)
			}
			if !reached {
				t.Fatalf("the recorded game never made the grant in %s and walked %v", c.main, c.steps)
			}
		})
	}
}
