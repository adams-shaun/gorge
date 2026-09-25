package effects

// This file pins the `modified` CardProperty (CR 700.9), the gate the
// attacks-trigger family carrying `ValidCard$ Creature.modified+YouCtrl`
// depends on. It asserts each half of the rule on its own preconditioned
// board -- a counter, an Equipment, and an Aura controlled by the
// candidate's controller -- plus the two negative halves (nothing attached,
// and an OPPONENT's Aura must not satisfy it). Everything runs through the
// real matcher, so the predicate's registration is exercised, and
// TestModifiedPredicateCensus checks UnknownPredicates reads the same map.
//
// Objects are carried BY ID, never by the pointer corpusObject returns:
// state.Game.AddObject appends to g.Objs and may reallocate the backing
// array, which would leave an earlier pointer reading a stale slot. Every
// assertion re-fetches through g.Obj.

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// placeOnBattlefield puts an object into the battlefield zone index of
// player p. corpusObject sets the object's Zone field but not the zone
// index, and hasAttachmentOfKind's scan reads g.Zone, so the index must be
// updated too or the attachment would be invisible to the predicate.
func placeOnBattlefield(g *state.Game, id state.ObjID, p state.PlayerID) {
	g.Obj(id).Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, p, append(g.Zone(state.ZBattlefield, p), id))
}

// TestModifiedPredicate pins CR 700.9 on the three positive halves and the
// two negative halves, each asserted after its own precondition.
func TestModifiedPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	bare := corpusObject(t, reg, g, "Grizzly Bears").ID
	countered := corpusObject(t, reg, g, "Grizzly Bears").ID
	equipped := corpusObject(t, reg, g, "Grizzly Bears").ID
	auraOwn := corpusObject(t, reg, g, "Grizzly Bears").ID
	auraOpp := corpusObject(t, reg, g, "Grizzly Bears").ID

	equip := corpusObject(t, reg, g, "Bonesplitter").ID
	placeOnBattlefield(g, equip, 0)
	g.Obj(equip).AttachedTo = equipped
	g.Obj(equip).Controller = 0

	ownAura := corpusObject(t, reg, g, "Unholy Strength").ID
	placeOnBattlefield(g, ownAura, 0)
	g.Obj(ownAura).AttachedTo = auraOwn
	g.Obj(ownAura).Controller = 0

	oppAura := corpusObject(t, reg, g, "Unholy Strength").ID
	placeOnBattlefield(g, oppAura, 1)
	g.Obj(oppAura).AttachedTo = auraOpp
	g.Obj(oppAura).Controller = 1

	// Precondition: the bare bear really is unmodified (no counters, nothing
	// attached), so the negative assertion below is not a false green.
	if len(g.Obj(bare).Counters) != 0 || g.Obj(bare).AttachedTo != 0 {
		t.Fatalf("precondition failed: bare bear has counters=%v attachedTo=%d", g.Obj(bare).Counters, g.Obj(bare).AttachedTo)
	}
	// Precondition: the attach relationships and controllers are what the
	// assertions depend on, and the two Auras really differ in controller.
	// Precondition: the attachments are really in the battlefield zone index
	// the predicate's scan reads, so the positive assertions are not vacuous.
	for _, id := range []state.ObjID{equip, ownAura, oppAura} {
		if g.Obj(id).Zone != state.ZBattlefield || !containsID(g.Zone(state.ZBattlefield, g.Obj(id).Controller), id) {
			t.Fatalf("precondition failed: attachment %d not in battlefield zone index (zone=%s)", id, g.Obj(id).Zone)
		}
	}
	if g.Obj(equip).AttachedTo != equipped || g.Obj(equip).Controller != 0 {
		t.Fatalf("precondition failed: equip AttachedTo=%d controller=%d", g.Obj(equip).AttachedTo, g.Obj(equip).Controller)
	}
	if g.Obj(ownAura).AttachedTo != auraOwn || g.Obj(oppAura).AttachedTo != auraOpp {
		t.Fatalf("precondition failed: ownAura AttachedTo=%d oppAura AttachedTo=%d, want %d and %d",
			g.Obj(ownAura).AttachedTo, g.Obj(oppAura).AttachedTo, auraOwn, auraOpp)
	}
	if g.Obj(ownAura).Controller != 0 || g.Obj(oppAura).Controller != 1 {
		t.Fatalf("precondition failed: ownAura controller=%d oppAura controller=%d, want 0 and 1",
			g.Obj(ownAura).Controller, g.Obj(oppAura).Controller)
	}
	if matchesModified(g, bare) {
		t.Fatalf("precondition failed: bare Grizzly Bears must not satisfy modified before setup")
	}

	// Positive half 1: a positive counter makes it modified.
	g.Obj(countered).Counters = append(g.Obj(countered).Counters, state.Counter{Kind: "P1P1", N: 2})
	if g.Obj(countered).Counter("P1P1") <= 0 {
		t.Fatalf("precondition failed: countered bear P1P1=%d, want > 0", g.Obj(countered).Counter("P1P1"))
	}
	if !matchesModified(g, countered) {
		t.Errorf("a creature with a +1/+1 counter must satisfy modified (CR 700.9)")
	}

	// Positive half 2: an attached Equipment makes it modified.
	if !matchesModified(g, equipped) {
		t.Errorf("an equipped creature must satisfy modified (CR 700.9)")
	}

	// Positive half 3: an Aura controlled by the candidate's own controller.
	if !matchesModified(g, auraOwn) {
		t.Errorf("a creature enchanted by its controller's Aura must satisfy modified (CR 700.9)")
	}

	// Negative half 1: no counter, nothing attached.
	if matchesModified(g, bare) {
		t.Errorf("an unmodified creature (no counters, nothing attached) must not satisfy modified")
	}

	// Negative half 2: an OPPONENT's Aura does not count (CR 700.9's
	// controller condition). The Aura bears the same type word as the
	// positive case, so only the controller check distinguishes them.
	if matchesModified(g, auraOpp) {
		t.Errorf("a creature enchanted ONLY by an opponent's Aura must not satisfy modified (CR 700.9 controller condition)")
	}
}

