package rules

// cr-g137: three spell-ability condition shapes that used to fail closed with
// an `unmodelled condition` Note: Condition$ EnduringStory (CR 702.195),
// ConditionDefined$ Enchanted/Equipped (CR 303.4) and the DoubleFaced filter
// word (CR 712.1).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func unmodelledNotes(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.HasPrefix(ev.Text, "unmodelled condition") {
			n++
		}
	}
	return n
}

func gateSource(gate string) string {
	return "Name:Gate Src\nManaCost:1\nTypes:Artifact\n" +
		"SVar:DB:DB$ GainLife | Defined$ You | LifeAmount$ 3 | " + gate + "\nOracle:x\n"
}

func TestConditionEnduringStoryGatesSub(t *testing.T) {
	t.Parallel()
	run := func(story bool) (int, int) {
		e := mayPlayBase(t)
		id := onBoardGrant(t, e, 0, gateSource("Condition$ EnduringStory"))
		if story {
			e.emit(events.Event{Kind: events.EnduringStoryChange, Player: 0, Text: "enduring story"})
		}
		if e.G.Players[0].EnduringStory != story {
			t.Fatalf("precondition: EnduringStory = %v, want %v", e.G.Players[0].EnduringStory, story)
		}
		before := e.G.Players[0].Life
		f := e.G.Obj(id).Face()
		effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0, SVars: f.SVars},
			cards.ResolveSVar(f.SVars, "DB"))
		return int(e.G.Players[0].Life - before), unmodelledNotes(e)
	}
	if d, n := run(true); d != 3 || n != 0 {
		t.Fatalf("with the story: life delta %d notes %d, want 3 and 0", d, n)
	}
	if d, n := run(false); d != 0 || n != 0 {
		t.Fatalf("without the story: life delta %d notes %d, want 0 and 0", d, n)
	}
}

func TestBalinEnduringStoryConditionModelled(t *testing.T) {
	t.Parallel()
	balin := corpusCard(t, "Balin, Loremaster")
	sa := cards.ResolveSVar(balin.Faces[0].SVars, "DBDamage")
	if sa == nil {
		t.Fatal("precondition: Balin has no DBDamage SVar")
	}
	if k, bad := effects.UnmodelledCondition(sa); bad {
		t.Fatalf("Balin's DBDamage is still unmodelled: %s", k)
	}
}

func TestAirtightAlibiUnsuspectsEnchantedCreature(t *testing.T) {
	t.Parallel()
	alibi := corpusCard(t, "Airtight Alibi")
	e, cfg, bear := staticGainControlGame(t, 702157, []*cards.Card{alibi}, 2)
	e.emit(events.Event{Kind: events.AlterAttribute, Obj: bear, Text: "Suspected", Amount: 1})
	if !e.G.Obj(bear).Suspected {
		t.Fatal("precondition: the Bear is not suspected before the Aura")
	}
	aura := findAndMoveToHand(t, e, 0, "Airtight Alibi")
	addMana(t, e, 0, "GGGGG")
	castFromPriority(t, e, aura)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(aura).AttachedTo; got != bear {
		t.Fatalf("precondition: AttachedTo = %d, want %d", got, bear)
	}
	if e.G.Obj(bear).Suspected {
		t.Fatal("the enchanted Bear is still suspected after Airtight Alibi's enters trigger")
	}
	replayCheck(t, e, cfg)
}

func TestConditionDefinedEquippedRing(t *testing.T) {
	t.Parallel()
	ring := corpusCard(t, "Ring of Valkas")
	run := func(color string) (int, int) {
		e := layerEngine(t)
		rid := onBoardCard(t, e, 0, ring)
		cid := onBoard(t, e, 0, "Name:Host\nManaCost:1 "+color+"\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		e.emit(events.Event{Kind: events.Attach, Obj: rid, IDs: []state.ObjID{cid}})
		if e.G.Obj(rid).AttachedTo != cid {
			t.Fatal("precondition: the Ring is not attached")
		}
		f := e.G.Obj(rid).Face()
		effects.Resolve(e, &effects.Ctx{Source: rid, Controller: 0, SVars: f.SVars},
			cards.ResolveSVar(f.SVars, "ValkasCounter"))
		n := 0
		for _, c := range e.G.Obj(cid).Counters {
			if c.Kind == "P1P1" {
				n += int(c.N)
			}
		}
		return n, unmodelledNotes(e)
	}
	if n, notes := run("R"); n != 1 || notes != 0 {
		t.Fatalf("red host: counters %d notes %d, want 1 and 0", n, notes)
	}
	if n, notes := run("G"); n != 0 || notes != 0 {
		t.Fatalf("green host: counters %d notes %d, want 0 and 0", n, notes)
	}
}

func TestDoubleFacedFilterWord(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	match := func(c *cards.Card, zone state.Zone) bool {
		o := e.G.AddObject(c, 0)
		o.Zone = zone
		return effects.MatchesSpec(e.G, "Card.DoubleFaced", o.ID, 0)
	}
	net := corpusCard(t, "Braided Net")
	if !match(net, state.ZHand) || !match(net, state.ZBattlefield) {
		t.Fatal("Braided Net (transforming) should match in hand and on the battlefield")
	}
	if !match(corpusCard(t, "Agadeem's Awakening"), state.ZHand) {
		t.Fatal("Agadeem's Awakening (modal) should match")
	}
	if match(card(t, creatureSrc("Plain")), state.ZHand) {
		t.Fatal("a plain card must not match")
	}
	if match(corpusCard(t, "Bruna, the Fading Light"), state.ZHand) {
		t.Fatal("a meld card must not match")
	}
	if u := effects.UnknownPredicates("Card.DoubleFaced"); len(u) != 0 {
		t.Fatalf("DoubleFaced still unknown: %v", u)
	}
}
