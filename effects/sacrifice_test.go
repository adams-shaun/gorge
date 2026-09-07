package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// putBattlefield drops a freshly built parsed card onto a player's
// battlefield in zone order, returning the object's ID. Mirrors filter_test's
// board(t) helper: test booths mutate zone directly rather than through
// events.Apply, which is a test-setup concern, not a rule-mutation one
// (the assertion path below always observes events through h.log).
func putBattlefield(h *fakeHost, owner state.PlayerID, src string) state.ObjID {
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		panic(d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	o := h.g.AddObject(c, owner)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, owner, append(h.g.Zone(state.ZBattlefield, owner), o.ID))
	return o.ID
}

func sacrificeParams(extra map[string]string) *cards.SA {
	p := map[string]string{"API": "Sacrifice"}
	for k, v := range extra {
		p[k] = v
	}
	return &cards.SA{Params: p}
}

// zoneOf reports an object's zone.
func sacZone(h *fakeHost, id state.ObjID) state.Zone {
	return h.g.Obj(id).Zone
}

// TestGatekeeperOfMalakirKickedETBMakesPlayerSacrifice is the head test of
// this fix: the ACTUAL compiled corpus SA for Gatekeeper of Malakir's kicked
// enters-the-battlefield trigger is driven through effSacrifice, and the
// targeted (opponent) player must give up a creature. Before the fix the
// player target was skipped entirely (`t.IsPlayer -> continue`), so the
// effect did nothing at all.
func TestGatekeeperOfMalakirKickedETBMakesPlayerSacrifice(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	card, ok := r.Lookup("Gatekeeper of Malakir")
	if !ok {
		t.Fatal("corpus has no Gatekeeper of Malakir")
	}
	f := card.Faces[0]

	var sac *cards.SA
	for _, tr := range f.Triggers {
		if tr.Mode == "ChangesZone" && tr.Effect != nil && tr.Effect.API == "Sacrifice" {
			sac = tr.Effect
			break
		}
	}
	if sac == nil {
		t.Fatal("Gatekeeper trigger has no Sacrifice effect")
	}
	if sac.Params["SacValid"] != "Creature" {
		t.Fatalf("compiled Gatekeeper SacValid = %q, want Creature", sac.Params["SacValid"])
	}

	h := newHost(t, 2)
	gk := h.g.AddObject(card, 0) // Gatekeeper permanent, controlled by seat 0
	h.g.Obj(gk.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), gk.ID))
	oppCreature := putBattlefield(h, 1, "Name:Victim\nManaCost:1 B\nTypes:Creature Zombie\nPT:2/2\nOracle:x\n")
	oppLand := putBattlefield(h, 1, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")

	// The engine would put the "target player" decision onto the stack and
	// later resolve the trigger's Execute$ with that chosen target; here the
	// resolved-from-corpus SA is driven directly with that target bound, which
	// is what the trigger's resolution feeds it.
	c := &Ctx{Source: gk.ID, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sac)

	if sacZone(h, oppCreature) != state.ZGraveyard {
		t.Fatalf("opponent creature zone = %v, want graveyard (the kicked ETB did nothing)", sacZone(h, oppCreature))
	}
	if sacZone(h, oppLand) != state.ZBattlefield {
		t.Fatalf("opponent land zone = %v, want battlefield (a creature-only SacValid must not take a land)", sacZone(h, oppLand))
	}
	// The Gatekeeper (source) must not have been sacrificed by this effect.
	if sacZone(h, gk.ID) != state.ZBattlefield {
		t.Fatalf("Gatekeeper itself zone = %v, want battlefield", sacZone(h, gk.ID))
	}
}

