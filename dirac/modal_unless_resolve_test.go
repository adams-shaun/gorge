package dirac

// A marked trigger gorge places unasked (every choice's UnlessCost$ unpayable
// at placement) re-runs the gate at resolution: effCharm calls
// charmGenericChoiceModes again and poses KModes over the payable choices once
// one became payable (a sibling trigger made a Treasure first). The kernel
// kept it with no mode and asked nothing.

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestMarkedTriggerAsksModesAtResolutionOnceAChoiceIsPayable(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ab := kop.Ability{Kind: kop.AbTriggered, NModes: 1, ModesPick: 1, AFlags: kop.AfCharmTrigger}
	ab.Prog = []kop.Instr{
		{Op: kop.OpCharm, C: 0, A: kop.Const(2)},
		{Op: kop.OpUnlessSwitched, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(1), C: int16(kop.PayMana)},
	}
	card := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Marked Trigger", Abilities: []kop.Ability{ab}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	r := &sc.s.res
	r.e = stackEntry{kind: seTrigger, card: card, ctrl: 0}
	a := &tbl.Cards[card].Abilities[0]

	// PRECONDITIONS: the entry is a trigger with no mode chosen, the ability
	// is the marked one, and the controller's pool is empty (unpayable).
	if r.e.kind != seTrigger || r.e.mode != 0 || a.AFlags&kop.AfCharmTrigger == 0 {
		t.Fatalf("precondition: entry kind %v mode %d flags %v", r.e.kind, r.e.mode, a.AFlags)
	}
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("precondition: pool = %v, want empty", sc.s.pl[0].pool)
	}

	// Unpayable at resolution too: no ask, the program runs with no mode.
	if sc.s.modalActAsk(sc.k, a) {
		t.Fatal("posed a mode ask though no choice is payable")
	}

	// Payable by resolution: the mode ask is posed over the payable choice.
	sc.s.pl[0].pool[kop.W] = 3
	if !sc.s.modalActAsk(sc.k, a) {
		t.Fatal("no mode ask at resolution though a choice became payable (gorge effCharm asks)")
	}
	var d Decision
	if !sc.s.askOptionsModalAct(sc.k, &d) || d.Kind != DkModes || d.N != 1 {
		t.Fatalf("mode ask = kind %v, %d options (ok-ness above), want DkModes with 1 option", d.Kind, d.N)
	}

	// An unmarked ability keeps the historic every-mode offer.
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	a.AFlags = 0
	r.ask = raNone
	var d2 Decision
	if !sc.s.modalActAsk(sc.k, a) || !sc.s.askOptionsModalAct(sc.k, &d2) || d2.N != 1 {
		t.Fatalf("unmarked charm offer = %d options, want 1", d2.N)
	}
}

// markedTriggerOnStack builds a table with the marked trigger of the test
// above, puts its entry on the stack with no mode chosen (the state cast.go's
// modesN = 255 arm leaves a trigger placed unasked in), and returns the
// scenario and that card.
func markedTriggerOnStack(t *testing.T) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ab := kop.Ability{Kind: kop.AbTriggered, NModes: 1, ModesPick: 1, AFlags: kop.AfCharmTrigger}
	ab.Prog = []kop.Instr{
		{Op: kop.OpCharm, C: 0, A: kop.Const(2)},
		{Op: kop.OpUnlessSwitched, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(1), C: int16(kop.PayMana)},
	}
	card := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Marked Trigger", Abilities: []kop.Ability{ab}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.stack[0] = stackEntry{kind: seTrigger, obj: NoObj, card: card, ctrl: 0}
	sc.s.sp = 1
	return sc, card
}

// TestResolveTopAsksMarkedTriggerModes drives resolveTop itself: the marked
// trigger on the stack with no mode poses DkModes (one option) once a choice
// is payable, and runs with no ask while none is.
func TestResolveTopAsksMarkedTriggerModes(t *testing.T) {
	// Payable: the controller holds the mana.
	sc, card := markedTriggerOnStack(t)
	if e := &sc.s.stack[0]; e.kind != seTrigger || e.mode != 0 || sc.k.t.Cards[card].Abilities[0].AFlags&kop.AfCharmTrigger == 0 {
		t.Fatalf("precondition: entry kind %v mode %d, marked %v", e.kind, e.mode, sc.k.t.Cards[card].Abilities[0].AFlags)
	}
	sc.s.pl[0].pool[kop.W] = 3
	sc.s.resolveTop(sc.k)
	d := sc.s.Step()
	if d.Kind != DkModes || d.N != 1 {
		t.Fatalf("payable: decision kind %v with %d options, want DkModes with 1 (resolveTop must pose the mode ask)", d.Kind, d.N)
	}

	// Unpayable: no ask, the program runs with no mode.
	sc, _ = markedTriggerOnStack(t)
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("precondition: pool = %v, want empty", sc.s.pl[0].pool)
	}
	sc.s.resolveTop(sc.k)
	if sc.s.res.ask == raGenModalMode {
		t.Fatal("unpayable: resolveTop posed a mode ask though no choice is payable")
	}
	if d := sc.s.Step(); d.Kind == DkModes {
		t.Fatalf("unpayable: decision is DkModes with %d options, want none", d.N)
	}
}
