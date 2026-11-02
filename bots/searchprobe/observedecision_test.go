package searchprobe

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestObserveDecisionRefusesOtherSeatsAndNil(t *testing.T) {
	c := NewCollector(0)
	if _, err := c.ObserveDecision(nil, nil); err == nil {
		t.Fatal("a nil decision was observed")
	}
	d := &decision.Decision{Seq: 3, Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass", Label: "Pass"}}}
	if _, err := c.ObserveDecision(nil, d); err == nil {
		t.Fatal("another seat's private decision was observed")
	}
}

// A fresh collector that has never Captured anything can name, and match
// back, every option of the actor's own decision once ObserveDecision has
// run -- the property an engine clone walked past the last Capture needs.
func TestObserveDecisionNamesEveryOptionOfARealDecision(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	da, err := testutil.LoadRepoDeck(reg, "mono-red-prowess")
	if err != nil {
		t.Fatal(err)
	}
	db, err := testutil.LoadRepoDeck(reg, "mono-blue-tempo")
	if err != nil {
		t.Fatal(err)
	}
	const seed = uint64(30000000)
	e := rules.New(rules.Config{Seed: seed, Names: []string{"mono-red-prowess", "mono-blue-tempo"},
		Decks: [][]*cards.Card{da, db}, Tokens: reg.Tokens})
	e.Advance()
	rngs := BotRandoms(seed, 2)
	board := botpolicy.NewBoard(2)
	checked := 0
	for steps := 0; steps < 600 && !e.G.Over && checked < 25; steps++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision at step %d", steps)
		}
		if d.Player == 0 && len(d.Options) > 1 {
			c := NewCollector(0)
			od, err := c.ObserveDecision(e, d)
			if err != nil {
				t.Fatalf("step %d (%s): ObserveDecision: %v", steps, d.Kind, err)
			}
			if len(od.Options) != len(d.Options) {
				t.Fatalf("step %d: observed %d options, decision has %d", steps, len(od.Options), len(d.Options))
			}
			for i := range d.Options {
				in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}
				if d.Validate(in) != nil {
					continue
				}
				acts, err := c.Actions(d, in)
				if err != nil {
					t.Fatalf("step %d option %d: Actions after ObserveDecision: %v", steps, i, err)
				}
				if acts[0] != od.Options[i].Action {
					t.Fatalf("step %d option %d: Actions %+v, observed %+v", steps, i, acts[0], od.Options[i].Action)
				}
				back, err := c.Match(d, acts)
				if err != nil {
					if strings.Contains(err.Error(), "unobserved") {
						t.Fatalf("step %d option %d: Match: %v", steps, i, err)
					}
					continue // an ambiguous semantic action is a legitimate refusal
				}
				if len(back.Choices) != 1 || back.Choices[0] != i {
					t.Fatalf("step %d option %d: matched back to %v", steps, i, back.Choices)
				}
			}
			checked++
		}
		in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
		if err := e.Submit(in); err != nil {
			t.Fatalf("step %d submit: %v", steps, err)
		}
	}
	if checked == 0 {
		t.Fatal("seat 0 never faced a multi-option decision")
	}
}

// An attack on a planeswalker or a battle names the attacked permanent too, and
// a clone walked past the last Capture may never have seen it.
func TestObserveDecisionNamesAnAttackedPermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deck, err := testutil.LoadRepoDeck(reg, "mono-red-prowess")
	if err != nil {
		t.Fatal(err)
	}
	e := rules.New(rules.Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
	e.Advance()
	lib := e.G.Zone(state.ZLibrary, 0)
	creature, walker := lib[0], lib[1]
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KAttackers, Max: 2, Options: []decision.Option{
		{Index: 0, Kind: "attacker", Label: "Attack b", Obj: creature, Player: 1},
		{Index: 1, Kind: "attacker", Label: "Attack the walker", Obj: creature, Player: 1, Battle: walker},
	}}
	c := NewCollector(0)
	if _, err := c.ObserveDecision(e, d); err != nil {
		t.Fatal(err)
	}
	player, err := c.IntentKey(d, decision.Intent{Seq: 1, Choices: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	player = slices.Clone(player)
	battle, err := c.IntentKey(d, decision.Intent{Seq: 1, Choices: []int{1}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(player, battle) {
		t.Fatal("an attack on the permanent has the key of an attack on its controller")
	}
}
