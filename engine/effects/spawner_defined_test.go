package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The Spawner> chain on a Defined$/DefinedPlayer$ consumer (cr-g256):
// Biting-Palm Ninja's `RevealHand | Defined$ Spawner>TriggeredTarget` and
// `ChangeZone | DefinedPlayer$ Spawner>TriggeredTarget`, and Breeches, the
// Blastmaker's `CopySpellAbility | Defined$ Spawner>TriggeredSpellAbility`.

func TestSpawnerDefinedTriggeredTarget(t *testing.T) {
	h, c := fixtureHost(t)
	c.TriggerTarget = state.Target{IsPlayer: true, Player: 1}
	// The spawner's capture rides in Captured on an immediate instance and
	// differs from the TriggerTarget, so the chain cannot pass by reading the
	// wrong list.
	c.Captured = []state.Target{{Obj: c.Source}}

	ts, ok := knownDefinedTargets(h, c, "Spawner>TriggeredTarget")
	if !ok || len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 1 {
		t.Fatalf("Spawner>TriggeredTarget = %+v ok=%v, want player 1", ts, ok)
	}
	if got := Defined(h, c, sa(t, "SP$ RevealHand | Defined$ Spawner>TriggeredTarget")); len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("Defined = %+v, want player 1, not the source", got)
	}
	cz := ChangeZoneOf(sa(t, "SP$ ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ Spawner>TriggeredTarget | Chooser$ You | ChangeType$ Card.nonLand | ChangeNum$ 1"))
	owners, ok := handMoveOwners(h, c, nil, cz)
	if !ok || len(owners) != 1 || owners[0] != 1 {
		t.Fatalf("handMoveOwners = %v ok=%v, want [1]", owners, ok)
	}
}

func TestSpawnerDefinedUnknownInnerStaysUnknown(t *testing.T) {
	h, c := fixtureHost(t)
	if ts, ok := knownDefinedTargets(h, c, "Spawner>NoSuchRef"); ok {
		t.Fatalf("Spawner>NoSuchRef resolved known: %+v", ts)
	}
}

func TestSpawnerDefinedCopiesTriggeredSpell(t *testing.T) {
	h := newHost(t, 2)
	spell := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	other := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1", 0)
	if spell.Zone != state.ZStack || spell.ID == other.ID {
		t.Fatalf("precondition: spell zone=%s ids %d/%d", spell.Zone, spell.ID, other.ID)
	}
	// An immediate instance: Remembered is capture-excluded (empty), the
	// spawner's capture is in Captured, and the instance has no targets of
	// its own except a decoy that the default arm would have copied.
	ctx := &Ctx{Source: other.ID, Controller: 0,
		Captured: []state.Target{{Obj: spell.ID}},
		Targets:  []state.Target{{Obj: other.ID}}}
	Resolve(h, ctx, sa(t, "SP$ CopySpellAbility | Defined$ Spawner>TriggeredSpellAbility"))
	if got := copyEvents(h); got != 1 {
		t.Fatalf("StackCopy events=%d, want 1", got)
	}
	for _, ob := range h.g.Objs {
		if ob.IsCopy && ob.Card != spell.Card {
			t.Fatalf("copy is of %v, want the triggering spell's card", ob.Card)
		}
	}

	// A chain that names nothing copies nothing, and never falls back to the
	// chosen targets.
	h2 := newHost(t, 2)
	spellOnStack(t, h2, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	decoy := spellOnStack(t, h2, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1", 0)
	Resolve(h2, &Ctx{Source: decoy.ID, Controller: 0, Targets: []state.Target{{Obj: decoy.ID}}},
		sa(t, "SP$ CopySpellAbility | Defined$ Spawner>TriggeredSpellAbility"))
	if got := copyEvents(h2); got != 0 {
		t.Fatalf("empty chain made %d copies, want 0", got)
	}
}
