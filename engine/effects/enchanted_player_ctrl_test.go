package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEnchantedPlayerCtrlWord pins Forge's EnchantedPlayerCtrl (CardProperty
// "EnchantedPlayer" + the Ctrl suffix; Trespasser's Curse's
// "Creature.EnchantedPlayerCtrl"): the candidate is controlled by the player
// the SOURCE Aura enchants. Before this word the spec was an unknown
// predicate and matched nothing, so the Curse never triggered.
func TestEnchantedPlayerCtrlWord(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them", "third"})
	curse := corpusObject(t, reg, g, "Trespasser's Curse")
	curse.HasAttachedPlayer = true
	curse.AttachedPlayer = 1
	mine := corpusObjectInSeat(t, reg, g, "Grizzly Bears", 0, state.ZBattlefield)
	theirs := corpusObjectInSeat(t, reg, g, "Grizzly Bears", 1, state.ZBattlefield)
	third := corpusObjectInSeat(t, reg, g, "Grizzly Bears", 2, state.ZBattlefield)
	if mine.Controller == theirs.Controller || theirs.Controller != curse.AttachedPlayer {
		t.Fatalf("precondition: controllers %d %d, enchanted seat %d", mine.Controller, theirs.Controller, curse.AttachedPlayer)
	}
	if got := UnknownPredicates("Creature.EnchantedPlayerCtrl"); len(got) != 0 {
		t.Fatalf("EnchantedPlayerCtrl is still an unknown predicate: %v", got)
	}
	sc := SpecContext{You: 0, Source: curse.ID}
	for _, tc := range []struct {
		spec string
		o    *state.Object
		want bool
	}{
		{"Creature.EnchantedPlayerCtrl", theirs, true},
		{"Creature.EnchantedPlayerCtrl", mine, false},
		{"Creature.EnchantedPlayerCtrl", third, false},
		{"Creature.!EnchantedPlayerCtrl", theirs, false},
		{"Creature.!EnchantedPlayerCtrl", mine, true},
	} {
		if got := MatchesObjectCtx(g, tc.spec, tc.o, sc); got != tc.want {
			t.Errorf("%s on seat %d's creature = %v, want %v", tc.spec, tc.o.Controller, got, tc.want)
		}
	}

	// A source enchanting no player (Forge: "source not enchanting a player")
	// matches nothing, beneath '!' too. A fresh game: the match memo is keyed
	// on the game's version, which a direct field write does not move.
	g2 := state.NewGame([]string{"you", "them", "third"})
	free := corpusObject(t, reg, g2, "Trespasser's Curse")
	theirs2 := corpusObjectInSeat(t, reg, g2, "Grizzly Bears", 1, state.ZBattlefield)
	if free.HasAttachedPlayer {
		t.Fatal("precondition: the fresh Curse must enchant no player")
	}
	for _, spec := range []string{"Creature.EnchantedPlayerCtrl", "Creature.!EnchantedPlayerCtrl"} {
		if MatchesObjectCtx(g2, spec, theirs2, SpecContext{You: 0, Source: free.ID}) {
			t.Errorf("%s with an unattached source matched", spec)
		}
	}
}
