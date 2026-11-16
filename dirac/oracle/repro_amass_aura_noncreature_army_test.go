package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Amass needs an Army CREATURE you control (CR 701.47a). One with the Stars
// ("enchanted permanent is an enchantment and loses all other card types")
// leaves an Orc Army token a noncreature enchantment, and the creature
// subtypes go with the type (CR 205.1a), so the next amass creates a fresh
// Army token. gorge once matched "Creature.Army" without the layer tables and
// put the counters on the enchanted token (pair 1393, projection NInst).
//
// The deck of the first player is Books of Mazarbul (a saga amassing Orcs); the
// second player's is One with the Stars. The game is recorded by
// TestReproAmassAuraNoncreatureArmyRecord (PINS_RECORD=1) and replayed by
// TestReproAmassAuraNoncreatureArmyReach, which requires an Aura on a token
// followed by an amass. The recording is a custom-pool fixture under
// testdata/ (not testdata/repro/): only this test reads it, so it is not one
// of the standard-pool recordings the repro globs replay.
func amassAuraNoncreatureArmyCase() effectLongCase {
	return effectLongCase{
		name:   "repro_amass_aura_noncreature_army_makes_fresh_token",
		prefer: []string{"One with the Stars", "Book of Mazarbul"},
		decks: [2][]string{
			append(deckOf(8, "Book of Mazarbul"), deckOf(16, "Mountain")...),
			append(deckOf(8, "One with the Stars"), deckOf(16, "Island")...),
		},
		surface: "full",
		reached: amassAfterAuraOnToken,
	}
}

// amassAfterAuraOnToken reads the finished gorge log: One with the Stars is
// attached to a token, and an amass lands afterwards (a new token or counters
// on the bearer).
func amassAfterAuraOnToken(g *GorgeSide) bool {
	var bearer state.ObjID
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.Attach:
			o := g.E.G.Obj(e.Obj)
			if o != nil && o.Face() != nil && o.Face().Name == "One with the Stars" && len(e.IDs) == 1 {
				if b := g.E.G.Obj(e.IDs[0]); b != nil && b.IsToken {
					bearer = e.IDs[0]
				}
			}
		case events.TokenCreate:
			if bearer != 0 {
				return true
			}
		case events.CounterChange:
			if bearer != 0 && e.Obj == bearer {
				return true
			}
		}
	}
	return false
}

// TestReproAmassAuraNoncreatureArmyReach replays the recorded game: no
// divergence, every scripted action offered, and an amass after the Aura.
func TestReproAmassAuraNoncreatureArmyReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := amassAuraNoncreatureArmyCase()
	r, err := ReadRepro(filepath.Join("testdata", c.name+".repro.json"))
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
		t.Fatal("the recorded game never amasses after One with the Stars enchants a token")
	}
}

// TestReproAmassAuraNoncreatureArmyRecord plays seeds until one reaches the
// amass and writes the repro. It runs only with PINS_RECORD=1.
func TestReproAmassAuraNoncreatureArmyRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := amassAuraNoncreatureArmyCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"},
			Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatal("no seed amassed after One with the Stars enchanted a token")
}