// matchesModified evaluates the real `modified` spelling through the matcher
// for the object named by id. The spec uses a bare base plus the token,
// exactly how the corpus writes it (`Creature.modified+YouCtrl`), so the
// registration and the tokeniser are both exercised.
func matchesModified(g *state.Game, id state.ObjID) bool {
	return MatchesObjectCtx(g, "Creature.modified", g.Obj(id), SpecContext{You: g.Obj(id).Controller})
}

// TestModifiedPredicateCensus pins that the matcher and UnknownPredicates
// share one classifier: the corpus spellings are recognised, a misspelled
// neighbour is still reported (fail-closed census), and a zero-N counter
// record does not make an object modified.
func TestModifiedPredicateCensus(t *testing.T) {
	for _, spec := range []string{
		"Creature.modified+YouCtrl",
		"Permanent.modified",
		"Creature.YouCtrl+modified+!EnchantedBy",
	} {
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Fatalf("UnknownPredicates(%q) = %v, want none", spec, got)
		}
	}
	if got := UnknownPredicates("Creature.modifed"); len(got) != 1 || got[0] != "modifed" {
		t.Fatalf("UnknownPredicates(misspelling) = %v, want [modifed]", got)
	}

	// A counter record removed to zero must not make an object modified: the
	// event path leaves a zero-N record behind (the same discipline
	// objectHasKeyword uses). This is the negative that keeps a stale record
	// from a false positive on a real board.
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	id := corpusObject(t, reg, g, "Grizzly Bears").ID
	g.Obj(id).Counters = append(g.Obj(id).Counters, state.Counter{Kind: "P1P1", N: 0})
	if g.Obj(id).Counter("P1P1") != 0 {
		t.Fatalf("precondition failed: zeroed counter reads %d, want 0", g.Obj(id).Counter("P1P1"))
	}
	if matchesModified(g, id) {
		t.Errorf("a zero-N counter record must not make a creature modified")
	}
}
