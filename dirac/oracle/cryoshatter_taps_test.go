package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cryoshatter's "when enchanted creature becomes tapped, destroy it"
// (ValidCard$ Card.AttachedBy, a deferring Taps watcher), played decision by
// decision against gorge: the enchanted creature's attack tap destroys it,
// and a vigilant one attacks untapped and stays. TestStdSetReprosLockstep replays the two repros;
// TestCryoshatterReprosReach requires each recorded game to reach its
// attack, since a lockstep entry alone would pass a script that never did.
//
// The games are recorded by TestCryoshatterRecord (CRYOSHATTER_RECORD=1) on
// the kernel table built from the decks, with effectLongPolicy.
func cryoshatterCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "cryoshatter_enchanted_attacker_tapped_is_destroyed",
			prefer: []string{"Cryoshatter", "Grizzly Bears"},
			decks: [2][]string{
				append(deckOf(8, "Cryoshatter"), deckOf(16, "Island")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return cryoshatterAttack(g, true) },
		},
		{
			name:   "cryoshatter_enchanted_vigilant_attacker_stays",
			prefer: []string{"Cryoshatter", "Brushstrider"},
			decks: [2][]string{
				append(deckOf(8, "Cryoshatter"), deckOf(16, "Island")...),
				append(deckOf(8, "Brushstrider"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return cryoshatterAttack(g, false) },
		},
	}
}

// cryoshatterAttack: a creature enchanted by a Cryoshatter was declared an
// attacker; with destroyed, it was tapped and then left the battlefield
// before the turn ended; without, it was never tapped and stayed on the
// battlefield to the end of that turn.
func cryoshatterAttack(g *GorgeSide, destroyed bool) bool {
	host := map[state.ObjID]state.ObjID{} // Cryoshatter -> enchanted creature
	enchanted := func(id state.ObjID) bool {
		for _, h := range host {
			if h == id {
				return true
			}
		}
		return false
	}
	var attacker state.ObjID
	tapped, left := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.Attach:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Cryoshatter" {
				host[e.Obj] = 0
				if len(e.IDs) == 1 {
					host[e.Obj] = e.IDs[0]
				}
			}
		case attacker == 0 && e.Kind == events.DeclareAttackers:
			for _, id := range e.IDs {
				if enchanted(id) {
					attacker = id
				}
			}
		case attacker != 0 && e.Kind == events.Tap && e.Obj == attacker:
			tapped = true
		case attacker != 0 && e.Kind == events.MoveZone && e.Obj == attacker && e.From == state.ZBattlefield:
			left = true
		case attacker != 0 && e.Kind == events.TurnChange:
			if destroyed && tapped && left || !destroyed && !tapped && !left {
				return true
			}
			attacker, tapped, left = 0, false, false
		}
	}
	return false
}

// TestCryoshatterReprosReach replays the recorded games: no divergence,
// every scripted action offered, the attack reached.
func TestCryoshatterReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range cryoshatterCases() {
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
				t.Fatalf("the recorded game never reaches the attack of %s", c.name)
			}
		})
	}
}

// TestCryoshatterRecord plays the cases' policies over seeds until one
// reaches the attack, and writes the repro. It runs only with
// CRYOSHATTER_RECORD=1; the games it writes are the committed repros.
func TestCryoshatterRecord(t *testing.T) {
	if os.Getenv("CRYOSHATTER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range cryoshatterCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
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
			t.Fatalf("no seed reached the attack of %s", c.name)
		})
	}
}
