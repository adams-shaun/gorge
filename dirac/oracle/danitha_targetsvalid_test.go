package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// Danitha, Spear of Agony's SpellCast TargetsValid$ "Opponent,Creature.OppCtrl"
// is a zone-free object alternative: a Counterspell aimed at an opponent's
// creature spell fires it, and the +1/+1 counter on Danitha is the record that
// the trigger resolved. Both decks hold no other spell that could set it off
// (seat 0's only other spell is Counterspell itself, which can never target a
// player), so the counter is the any-zone read in a real game.
//
// TestStdSetReprosLockstep replays the recorded game; the record test below
// (DANITHA_RECORD=1) writes it. TestDanithaTargetsValidReproReach requires the
// recorded game to reach the counter, since a lockstep entry alone would pass a
// script that never cast Danitha or never countered a creature spell.
func danithaTargetsValidCase() effectLongCase {
	return effectLongCase{
		name:   "danitha_counters_a_creature_spell",
		prefer: []string{"Danitha, Spear of Agony", "Counterspell"},
		decks: [2][]string{
			append(deckOf(4, "Danitha, Spear of Agony"), append(deckOf(4, "Counterspell"), append(deckOf(8, "Island"), deckOf(8, "Swamp")...)...)...),
			append(deckOf(8, "Grizzly Bears"), deckOf(12, "Forest")...),
		},
		surface: "autopay",
		reached: danithaCounteredACreatureSpell,
	}
}

// danithaCounteredACreatureSpell: a +1/+1 counter was put on Danitha.
func danithaCounteredACreatureSpell(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	for _, e := range g.E.L.Events {
		if e.Kind != events.CounterChange || e.Amount <= 0 || !strings.Contains(e.Counter, "P1P1") {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Danitha, Spear of Agony" {
			return true
		}
	}
	return false
}

// TestDanithaTargetsValidReproReach replays the recorded game: no divergence,
// every scripted action offered, and Danitha's counter reached.
func TestDanithaTargetsValidReproReach(t *testing.T) {
	reg := synthReg(t)
	c := danithaTargetsValidCase()
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
		t.Fatalf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatal("the recorded game never put a counter on Danitha")
	}
}

// TestDanithaTargetsValidRecord plays the case's policy over seeds until one
// reaches the counter, and writes the repro. It runs only with
// DANITHA_RECORD=1.
func TestDanithaTargetsValidRecord(t *testing.T) {
	if os.Getenv("DANITHA_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := danithaTargetsValidCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
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
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatalf("no seed reached Danitha's counter")
}
