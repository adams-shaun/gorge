package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cabal Ritual (Amount$ X, Count$Threshold.5.3: {B}{B}{B}, or five with seven
// or more cards in its controller's graveyard, CR 702.24a) and Rite of Flame
// ({R}{R}, then {R} per card named Rite of Flame in each graveyard) chain in
// one step: each ritual's mana pays the next, and the graveyard they fill
// moves the amount. The recorded games are replayed in lockstep by
// TestStdSetReprosLockstep and checked for their shape by TestRitualReach.
// TestRitualRecord (RITUAL_RECORD=1) writes them.
type ritualCase struct {
	effectLongCase
	land   string
	ritual string
	colour string
	// amounts are the ManaAdd amounts of the colour, in order, a resolving
	// ritual must have produced somewhere in the game
	amounts []int32
	// mana is the pool slot of colour; held is the amount of it that must sit
	// in the caster's pool at a decision both sides posed. The ritual's ManaAdd
	// events are gorge's alone, so only a floating pool the projections are
	// compared at tells that dirac added the same amount.
	mana int
	held int32
}

func ritualCases() []ritualCase {
	mk := func(name, land, ritual, colour string, mana int, held int32, amounts ...int32) ritualCase {
		return ritualCase{
			effectLongCase: effectLongCase{
				name:    name,
				prefer:  []string{ritual},
				decks:   [2][]string{append(deckOf(8, land), deckOf(16, ritual)...), deckOf(24, "Plains")},
				surface: "full",
			},
			land: land, ritual: ritual, colour: colour, amounts: amounts, mana: mana, held: held,
		}
	}
	return []ritualCase{
		mk("cabal_ritual_threshold_adds_five", "Swamp", "Cabal Ritual", "B", state.MB, 5, 3, 5),
		mk("rite_of_flame_counts_graveyard_copies", "Mountain", "Rite of Flame", "R", state.MR, 2, 2, 1),
	}
}

// reached: a resolved ritual's ManaAdd events carry every wanted amount.
func (c ritualCase) reach(g *GorgeSide) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	seen := make(map[int32]bool)
	resolving := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.Resolve:
			resolving = name(e) == c.ritual
		case e.Kind == events.StepChange:
			resolving = false
		case resolving && e.Kind == events.ManaAdd && e.Player == 0 && e.Amount > 0 && e.Counter == c.colour:
			seen[int32(e.Amount)] = true
		}
	}
	for _, a := range c.amounts {
		if !seen[a] {
			return false
		}
	}
	return true
}

func (c ritualCase) policy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			// the ritual first, while the land drop is unspent: the decision
			// after it resolves still offers the land, so it is posed (not an
			// elided pass) with the ritual's mana floating
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.ritual {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand && m.InstanceCard(int(o.Src())) == c.land {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// play runs the lockstep game and reports whether gorge's log reaches every
// wanted amount and whether the caster's pool held c.held of the colour at a
// decision both sides posed, where the projections (the pool among them) were
// compared and agreed.
func (c ritualCase) play(t *testing.T, reg *cards.Registry, r *Repro, set cardset.Set, pol Policy) (res Result, logged, held bool) {
	t.Helper()
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
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	} else {
		rc.MaxDecisions = 2000
	}
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		if ref.E.G.Players[0].Pool[c.mana] == c.held {
			held = true
		}
		return nil
	}
	rc.Finish = func(g *GorgeSide) { logged = c.reach(g) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), logged, held
}

// TestRitualReach replays each recorded game: no divergence, every scripted
// action offered, and the ritual produced each wanted amount.
func TestRitualReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range ritualCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached, held := c.play(t, reg, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never has %s add %v %s", c.ritual, c.amounts, c.colour)
			}
			if !held {
				t.Fatalf("no decision both sides posed has %d %s floating: dirac's amount for %s was never compared", c.held, c.colour, c.ritual)
			}
		})
	}
}

// TestRitualRecord plays over seeds until one reaches the shape and writes
// the repro. It runs only with RITUAL_RECORD=1.
func TestRitualRecord(t *testing.T) {
	if os.Getenv("RITUAL_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range ritualCases() {
		done := false
		for seed := uint64(1); seed <= 200 && !done; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
				Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
			set := reproSet(t, "custom", r)
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			res, logged, held := c.play(t, reg, r, set, c.policy(m, seed))
			if res.Div != nil {
				t.Fatalf("%s seed %d: divergence at %d: %s %s %v", c.name, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if !logged || !held {
				continue
			}
			r.setActions(res.Actions)
			if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s seed %d: %d decisions", c.name, seed, len(res.Actions))
			done = true
		}
		if !done {
			t.Fatalf("%s: no seed reached the shape", c.name)
		}
	}
}
