package trigmatch

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// payingBoard is a Board whose only live answer is the open cast flow's
// keyword; whileKeywordAdmits reads nothing else.
type payingBoard struct {
	Board
	kw        string
	isSpell   bool
	emergeSac state.ObjID
}

func (b payingBoard) PayingCostOf() (string, bool, state.ObjID, bool) {
	return b.kw, b.isSpell, b.emergeSac, true
}

// Foul Emissary's WhileKeyword$ Spell.withEmerge reads the EMERGE sacrifice,
// not any sacrifice while an emerge cast is open (Forge whileKeywordCheck reads
// the SA whose cost is being paid). Sacrificing the Emissary to an Altar-style
// mana ability mid-cast pays that ability's cost, so it does not qualify.
func TestWhileKeywordEmergeReadsOnlyTheEmergeSacrifice(t *testing.T) {
	t.Parallel()
	const emissary, chosen state.ObjID = 7, 9
	trig := cards.Trigger{Params: map[string]string{"WhileKeyword": "Spell.withEmerge"}}
	b := payingBoard{kw: "Emerge", isSpell: true, emergeSac: chosen}
	if !whileKeywordAdmits(b, trig, events.Sacrifice(chosen)) {
		t.Fatal("precondition: the emerge cost's own sacrifice must admit")
	}
	if whileKeywordAdmits(b, trig, events.Sacrifice(emissary)) {
		t.Fatal("a sacrifice paying a mana ability during an emerge cast admitted")
	}
}
