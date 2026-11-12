package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A source that left the battlefield and came back with its ability on the stack is
// a new object with no relation to that ability (CR 400.7, 113.7a), so "another
// creature" can name it. Forge (Card.equals, CardProperty "Other") and XMage
// (AnotherPredicate) compare a card identity that survives the move and exclude it
// (read, not run); the rule is the CR's.

// attackingWithTrigger declares a ready creature attacking, its trigger on the stack.
func attackingWithTrigger(t *testing.T, c *cards.Card) (*Engine, state.ObjID) {
	t.Helper()
	e := New(Config{Seed: 717, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	src := onBoardReadyCard(t, e, 0, c)
	e.G.Active, e.G.Step = 0, state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, src)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v after attacking, want the attack trigger", e.G.Stack)
	}
	return e, src
}

func flicker(e *Engine, id state.ObjID) {
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
}

func TestGourmandFlickeredWithItsTriggerOnTheStackIsAnotherCreature(t *testing.T) {
	e, g := attackingWithTrigger(t, corpusAlternativeCard(t, "Vampire Gourmand"))
	flicker(e, g)
	d := passUntilNonPriority(t, e, 8)
	if d == nil {
		t.Fatal("the trigger's cost window never opened")
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	chooseKindOption(t, e, d, "trigger_cost_pay")
	if z := e.G.Obj(g).Zone; z != state.ZGraveyard {
		t.Fatalf("the returned Gourmand is in %v after paying with it", z)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand %d after paying, want %d", got, hand+1)
	}
}

// A cost naming another permanent can be paid only by the returned source here, and a
// cost naming the source only by the source that stayed.
func TestCostReadsAReturnedSourceAsANewObject(t *testing.T) {
	const head = "Name:Tester\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nT:Mode$ Attacks | ValidCard$ Card.Self | Execute$ Body | TriggerDescription$ x\n"
	for _, tc := range []struct {
		name, body string
		self       bool
	}{
		{"sacrifice another", "AB$ GainLife | Cost$ Sac<1/Creature.Other/another creature> | LifeAmount$ 1", false},
		{"tap another", "AB$ GainLife | Cost$ tapXType<1/Creature.Other/another creature> | LifeAmount$ 1", false},
		{"return another", "AB$ GainLife | Cost$ Return<1/Creature.Other/another creature> | LifeAmount$ 1", false},
		{"unless return another", "DB$ LoseLife | LifeAmount$ 1 | UnlessCost$ Return<1/Creature.Other/another creature> | UnlessPayer$ You", false},
		{"sacrifice this", "AB$ GainLife | Cost$ Sac<1/CARDNAME> | LifeAmount$ 1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := card(t, head+"SVar:Body:"+tc.body+"\nOracle:x\n")
			for _, returned := range []bool{false, true} {
				e, src := attackingWithTrigger(t, c)
				if returned {
					flicker(e, src)
				}
				d := passUntilNonPriority(t, e, 8)
				if d == nil {
					t.Fatalf("returned %v: no ask", returned)
				}
				declineOnly := len(d.Options) == 1 && (d.Options[0].Kind == "trigger_cost_decline" || d.Options[0].Mode == "unless_decline")
				if payable := !declineOnly; payable == (returned == tc.self) {
					t.Fatalf("returned %v: payable %v at %+v", returned, payable, d.Options)
				}
			}
		})
	}
}

func TestGourmandThatStayedIsNotAnotherCreature(t *testing.T) {
	e, _ := attackingWithTrigger(t, corpusAlternativeCard(t, "Vampire Gourmand"))
	d := passUntilNonPriority(t, e, 8)
	if d == nil || len(d.Options) != 1 || d.Options[0].Kind != "trigger_cost_decline" {
		t.Fatalf("the cost window = %+v, want decline only", d)
	}
}

// A resolving filter reads the same way: "other creatures you control" reaches the
// returned source, and "this" does not name it.
func TestResolvingFilterReadsAReturnedSourceAsANewObject(t *testing.T) {
	c := corpusAlternativeCard(t, "Syr Alin, the Lion's Claw")
	for _, returned := range []bool{false, true} {
		e, src := attackingWithTrigger(t, c)
		if returned {
			flicker(e, src)
		}
		base := e.Power(src)
		passUntilStackEmpty(t, e, 8)
		if got, want := e.Power(src)-base, map[bool]int32{false: 0, true: 1}[returned]; int32(got) != want {
			t.Fatalf("returned=%v: Syr Alin's power moved by %d, want %d", returned, got, want)
		}
	}
}

// A connive trigger whose source left and came back is about the old permanent,
// which is gone: its controller still draws and discards, and the new permanent
// gets no counter (CR 701.50a, 701.50b, 400.7).
func TestConniveCounterSkipsAReturnedSource(t *testing.T) {
	c := card(t, "Name:Tester\nManaCost:1 B\nTypes:Creature Human\nPT:2/2\nT:Mode$ Attacks | ValidCard$ Card.Self | Execute$ Body | TriggerDescription$ x\nSVar:Body:DB$ Connive\nOracle:x\n")
	for _, returned := range []bool{false, true} {
		e := New(Config{Seed: 717, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{{c, c, c, c, c, c, c, c, c, c, c, c, c, c, c, c, c, c, c, c}, mountainDeck(t, 40)}})
		src := onBoardReadyCard(t, e, 0, c)
		e.G.Active, e.G.Step = 0, state.StepDeclareAttackers
		e.askAttackers()
		submitAttackersOnly(t, e, src)
		if returned {
			flicker(e, src)
		}
		for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		passUntilStackEmpty(t, e, 8)
		want := int32(1)
		if returned {
			want = 0
		}
		if got := counterCount(e.G.Obj(src), "P1P1"); got != want {
			t.Fatalf("returned=%v: %d +1/+1 counters, want %d", returned, got, want)
		}
	}
}
