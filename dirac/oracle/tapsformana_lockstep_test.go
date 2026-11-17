package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Manabarbs's ordinary (stack, no Static$) Mode$ TapsForMana trigger played
// decision by decision against gorge (dirac/kgen lower_tapsformana.go): any
// player's land tapped for mana is damaged by one (the watched land is a
// deferred payment tier, which the kernel's tapsWatchClass mirrors).
//
// TestTapsForManaLockstep replays the recorded games; TestTapsForManaLockstepRecord
// (PINS_RECORD=1) searches seeds and writes them.
func tapsForManaBarbsCases() []effectLongCase {
	decks := func() [2][]string {
		return [2][]string{
			append(deckOf(8, "Manabarbs"), deckOf(16, "Mountain")...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		}
	}
	return []effectLongCase{
		{name: "manabarbs_damages_tapper_full", prefer: []string{"Manabarbs", "Grizzly Bears"}, decks: decks(), surface: "full", reached: manabarbsDamagedTapper},
	}
}

// manabarbsDamagedTapper: a Manabarbs on the battlefield dealt damage to a
// player after one of that player's lands was tapped, once for each seat.
func manabarbsDamagedTapper(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	hit := map[state.PlayerID]bool{}
	for _, e := range g.E.L.Events {
		if e.Kind == events.DamageProvenance && e.Amount >= 1 && name(e.Obj) == "Manabarbs" {
			hit[e.Player] = true
		}
	}
	return len(hit) > 0
}

func TestTapsForManaLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range tapsForManaBarbsCases() {
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
				t.Fatalf("the recorded game never reaches the effect of %s", c.name)
			}
		})
	}
}

func TestTapsForManaLockstepRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") != "1" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range tapsForManaBarbsCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
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
			t.Fatalf("no seed reached the effect of %s", c.name)
		})
	}
}
