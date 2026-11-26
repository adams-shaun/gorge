package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestUpdatedAddTypeAnimateCommutes: two "each other creature you control
// enters with N additional +1/+1 counters and as a Mutant in addition to its
// other types" bodies (two Master Biomancers) compose in either order, so the
// entry poses no CR 616.1 order ask; an Animate that sets P/T or grants a
// keyword stays order-sensitive. The bodies are authored here, not copied.
func TestUpdatedAddTypeAnimateCommutes(t *testing.T) {
	t.Parallel()
	body := func(animate map[string]string) replMatch {
		an := &cards.SA{Kind: "DB", API: "Animate", Params: animate}
		put := &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
			"ETB": "True", "Defined": "ReplacedCard", "CounterType": "P1P1", "CounterNum": "X", "SubAbility": "DBAnimate"}, Sub: an}
		return replMatch{repl: &cards.Repl{With: put}}
	}
	addType := map[string]string{"Types": "Mutant", "Duration": "Permanent", "Defined": "ReplacedCard"}
	if !updatedReplacementsCommute([]replMatch{body(addType), body(addType)}) {
		t.Fatal("two add-a-type Animate entry bodies do not commute: the entry would pose an order ask gorge's rule says is moot")
	}
	setPT := map[string]string{"Power": "3", "Toughness": "3", "Duration": "Permanent", "Defined": "ReplacedCard"}
	if updatedReplacementsCommute([]replMatch{body(addType), body(setPT)}) {
		t.Fatal("a P/T-setting Animate commutes: it must stay order-sensitive")
	}
	untilEOT := map[string]string{"Types": "Mutant", "Defined": "ReplacedCard"}
	if updatedReplacementsCommute([]replMatch{body(untilEOT), body(addType)}) {
		t.Fatal("an Animate with no Duration$ Permanent commutes: only the permanent add-a-type shape may")
	}
}
