package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-tdm lockstep (TDM-s116 game xe:RAND:2:7) diverged on the stack depth
// at turn 10 with Undergrowth Leopard implicated: {1}, Sacrifice it: destroy target
// artifact or enchantment. The pinned game reaches that activation in response to a
// Cori-Steel Cutter Flurry trigger and must replay clean: the Leopard destroys the
// Equipment before the Flurry's optional attach resolves, so neither engine may ask
// the attach.
func TestPinnedFarmUndergrowthLeopard(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-tdm", Config{GorgeAutopaySurface: true}, leopardAbilityOnStack)
}

// leopardAbilityOnStack: an activated ability of Undergrowth Leopard is on the stack.
// An ability object has no face of its own; its Source is the permanent.
func leopardAbilityOnStack(ref *GorgeSide) bool {
	g := ref.E.G
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Ability == nil || state.StackKindOf(g, o) != state.StackKindActivated {
			continue
		}
		if src := g.Obj(o.Source); src != nil && src.Face() != nil && src.Face().Name == "Undergrowth Leopard" {
			return true
		}
	}
	return false
}
