package payexec

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestIntrinsicStepTakesTheLastTiedOption: a painland under a granted Island
// type lists {C}, its painful ability's flattened {U} and {B}, then the
// intrinsic {U} (printed abilities first, CR 305.6). A witness step naming the
// intrinsic ability takes the last of the tied "Add U" options, so the lowering
// deals none of the damage the plan priced at nothing; a printed step keeps the
// first.
func TestIntrinsicStepTakesTheLastTiedOption(t *testing.T) {
	d := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "mana", Obj: 7, Ability: 0, Label: "Add C", ManaSymbol: "C"},
		{Index: 1, Kind: "mana", Obj: 7, Ability: 1, Label: "Add U", ManaSymbol: "U"},
		{Index: 2, Kind: "mana", Obj: 7, Ability: 1, Label: "Add B", ManaSymbol: "B"},
		{Index: 3, Kind: "mana", Obj: 7, Ability: 2, Label: "Add U", ManaSymbol: "U"},
	}}
	want := decision.ManaAmount{1: 1}
	intr := decision.PaymentActivation{Source: 7, Produces: want, Ability: decision.PaymentAbility{Kind: decision.PaymentAbilityIntrinsic, Intrinsic: "basic_land"}}
	if got, ok := matchMana(d, intr); !ok || len(got) != 1 || got[0] != 3 {
		t.Fatalf("intrinsic step: pick %v (ok %v), want [3]", got, ok)
	}
	printed := decision.PaymentActivation{Source: 7, Produces: want, Ability: decision.PaymentAbility{Kind: decision.PaymentAbilityPrinted, Index: 1}}
	if got, ok := matchMana(d, printed); !ok || len(got) != 1 || got[0] != 1 {
		t.Fatalf("printed step: pick %v (ok %v), want [1]", got, ok)
	}
}
