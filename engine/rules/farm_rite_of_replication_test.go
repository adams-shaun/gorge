package rules

import "testing"

// Farm FDN-s475 window xe:RAND:1:9:w18 (field permanents, 3way FX|G). The
// farm input seats two Confiscates on the same Needletooth Pack: p0's, then
// p1's. The first differing value is the Pack's controller -- gorge says p0,
// Forge and XMage say p1 -- and the ticket attributes the window to Rite of
// Replication, which sat in the same window but does not touch the Pack.
//
// CR 613.1b applies control-changing effects in layer 2, and CR 613.7a gives
// a static ability's effect the timestamp of the object it is on, so the two
// Confiscates apply in timestamp order and the later one (p1's, placed
// second) wins. The 3way FX|G verdict is right: gorge let the older Aura
// keep the permanent because its wanted transfer was skipped while it
// already matched and then re-registered after the newer Aura took control.
func TestFarmRiteofReplication(t *testing.T) {
	t.Parallel()
	// The battlefield and attached lists are the farm window's setup; the
	// lands let the window's own first step (an activate) still run.
	const scenario = `{
	  "name": "farm-rite-of-replication-two-confiscates",
	  "turn": 1,
	  "cr": ["613.1b", "613.7a"],
	  "why": "FDN-s475 xe:RAND:1:9:w18: two Confiscates on one Needletooth Pack, later timestamp wins",
	  "setup": {
	    "p0": {
	      "battlefield": ["Thornwood Falls", "Forest", "Forest", "Island", "Forest", "Forest", "Needletooth Pack", "Island", "Confiscate"],
	      "hand": ["Rite of Replication"],
	      "attached": [{"card": "Confiscate", "to": "p0:Needletooth Pack"}],
	      "life": 19
	    },
	    "p1": {
	      "battlefield": ["Confiscate"],
	      "attached": [{"card": "Confiscate", "to": "p0:Needletooth Pack"}],
	      "life": 23
	    }
	  },
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Forest#2", "ability_index": 0}
	  ]
	}`
	res, run := runFixtureScenario(t, scenario)

	pack := run.refs["p0:Needletooth Pack"]
	p0Aura := run.e.G.Obj(run.refs["p0:Confiscate"])
	p1Aura := run.e.G.Obj(run.refs["p1:Confiscate"])
	if pack == 0 || p0Aura == nil || p1Aura == nil {
		t.Fatalf("setup did not place the Pack and both Confiscates: pack=%d p0=%v p1=%v", pack, p0Aura, p1Aura)
	}
	// Precondition: both control statics live on the same bearer, and p1's
	// Aura is the later layer-2 effect -- so the rule under test really is
	// timestamp order, not the Aura count or seat order.
	if p0Aura.AttachedTo != pack || p1Aura.AttachedTo != pack {
		t.Fatalf("Confiscates attached to %d and %d, want both on the Pack %d", p0Aura.AttachedTo, p1Aura.AttachedTo, pack)
	}
	if !(p0Aura.LayerTimestamp() < p1Aura.LayerTimestamp()) {
		t.Fatalf("timestamps p0=%d p1=%d: want p1's Confiscate later", p0Aura.LayerTimestamp(), p1Aura.LayerTimestamp())
	}
	for _, s := range res.Snapshots {
		p, ok := snapPerm(s, "p0:Needletooth Pack")
		if !ok {
			t.Fatalf("%s: Needletooth Pack not on the battlefield", s.Checkpoint)
		}
		if p.Owner != 0 {
			t.Fatalf("%s: Pack owner = %d, want 0 (the Auras must not change ownership)", s.Checkpoint, p.Owner)
		}
		if p.PT != "4/5" || len(p.Types) != 2 || p.Types[0] != "Creature" || p.Types[1] != "Dinosaur" {
			t.Fatalf("%s: Pack = %v %s, want creature dinosaur 4/5", s.Checkpoint, p.Types, p.PT)
		}
		if p.Controller != 1 {
			t.Fatalf("%s: Pack controller = %d, want 1: p1's Confiscate has the later timestamp and wins layer 2 (CR 613.1b, 613.7a)", s.Checkpoint, p.Controller)
		}
	}
}
