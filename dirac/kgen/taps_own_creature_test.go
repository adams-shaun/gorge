package kgen

import "testing"

// TestTapsOwnCreature: the non-Self Taps watchers lower. Hawkeye's Bow's
// "whenever equipped creature becomes tapped" (ValidCard$ Creature.EquippedBy,
// no ValidPlayer$: any tapper) is a trigger whose FAttachedBySrc filter is
// read against the tapped permanent relative to the Bow's slot, and whose
// body deals 1 to each opponent from the trigger object (DamageSource$
// TriggeredCardLKICopy, dmgSrcPlayers). Captain America, Living Legend's
// YouCtrl watcher lowers too: FirstTime$ True reads the per-permanent
// tapped-this-turn mark in the mechanic arena.
func TestTapsOwnCreature(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Hawkeye's Bow", "Captain America, Living Legend"}) {
		switch c.Name {
		case "Captain America, Living Legend":
			if len(c.Refusals) > 0 {
				t.Fatalf("%s: refused: %v", c.Name, c.Refusals)
			}
			found := false
			for _, a := range c.Abilities {
				if a.Trigger != nil && a.Trigger.Event == "Taps" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no Taps trigger lowered", c.Name)
			}
		case "Hawkeye's Bow":
			if len(c.Refusals) > 0 {
				t.Fatalf("refused: %v", c.Refusals)
			}
			found := false
			for _, a := range c.Abilities {
				tr := a.Trigger
				if tr == nil || tr.Event != "Taps" {
					continue
				}
				found = true
				if tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Types: tCreature, Flags: fEquippedBy}) {
					t.Errorf("%s: trigger filter %v", c.Name, tr.F)
				}
				if tr.Whose != whoAny {
					t.Errorf("%s: Whose %d, want whoAny (any tapper)", c.Name, tr.Whose)
				}
				dmg := false
				for _, in := range a.Prog {
					if in.Op == "DealDamageFrom" && in.Obj.Kind == rTriggered && in.CS == "players" {
						dmg = true
					}
				}
				if !dmg {
					t.Errorf("%s: no DealDamageFrom from the trigger object to each opponent: %v", c.Name, a.Prog)
				}
			}
			if !found {
				t.Errorf("%s: no Taps trigger", c.Name)
			}
		}
	}
}
