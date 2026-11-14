package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestManaGrantIsPresentGate: a Continuous AddAbility$ mana grant carrying an
// IsPresent$ band joins the mana membership only while the band holds
// (Forge's StaticAbility.checkConditions). Joraga Treespeaker's "LEVEL 1-4"
// {T}: Add {G}{G} must not be offered at level 0, and is offered at levels 1
// to 4; its level-5 grant goes to Elves, itself included.
func TestManaGrantIsPresentGate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Joraga Treespeaker")}, nil)
	id := moveByName(t, e, 0, "Joraga Treespeaker", state.ZBattlefield)
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Treespeaker is not on the battlefield")
	}
	e.G.Obj(id).SummonSick = false // a fixture setup, as the autopay fixtures do
	counted := func() int { return len(e.availableManaAbilities(0, id)) }
	if n := counted(); n != 0 {
		t.Fatalf("level 0: %d mana abilities offered, want 0 (the LEVEL 1-4 grant is not live)", n)
	}
	for _, level := range []int32{1, 4, 5} {
		cur := e.G.Obj(id).Counter("LEVEL")
		putCountersOn(t, e, 0, "Joraga Treespeaker", "LEVEL", level-cur)
		if got := e.G.Obj(id).Counter("LEVEL"); got != level {
			t.Fatalf("precondition: level %d, want %d", got, level)
		}
		if n := counted(); n != 1 {
			t.Fatalf("level %d: %d mana abilities offered, want 1", level, n)
		}
	}
}
