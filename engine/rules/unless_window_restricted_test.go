package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An unless cost is not a spell or an activated ability, so mana "only to cast
// Elemental spells" cannot pay it (CR 106.6): the window does not list the
// source that makes it, and opens for the plain land beside it.
func TestUnlessWindowOmitsSourceWhoseManaCannotPayIt(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, _ := counterFixture(t, reg, "Daze", "Grizzly Bears")
	e.G.Players[0].Pool = state.Mana{}
	restricted := onBoard(t, e, 0, "Name:Restricted Source\nTypes:Creature Elemental\nPT:2/2\n"+
		"A:AB$ Mana | Cost$ T | Produced$ Any | RestrictValid$ Spell.Elemental,Activated.Elemental | SpellDescription$ x\nOracle:x\n")
	e.G.Obj(restricted).SummonSick = false
	land := onBoard(t, e, 0, choiceLand("W"))

	d := drainUntilUnlessPay(t, e, 30)
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("no payable election: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = e.Pending()
	if d == nil || d.ResumeKind != "unless_mana" {
		t.Fatalf("no mana window after choosing to pay: %+v", d)
	}
	if indexOfObjOption(d, land) < 0 {
		t.Errorf("the plain land is not offered: %+v", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == restricted {
			t.Errorf("the source whose mana cannot pay the unless cost is offered: %+v", d.Options)
		}
	}
}
