package rules

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

func TestEngineCompiledTextSharesWithCloneAndFallsBack(t *testing.T) {
	c := card(t, "Name:Cache Test\nManaCost:1 U\nTypes:Creature Test\nPT:1/1\nA:AB$ Draw | Cost$ GWP 2B Sac<1/Creature> | ValidTgts$ Creature.YouCtrl+untapped\nOracle:x\n")
	e := New(Config{Names: []string{"you"}, Decks: [][]*cards.Card{{c}}})
	if e.compiledText == nil || e.compiledText.predicates == nil {
		t.Fatal("New did not build compiled text")
	}
	if got := e.specCtx(0, 0).PredicatePrograms; got != e.compiledText.predicates {
		t.Fatal("spec context did not carry compiled predicates")
	}
	raw := "GWP 2B Sac<1/Creature>"
	if got, want := e.parseCost(raw), ParseCost(raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("cached cost = %#v, want %#v", got, want)
	}
	if got, want := e.parseCost("dynamic cost"), ParseCost("dynamic cost"); !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback cost = %#v, want %#v", got, want)
	}
	clone := e.Clone()
	if clone.compiledText != e.compiledText {
		t.Fatal("clone did not share immutable compiled text")
	}
}
