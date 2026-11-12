package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestManaDrainClearEmptiesPersistent pins DB$ DrainMana's empty-pool shape:
// unlike a step boundary's ManaClear, persistent and combat units go too, as
// Forge's clearPool(false) drains every unit.
func TestManaDrainClearEmptiesPersistent(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	p := &g.Players[0]
	p.Pool[state.MG] = 3
	p.PersistentMana[state.MG] = 2
	p.CombatMana[state.MG] = 1
	p.Snow[state.MG] = 1
	p.TypedMana[0][state.MG] = 1
	p.ArtifactTyped[0][state.MG] = 1
	p.RestrictedMana = []state.ManaRestriction{{Color: "G", Persistent: true, Amount: 2}}
	if p.Pool[state.MG] != 3 || p.PersistentMana[state.MG] != 2 {
		t.Fatalf("precondition: pool %d persistent %d", p.Pool[state.MG], p.PersistentMana[state.MG])
	}
	Apply(g, Event{Kind: ManaClear, Player: 0, Text: ManaDrainText})
	if p.Pool != (state.Mana{}) || p.PersistentMana != (state.Mana{}) || p.CombatMana != (state.Mana{}) {
		t.Fatalf("drain left pool %v persistent %v combat %v", p.Pool, p.PersistentMana, p.CombatMana)
	}
	if p.Snow != (state.Mana{}) || p.TypedMana != [4]state.Mana{} || p.ArtifactTyped != [3]state.Mana{} {
		t.Fatalf("drain left tallies snow %v typed %v artifact %v", p.Snow, p.TypedMana, p.ArtifactTyped)
	}
	if len(p.RestrictedMana) != 0 {
		t.Fatalf("drain left %d restricted batches", len(p.RestrictedMana))
	}
}

// TestManaClearKeepsPersistent is the boundary twin: an ordinary ManaClear
// still keeps PersistentMana$ units (CR 500.4's exception).
func TestManaClearKeepsPersistent(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	p := &g.Players[0]
	p.Pool[state.MG] = 3
	p.PersistentMana[state.MG] = 2
	Apply(g, Event{Kind: ManaClear, Player: 0})
	if p.Pool[state.MG] != 2 || p.PersistentMana[state.MG] != 2 {
		t.Fatalf("boundary ManaClear = pool %d persistent %d, want 2/2", p.Pool[state.MG], p.PersistentMana[state.MG])
	}
}
