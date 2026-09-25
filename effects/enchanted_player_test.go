package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestEnchantedPlayerReader is the reader test for the seat a source Aura
// enchants, now that state.Object carries the AttachedPlayer/HasAttachedPlayer
// pair. It covers both consumers added with the player-attachment model:
// definedSpec's `Defined$ EnchantedPlayer` (Curse of Misfortunes'
// `AttachedToPlayer$ EnchantedPlayer`) and unlessPayerTargets'
// `UnlessPayer$ EnchantedPlayer` (Overencumbered). Both resolve through the
// resolving SOURCE's own AttachedPlayer -- the attached Curse is the source of
// its own trigger -- and both fail closed on a source that is not attached to
// a player.
func TestEnchantedPlayerReader(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	curse := corpusObject(t, reg, g, "Curse of the Pierced Heart")
	curse.HasAttachedPlayer = true
	curse.AttachedPlayer = 1

	// Precondition: the enchanted seat must differ from the resolving
	// controller, or a wrong "falls back to the controller" read would pass.
	if curse.AttachedPlayer == 0 {
		t.Fatal("precondition: the Curse must enchant seat 1, not the controller")
	}
	c := &Ctx{Source: curse.ID, Controller: 0}

	ts := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "EnchantedPlayer"}})
	if len(ts) != 1 || !ts[0].IsPlayer || ts[0].Player != 1 {
		t.Fatalf("Defined$ EnchantedPlayer = %+v, want the enchanted seat 1", ts)
	}

	payers, known := unlessPayerTargets(h, c, &cards.SA{Params: map[string]string{"UnlessPayer": "EnchantedPlayer"}})
	if !known {
		t.Fatal("UnlessPayer$ EnchantedPlayer must resolve from the source's own attachment")
	}
	if len(payers) != 1 || !payers[0].IsPlayer || payers[0].Player != 1 {
		t.Fatalf("UnlessPayer$ EnchantedPlayer = %+v, want the enchanted seat 1", payers)
	}

	// A source that is not attached to a player resolves to nobody, never a
	// guessed seat (the fail-closed convention).
	free := corpusObject(t, reg, g, "Unholy Strength")
	if free.HasAttachedPlayer {
		t.Fatal("precondition: the control Aura must not enchant a player")
	}
	cFree := &Ctx{Source: free.ID, Controller: 0}
	if ts := Defined(h, cFree, &cards.SA{Params: map[string]string{"Defined": "EnchantedPlayer"}}); len(ts) != 0 {
		t.Fatalf("Defined$ EnchantedPlayer on an unattached source = %+v, want nobody", ts)
	}
	if _, known := unlessPayerTargets(h, cFree, &cards.SA{Params: map[string]string{"UnlessPayer": "EnchantedPlayer"}}); known {
		t.Fatal("UnlessPayer$ EnchantedPlayer on an unattached source must be unresolved")
	}
}
