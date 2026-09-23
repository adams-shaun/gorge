package seat

import (
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/view"
)

// An ability's StackView.Card is display art from its source, not the
// ability's own face. Its mana cost must not be used to rank counter targets.
func TestTargetStackValueIgnoresAbilitySourceCard(t *testing.T) {
	source := view.CardView{ManaCost: "2 U"}
	if got := botpolicy.CmcOf(source.ManaCost); got != 3 {
		t.Fatalf("fixture source mana value = %d, want nonzero 3", got)
	}
	v := view.View{Stack: []view.StackView{
		{ID: 11, Kind: "trigger", Card: &source},
		{ID: 12, Kind: "spell", Card: &source},
	}}
	if v.Stack[0].Card == nil || v.Stack[0].Kind != "trigger" || v.Stack[1].Kind != "spell" {
		t.Fatal("fixture must offer a displayed source card on an ability and a spell")
	}
	got := boardFromView(v).Stack
	if len(got) != 2 || got[0].IsSpell || !got[1].IsSpell {
		t.Fatalf("stack spell identities = %+v, want ability then spell", got)
	}
	if got[0].CMC != 0 || got[1].CMC != 3 {
		t.Fatalf("stack mana values = %d, %d; want source-bearing ability 0, spell 3", got[0].CMC, got[1].CMC)
	}
}
