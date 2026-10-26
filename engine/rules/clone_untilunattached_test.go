package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestCloneUntilUnattachedEndsOnReattach(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Assimilation Aegis"), lookup(t, reg, "Llanowar Elves"), lookup(t, reg, "Serra Angel")}, mountainDeck(t, 37)...),
			append([]*cards.Card{lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Hill Giant")}, mountainDeck(t, 38)...),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	aeg := findAndMoveToHand(t, e, 0, "Assimilation Aegis")
	elf := findAndMoveToHand(t, e, 0, "Llanowar Elves")
	moveToBattlefield(t, e, elf)
	bear := findAndMoveToHand(t, e, 1, "Grizzly Bears")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	moveToBattlefield(t, e, bear)
	e.pending = nil
	e.Advance()
	dump := func(label string) {
		d := e.Pending()
		if d == nil {
			t.Logf("%s: no pending", label)
			return
		}
		t.Logf("%s: kind=%s resume=%q min=%d max=%d prompt=%q", label, d.Kind, d.ResumeKind, d.Min, d.Max, d.Prompt)
		for _, o := range d.Options {
			if o.Kind == "pass" || o.Kind == "play_land" {
				continue
			}
			t.Logf("   opt %d kind=%s obj=%d label=%q mode=%q", o.Index, o.Kind, o.Obj, o.Label, o.Mode)
		}
	}
	addMana(t, e, 0, "WUW")
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == aeg {
			submitChoices(t, e, o.Index)
			break
		}
	}
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		dump("loop")
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
			continue
		}
		// choose the bear for target
		pick := 0
		for _, o := range d.Options {
			if o.Obj == bear {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	t.Logf("bear zone %v exiledWith=%d; aegis zone %v", e.G.Obj(bear).Zone, e.G.Obj(bear).ExiledWith, e.G.Obj(aeg).Zone)
	// now equip
	addMana(t, e, 0, "WW")
	dump("main")
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == aeg {
			submitChoices(t, e, o.Index)
			break
		}
	}
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		dump("equip")
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
			continue
		}
		pick := 0
		for _, o := range d.Options {
			if o.Obj == elf {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	o := e.G.Obj(elf)
	t.Logf("elf face %q attached=%d", o.Face().Name, e.G.Obj(aeg).AttachedTo)
	// move the equipment to a second creature the same turn
	ang := findAndMoveToHand(t, e, 0, "Serra Angel")
	moveToBattlefield(t, e, ang)
	e.pending = nil
	e.Advance()
	addMana(t, e, 0, "WW")
	for _, op := range e.Pending().Options {
		if op.Kind == "ability" && op.Obj == aeg {
			submitChoices(t, e, op.Index)
			break
		}
	}
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			for _, op := range d.Options {
				if op.Kind == "pass" {
					submitChoices(t, e, op.Index)
					break
				}
			}
			continue
		}
		pick := 0
		for _, op := range d.Options {
			if op.Obj == ang {
				pick = op.Index
			}
		}
		submitChoices(t, e, pick)
	}
	// Duration$ UntilUnattached: the first bearer stops being a copy the
	// moment the Equipment moves on, with no wait for cleanup.
	if got := e.G.Obj(aeg).AttachedTo; got != ang {
		t.Fatalf("Aegis attached to %d, want the Angel %d", got, ang)
	}
	if n := e.Derived(elf).Name; n != "Llanowar Elves" {
		t.Errorf("first bearer is %q after re-equip, want itself", n)
	}
	if n := e.Derived(ang).Name; n != "Grizzly Bears" {
		t.Errorf("second bearer is %q, want the copy Grizzly Bears", n)
	}
}
