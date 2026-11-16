package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// choiceInstrs is every PutCounterChoice in c: the abilities' programs and the
// replacements' (the as-enters form).
func choiceInstrs(c *lCard) []lInstr {
	var out []lInstr
	add := func(prog []lInstr) {
		for _, in := range prog {
			if in.Op == "PutCounterChoice" {
				out = append(out, in)
			}
		}
	}
	for _, a := range c.Abilities {
		add(a.Prog)
	}
	for _, r := range c.Repls {
		add(r.Prog)
	}
	return out
}

// A PutCounter whose CounterType$ lists kinds is the controller's choice of
// one (gorge effPutCounter's counterKinds > 1 arm), not CounterTypes$'s
// every-kind list: every corpus card that spells one lowers to
// PutCounterChoice over its listed kinds, in printed order, on the object it
// puts on (the resolving form) or on the entering permanent (ETB$ True).
func TestCounterKindChoiceLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		card, kinds string
		enters      bool
	}{
		{"Assaultron Dominator", "P1P1,First Strike,Trample", false},
		{"Boot Nipper", "Deathtouch,Lifelink", true},
		{"Denry Klin, Editor in Chief", "P1P1,First Strike,Vigilance", true},
		{"Ferocious Tigorilla", "Trample,Menace", true},
		{"Flycatcher Giraffid", "Vigilance,Reach", true},
		{"Helica Glider", "Flying,First Strike", true},
		{"Owen Grady, Raptor Trainer", "Menace,Trample,Reach,Haste", false},
		{"Reluctant Role Model", "Flying,Lifelink,P1P1", false},
		{"T-45 Power Armor", "Menace,Trample,Lifelink", false},
		{"Tizerus Charger", "P1P1,Flying", true},
		{"Wingfold Pteron", "Flying,Hexproof", true},
	} {
		card, ok := reg.Lookup(tc.card)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", tc.card)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		c := lowerCard(p, card.Faces[0].Name, card, false)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", tc.card, c.Refusals)
			continue
		}
		ins := choiceInstrs(c)
		if len(ins) != 1 {
			t.Errorf("%s lowers %d PutCounterChoice instructions, want 1", tc.card, len(ins))
			continue
		}
		if ins[0].CS != tc.kinds {
			t.Errorf("%s kinds = %q, want %q", tc.card, ins[0].CS, tc.kinds)
		}
		if enters := ins[0].Obj.Kind == rReplaced; enters != tc.enters {
			t.Errorf("%s as-enters form = %v, want %v", tc.card, enters, tc.enters)
		}
	}
}

// A list kgen cannot carry refuses by name: more than four kinds, a repeated
// kind, an unknown kind, a count above one.
func TestCounterKindChoiceRefusals(t *testing.T) {
	for _, tc := range []struct{ params, want string }{
		{"CounterType$ Flying,Reach,Haste,Menace,Trample | CounterNum$ 1", "choice of 5 counter kinds"},
		{"CounterType$ Flying,Reach,Flying | CounterNum$ 1", `repeats "Flying"`},
		{"CounterType$ Flying,Nonesuch | CounterNum$ 1", `entry "Nonesuch"`},
		{"CounterType$ Flying,Reach | CounterNum$ 2", "CounterNum$"},
	} {
		c := lowerScriptCard(t, "Probe Choice", "Name:Probe Choice\nManaCost:1 W\nTypes:Creature Bird\nPT:1/1\n"+
			"A:AB$ PutCounter | Cost$ T | Defined$ Self | "+tc.params+"\nOracle:x\n")
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Reason)
		}
		if !strings.Contains(strings.Join(reasons, "\n"), tc.want) {
			t.Errorf("%s: refusals %q, want one containing %q", tc.params, reasons, tc.want)
		}
	}
}
