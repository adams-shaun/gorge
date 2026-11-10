package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Undying and Exalted have no KwMask bit: kgen lowers both from gorge's
// expansion into ordinary trigger lines, so a creature with either printed on
// the battlefield projects without a "no kernel bit" error and is compared
// through the zone moves, counters and pumps its triggers make.
func TestProjCharsExpandedUndyingExalted(t *testing.T) {
	t.Parallel()

	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, n := range []string{"Butcher Ghoul", "Battlegrace Angel"} {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		deck = append(deck, c, c)
	}
	e := rules.New(rules.Config{Seed: 1, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{deck, deck}, Tokens: reg.Tokens})
	v := newCharVocab(&kop.Table{})

	seen := map[string]bool{}
	var lib []state.ObjID
	for i := range e.G.Objs {
		lib = append(lib, e.G.Objs[i].ID)
	}
	if len(lib) == 0 {
		t.Fatal("no game objects")
	}
	for _, id := range lib {
		o := e.G.Obj(id)
		name := cardName(o.Card)
		e.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
		if got := e.G.Obj(o.ID); got == nil || got.Zone != state.ZBattlefield {
			t.Fatalf("%s did not reach the battlefield", name)
		}
		kw := ""
		for _, k := range e.Derived(o.ID).Keywords {
			if k == "Undying" || k == "Exalted" {
				kw = k
			}
		}
		if kw == "" {
			t.Fatalf("%s carries neither Undying nor Exalted: %v", name, e.Derived(o.ID).Keywords)
		}
		seen[kw] = true
		if _, msg := gorgeChars(e, v, o.ID); msg != "" {
			t.Errorf("%s (%s): %s", name, kw, msg)
		}
	}
	if !seen["Undying"] || !seen["Exalted"] {
		t.Fatalf("fixture too weak: keywords seen %v", seen)
	}
}
