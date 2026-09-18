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
	again := New(Config{Names: []string{"you"}, Decks: [][]*cards.Card{{c}}})
	if again.compiledText != e.compiledText {
		t.Fatal("equivalent configurations rebuilt immutable compiled text")
	}
}

func TestEngineCompiledTextCacheSeparatesCardLayouts(t *testing.T) {
	deckA := card(t, "Name:Deck A\nTypes:Creature Test\nPT:1/1\nOracle:x\n")
	deckB := card(t, "Name:Deck B\nTypes:Creature Test\nPT:1/1\nOracle:x\n")
	tokenA := card(t, "Name:Token A\nTypes:Creature Test\nPT:1/1\nOracle:x\n")
	tokenB := card(t, "Name:Token B\nTypes:Creature Test\nPT:1/1\nOracle:x\n")

	base := New(Config{Decks: [][]*cards.Card{{deckA}}, Tokens: map[string]*cards.Card{"T": tokenA}})
	otherDeck := New(Config{Decks: [][]*cards.Card{{deckB}}, Tokens: map[string]*cards.Card{"T": tokenA}})
	if otherDeck.compiledText == base.compiledText {
		t.Fatal("different deck card reused immutable compiled text")
	}
	otherToken := New(Config{Decks: [][]*cards.Card{{deckA}}, Tokens: map[string]*cards.Card{"T": tokenB}})
	if otherToken.compiledText == base.compiledText {
		t.Fatal("different token card reused immutable compiled text")
	}
}
