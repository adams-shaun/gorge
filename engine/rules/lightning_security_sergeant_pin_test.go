package rules

import "testing"

// TestLightningSecuritySergeantExilesTopCardAndMayPlay pins the FIN farm
// windows (s584 / s667) whose step is Lightning, Security Sergeant's
// combat-damage trigger: Dig 1 to exile, then a may-play grant while you
// control Lightning. The engine exiles the library's top card and offers it
// for casting; the farm "divergence" is only each engine's post-shuffle top.
func TestLightningSecuritySergeantExilesTopCardAndMayPlay(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "lightning-security-sergeant-exiles-top",
	  "setup": {"p0": {"battlefield": ["Lightning, Security Sergeant"], "library_top": ["Lightning Bolt", "Mountain", "Forest"]},
	            "p1": {"life": 20}},
	  "steps": [
	    {"op": "attack", "seat": 0, "attackers": ["p0:Lightning, Security Sergeant"], "defender": "p1"},
	    {"op": "pass_to", "step": "combat-damage", "expect": [{"life": {"p1": 18}}]},
	    {"op": "resolve", "expect": [{"card": "p0:Lightning Bolt", "zone": "exile"}]},
	    {"op": "mana", "seat": 0, "mana": "R",
	     "expect": [{"offered": {"seat": 0, "kind": "cast", "card": "p0:Lightning Bolt"}, "want": true}]},
	    {"op": "cast", "seat": 0, "card": "p0:Lightning Bolt", "targets": ["p1"]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"life": {"p1": 15}}, {"card": "p0:Lightning Bolt", "zone": "graveyard"}]
	}`)
	if run.refs["p0:Lightning Bolt"] == 0 {
		t.Fatal("setup: p0:Lightning Bolt was never bound")
	}
	if run.refs["p0:Lightning, Security Sergeant"] == 0 {
		t.Fatal("setup: Lightning, Security Sergeant was never bound")
	}
}