// TestSacValidCreatureNeverSacrificesLandOrArtifact proves the SacValid$
// gate: a player hands over a creature, never their land or artifact.
func TestSacValidCreatureNeverSacrificesLandOrArtifact(t *testing.T) {
	h := newHost(t, 2)
	creature := putBattlefield(h, 1, "Name:Grizzly\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	land := putBattlefield(h, 1, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	artifact := putBattlefield(h, 1, "Name:Sigil\nManaCost:2\nTypes:Artifact\nOracle:x\n")

	c := &Ctx{Source: 1, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	sac := sacrificeParams(map[string]string{"Defined": "Targeted", "SacValid": "Creature"})
	effSacrifice(h, c, sac)

	if sacZone(h, creature) != state.ZGraveyard {
		t.Fatalf("creature zone = %v, want graveyard", sacZone(h, creature))
	}
	if sacZone(h, land) != state.ZBattlefield {
		t.Fatalf("land zone = %v, want battlefield", sacZone(h, land))
	}
	if sacZone(h, artifact) != state.ZBattlefield {
		t.Fatalf("artifact zone = %v, want battlefield", sacZone(h, artifact))
	}
}

// TestSacrificePlayerWithNoMatchingPermanentSacrificesNothing: no matching
// permanent means nothing is sacrificed and nothing crashes.
func TestSacrificePlayerWithNoMatchingPermanentSacrificesNothing(t *testing.T) {
	h := newHost(t, 2)
	land := putBattlefield(h, 1, "Name:Mountainside\nTypes:Basic Land Mountain\nOracle:x\n")

	c := &Ctx{Source: 1, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	sac := sacrificeParams(map[string]string{"Defined": "Targeted", "SacValid": "Creature"})
	effSacrifice(h, c, sac)

	if sacZone(h, land) != state.ZBattlefield {
		t.Fatalf("land zone = %v, want battlefield", sacZone(h, land))
	}
	if len(h.log) != 0 {
		t.Fatalf("expected no events, got %+v", h.log)
	}
}

// TestSacrificeChoiceIsDeterministic: the same board sacrifices the same
// permanent every time -- no rng, no map-range, no clock.
func TestSacrificeChoiceIsDeterministic(t *testing.T) {
	run := func() state.ObjID {
		h := newHost(t, 2)
		first := putBattlefield(h, 1, "Name:Alpha\nManaCost:1 B\nTypes:Creature Horror\nPT:3/3\nOracle:x\n")
		second := putBattlefield(h, 1, "Name:Beta\nManaCost:1 B\nTypes:Creature Horror\nPT:3/3\nOracle:x\n")
		c := &Ctx{Source: 1, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
		effSacrifice(h, c, sacrificeParams(map[string]string{"Defined": "Targeted", "SacValid": "Creature"}))
		if sacZone(h, first) != state.ZGraveyard && sacZone(h, second) != state.ZGraveyard {
			t.Fatalf("nothing sacrificed on this run")
		}
		if sacZone(h, first) == state.ZGraveyard {
			return first
		}
		return second
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("sacrifice choice not deterministic: run1=%d run2=%d", a, b)
	}
}

// TestSacrificeIgnoresIndestructibleViaPlayerTarget: sacrificing is not destruction, so an
// Indestructible permanent is a perfectly legal sacrifice.
func TestSacrificeIgnoresIndestructibleViaPlayerTarget(t *testing.T) {
	h := newHost(t, 2)
	indestructible := putBattlefield(h, 1, "Name:Wardstone\nManaCost:4\nTypes:Artifact Creature\nPT:5/5\nK:Indestructible\nOracle:x\n")

	c := &Ctx{Source: 1, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	effSacrifice(h, c, sacrificeParams(map[string]string{"Defined": "Targeted", "SacValid": "Creature"}))

	if sacZone(h, indestructible) != state.ZGraveyard {
		t.Fatalf("indestructible permanent zone = %v, want graveyard (sacrifice is not destruction)", sacZone(h, indestructible))
	}
}

// TestSacrificeDoesNotConsumeRegenerationShield: a permanent with a
// regeneration shield is sacrificed and the shield is NOT consumed --
// ReplaceDestruction lives in this package and must not be called.
func TestSacrificeDoesNotConsumeRegenerationShield(t *testing.T) {
	h := newHost(t, 2)
	creature := putBattlefield(h, 1, "Name:Regener\nManaCost:1 G\nTypes:Creature Beast\nPT:3/3\nOracle:x\n")
	h.g.Obj(creature).AddCounter("Shield", 1)

	c := &Ctx{Source: 1, Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	effSacrifice(h, c, sacrificeParams(map[string]string{"Defined": "Targeted", "SacValid": "Creature"}))

	if sacZone(h, creature) != state.ZGraveyard {
		t.Fatalf("creature zone = %v, want graveyard (a regen shield must not save a sacrifice)", sacZone(h, creature))
	}
	for _, e := range h.log {
		if e.Kind == events.CounterChange && e.Counter == "Shield" {
			t.Fatalf("sacrifice consumed a regeneration shield: %+v", e)
		}
	}
}

// TestSacrificeObjectTargetPathUnchanged guards the existing non-player
// Defined$ path: a concrete object target is sacrificed as-is, with no
// SacValid$ re-filter, exactly as before the fix.
func TestSacrificeObjectTargetPathUnchanged(t *testing.T) {
	h := newHost(t, 2)
	src := putBattlefield(h, 0, "Name:TriggerSrc\nTypes:Creature\nPT:1/1\nOracle:x\n")
	target := putBattlefield(h, 0, "Name:Victim2\nManaCost:1 R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")

	c := &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target}}}
	sac := sacrificeParams(map[string]string{"Defined": "Targeted"})
	effSacrifice(h, c, sac)

	if sacZone(h, target) != state.ZGraveyard {
		t.Fatalf("object-targeted creature zone = %v, want graveyard", sacZone(h, target))
	}
	if sacZone(h, src) != state.ZBattlefield {
		t.Fatalf("source zone = %v, want battlefield", sacZone(h, src))
	}
}
