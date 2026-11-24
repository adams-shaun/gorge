package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Welding Jar's "Sacrifice: Regenerate target artifact" aimed at Herbal Poultice
// leaves a this-turn regeneration shield on the Poultice (CR 701.19a). The
// engine keeps that shield as the internal "Shield" marker, which is not a
// counter (CR 122.1): neither Forge nor XMage lists one, so the oracle snapshot
// must not export it as a SHIELD counter (xproof 11240626).
func TestHerbalPoulticeNoSelfShield(t *testing.T) {
	const raw = `{
 "name":"herbal-poultice-no-self-shield","cr":["701.19a","122.1"],
 "setup":{"p0":{"battlefield":["Welding Jar"]},
          "p1":{"battlefield":["Herbal Poultice"]}},
 "steps":[{"op":"activate","seat":0,"card":"p0:Welding Jar","targets":["p1:Herbal Poultice"],"ability_index":0},
          {"op":"pass","seat":0},{"op":"pass","seat":1}],
 "expect":[]}`
	res, run := runFixtureScenario(t, raw)
	var poultice int32
	for _, id := range run.e.G.Zone(state.ZBattlefield, 1) {
		if o := run.e.G.Obj(id); o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Herbal Poultice" {
			poultice = o.Counter("Shield")
		}
	}
	if poultice != 1 {
		t.Fatalf("precondition: Herbal Poultice regeneration marker = %d, want 1 (the Jar resolved onto it)", poultice)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	found := false
	for _, p := range last.Permanents {
		if p.Name != "Herbal Poultice" {
			continue
		}
		found = true
		if len(p.Counters) != 0 {
			t.Fatalf("Herbal Poultice exports counters %v, want none (a regeneration shield is not a counter)", p.Counters)
		}
	}
	if !found {
		t.Fatal("Herbal Poultice is not on the battlefield in the last snapshot")
	}
}

// A real shield counter (CR 122.1c), placed by setup, is still exported.
func TestSnapshotKeepsRealShieldCounter(t *testing.T) {
	const raw = `{
 "name":"real-shield-counter-exported","cr":["122.1c"],
 "setup":{"p0":{"battlefield":["Herbal Poultice"],"counters":{"Herbal Poultice":{"SHIELD":1}}}},
 "steps":[],"expect":[]}`
	res, _ := runFixtureScenario(t, raw)
	for _, p := range res.Snapshots[0].Permanents {
		if p.Name == "Herbal Poultice" {
			if p.Counters["SHIELD"] != 1 {
				t.Fatalf("real SHIELD counter lost from the snapshot: %v", p.Counters)
			}
			return
		}
	}
	t.Fatal("Herbal Poultice missing from the snapshot")
}
