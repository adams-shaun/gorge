package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func hasCommanderRejectionNote(e *Engine) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "commander configuration rejected under CR 903") {
			return true
		}
	}
	return false
}

// TestChooseABackgroundSeatsBothCommanders: a Choose-a-Background card and a
// legendary Background seat as a pair (CR 702.124k); the Background alone
// seats nothing.
func TestChooseABackgroundSeatsBothCommanders(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	veteran := lookup(t, reg, "Veteran Soldier")
	if commanderCardLegal(veteran) {
		t.Fatal("fixture drift: Veteran Soldier is commander-legal alone")
	}
	for _, name := range []string{"Karlach, Fury of Avernus", "Lae'zel, Vlaakith's Champion", "Wyll, Blade of Frontiers"} {
		t.Run(name, func(t *testing.T) {
			carrier := lookup(t, reg, name)
			if !carrier.Faces[0].HasKeyword("Choose a Background") {
				t.Fatalf("fixture drift: %s lacks the keyword", name)
			}
			if !commanderPairOK(carrier, veteran) || !commanderPairOK(veteran, carrier) {
				t.Fatalf("commanderPairOK(%s, Veteran Soldier) must hold both ways", name)
			}
			e := New(commanderConfig(t, reg, []*cards.Card{carrier, veteran}, []int{0, 1}))
			e.Advance()
			if hasCommanderRejectionNote(e) {
				t.Fatal("the pair was rejected at seating")
			}
			cmds := e.G.Players[0].Commanders
			if len(cmds) != 2 {
				t.Fatalf("seat 0 has %d commanders, want 2", len(cmds))
			}
			for i, id := range cmds {
				if o := e.G.Obj(id); o == nil || o.Zone != state.ZCommand {
					t.Fatalf("commander %d is not in the command zone (obj %v)", i, o)
				}
			}
			toMain1(t, e)
			addMana(t, e, 0, "RRRRRWWW")
			var gotCarrier, gotVeteran bool
			for _, o := range e.Pending().Options {
				if o.Kind == "cast" && o.Label == "Cast "+carrier.Faces[0].Name {
					gotCarrier = true
				}
				if o.Kind == "cast" && o.Label == "Cast Veteran Soldier" {
					gotVeteran = true
				}
			}
			if !gotCarrier || !gotVeteran {
				t.Fatalf("both commanders must be offered as casts (carrier=%v veteran=%v): %+v", gotCarrier, gotVeteran, e.Pending().Options)
			}
		})
	}

	// Guard: the Background alone seats nothing.
	e := New(commanderConfig(t, reg, []*cards.Card{veteran}, []int{0}))
	e.Advance()
	if n := len(e.G.Players[0].Commanders); n != 0 {
		t.Fatalf("a lone Background seated %d commanders, want 0", n)
	}
	if !hasCommanderRejectionNote(e) {
		t.Fatal("a lone Background must emit the CR 903 rejection Note")
	}
}

// TestChooseABackgroundClearsTheKeywordGap: the keyword is registered, so the
// registry stops reporting it. Wyll's remaining repl:RollDice is owned by
// another ticket and deliberately not asserted either way.
func TestChooseABackgroundClearsTheKeywordGap(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	if !sup["kw:Choose a Background"] {
		t.Fatal("kw:Choose a Background is not registered")
	}
	for _, name := range []string{"Karlach, Fury of Avernus", "Lae'zel, Vlaakith's Champion", "Wyll, Blade of Frontiers"} {
		c := lookup(t, reg, name)
		if !c.Faces[0].HasKeyword("Choose a Background") {
			t.Fatalf("fixture drift: %s lacks the keyword", name)
		}
		un := reg.Unsupported(c, sup)
		for _, u := range un {
			if u == "kw:Choose a Background" {
				t.Fatalf("%s still reports kw:Choose a Background", name)
			}
		}
		if name != "Wyll, Blade of Frontiers" && len(un) != 0 {
			t.Fatalf("%s still reports unsupported %v", name, un)
		}
	}
}
