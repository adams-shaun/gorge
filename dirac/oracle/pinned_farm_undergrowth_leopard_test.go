package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-tdm lockstep (TDM-s116 game xe:RAND:2:7) diverged on the stack depth
// at turn 10 with Undergrowth Leopard implicated: {1}, Sacrifice it: destroy target
// artifact or enchantment. The pinned games activate the Leopard (its ability on the
// stack) and must replay clean.
func TestPinnedFarmUndergrowthLeopard(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-tdm", Config{}, leopardAbilityOnStack)
}

// leopardAbilityOnStack: an activated ability of Undergrowth Leopard is on the stack.
func leopardAbilityOnStack(ref *GorgeSide) bool {
	g := ref.E.G
	for _, id := range g.Zone(state.ZStack, 0) {
		if o := g.Obj(id); o != nil && o.StackKind == state.StackKindActivated && o.Face() != nil && o.Face().Name == "Undergrowth Leopard" {
			return true
		}
	}
	return false
}
