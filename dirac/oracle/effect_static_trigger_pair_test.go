package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// reproObjName is the card name of object id, "" if it is gone.
func reproObjName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// TestEffectStaticTriggerPairReprosReach: the two repros replay clean in
// lockstep with every scripted action offered and reach what they pin; the
// lockstep rows alone would accept a script that never got there.
//
// Long River Lurker's Effect remembers the Bear it targets: the Bear is
// unblockable, and when it deals combat damage its controller flickers it
// (the trigger fires once, for that creature alone). The Pride of Hull
// Clade's Effect lets the targeted Defender creature attack while an
// Overgrown Battlement stays home, and it draws on the combat damage to a
// player (the one trigger the Animate grant gives).
func TestEffectStaticTriggerPairReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range []struct {
		file  string
		reach func(g *state.Game, evs []events.Event, from int, st map[string]bool)
		want  []string
	}{
		{"effect_capture_trigger_flickers_the_dealer", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
			for _, ev := range evs[from:] {
				switch {
				case ev.Kind == events.TriggerPush && reproObjName(g, ev.Obj) == "Long River Lurker" && g.Step == state.StepCombatDamage:
					st["damage trigger"] = true
				case st["damage trigger"] && ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To == state.ZExile:
					st["exiled"] = true
				case st["exiled"] && ev.Kind == events.MoveZone && ev.From == state.ZExile && ev.To == state.ZBattlefield:
					st["returned"] = true
				}
			}
		}, []string{"damage trigger", "exiled", "returned"}},
		{"effect_capture_static_lets_a_defender_attack", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
			for _, ev := range evs[from:] {
				switch {
				case ev.Kind == events.DeclareAttackers:
					// another Defender creature stays home: the grant is the
					// captured creature's alone
					pride, wall := false, false
					for _, id := range ev.IDs {
						pride = pride || reproObjName(g, id) == "The Pride of Hull Clade"
					}
					for _, id := range g.Zone(state.ZBattlefield, 0) {
						wall = wall || reproObjName(g, id) == "Overgrown Battlement"
					}
					if pride && wall {
						st["defender attacked beside a wall"] = true
					}
				case st["defender attacked beside a wall"] && ev.Kind == events.Draw && ev.Player == 0 && g.Step == state.StepCombatDamage:
					st["drew"] = true
				}
			}
		}, []string{"defender attacked beside a wall", "drew"}},
	} {
		t.Run(c.file, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.file+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			set := printedDelveSet(t, r.Cards)
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
			st, seen := map[string]bool{}, 0
			rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
				c.reach(ref.E.G, ref.E.L.Events, seen, st)
				seen = len(ref.E.L.Events)
				return nil
			}
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			script := &ScriptPolicy{Actions: r.Actions}
			res := Lockstep(m, r.Seed, k, script, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			for _, w := range c.want {
				if !st[w] {
					t.Errorf("the recorded game never reaches %q", w)
				}
			}
		})
	}
}
