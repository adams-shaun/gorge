package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

const watcherSrc = "Name:Watcher\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ When CARDNAME enters, you gain 1 life.\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

const boltSrc = "Name:Bolt\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Any | TgtPrompt$ Select any target | NumDmg$ 3 | SpellDescription$ Bolt deals 3 damage to any target.\nOracle:x\n"

const shockCreatureSrc = "Name:Shocker\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2 | SpellDescription$ Shocker deals 2 damage to target creature.\nOracle:x\n"

// twoSeatWith builds a two-seat game with one copy of src in seat 0's hand
// and returns the game and that object's id.
func twoSeatWith(t *testing.T, src string) (*state.Game, state.ObjID) {
	t.Helper()
	c, diags := cards.ParseBytes("fixture.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	c.Link()
	g := state.NewGame([]string{"Ann", "Bob"})
	o := g.AddObject(c, 0)
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	return g, o.ID
}

func TestCardViewCarriesPrintingTokenAndManaCost(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Players[0].Battlefield) != 1 {
		t.Fatalf("battlefield %+v", v.Players[0].Battlefield)
	}
	cv := v.Players[0].Battlefield[0]
	if cv.Printing.Name != "Watcher" || cv.Printing.Set != "" || cv.Printing.Number != "" {
		t.Fatalf("printing %+v", cv.Printing)
	}
	if cv.Token != "#1" {
		t.Fatalf("token %q, want #1", cv.Token)
	}
	if cv.ManaCost != "1 W" {
		t.Fatalf("mana cost %q", cv.ManaCost)
	}
}

func TestStackViewKindIsTriggerForATriggerPushObject(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	if g.Obj(id).Face().Triggers[0].Effect == nil {
		t.Fatal("fixture: the T: line's Execute$ did not link to its SVar; check cards/link.go's entry point")
	}
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.TriggerPush, Player: 0, Obj: id, Amount: 0})
	v := Project(g, flatChars{g}, 1, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	sv := v.Stack[0]
	if sv.Kind != "trigger" || sv.Name != "Watcher" || sv.Source != id {
		t.Fatalf("stack view %+v", sv)
	}
	if sv.Text != "When CARDNAME enters, you gain 1 life." {
		t.Fatalf("text %q", sv.Text)
	}
}

func TestStackViewKindIsAbilityForANonTriggerAbilityObject(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	// An ability object whose SA is not one of the source's T: lines — the
	// shape an activated ability will have once the engine enumerates them.
	ab := g.AddObject(nil, 0)
	events.Move(g, ab.ID, state.ZLibrary, state.ZStack)
	ab.Ability = &cards.SA{Kind: "AB", API: "GainLife", Params: map[string]string{"SpellDescription": "Gain 1 life."}}
	ab.Source = id
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 || v.Stack[0].Kind != "ability" || v.Stack[0].Text != "Gain 1 life." {
		t.Fatalf("stack %+v", v.Stack)
	}
}

func TestTargetLabelPrefersTgtPromptThenValidTgts(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{boltSrc, "Select any target"},
		{shockCreatureSrc, "Creature"},
	} {
		g, id := twoSeatWith(t, tc.src)
		events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
		events.Apply(g, events.Event{Kind: events.TargetsChosen, Obj: id, Player: 1, Amount: 1})
		v := Project(g, flatChars{g}, 1, nil)
		if len(v.Stack) != 1 || len(v.Stack[0].Targets) != 1 {
			t.Fatalf("%s: stack %+v", tc.want, v.Stack)
		}
		tg := v.Stack[0].Targets[0]
		if !tg.IsPlayer || tg.Player != 1 || tg.Label != tc.want {
			t.Fatalf("target %+v, want label %q", tg, tc.want)
		}
	}
}

func TestCardViewCarriesCombatRelationships(t *testing.T) {
	g, attacker := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: attacker, From: state.ZHand, To: state.ZBattlefield})
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	blocker := g.AddObject(c, 1).ID
	g.SetZone(state.ZHand, 1, []state.ObjID{blocker})
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: blocker, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	events.Apply(g, events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{attacker, blocker}}})
	v := Project(g, flatChars{g}, 0, nil)
	a := v.Players[0].Battlefield[0]
	if !a.Attacking || a.AttackingPlayer == nil || *a.AttackingPlayer != 1 {
		t.Fatalf("attacker %+v", a)
	}
	if len(a.BlockedBy) != 1 || a.BlockedBy[0] != blocker {
		t.Fatalf("blocked_by %v", a.BlockedBy)
	}
	b := v.Players[1].Battlefield[0]
	if b.Attacking || b.AttackingPlayer != nil || b.BlockedBy != nil {
		t.Fatalf("blocker carries attack state: %+v", b)
	}
	events.Apply(g, events.Event{Kind: events.EndCombatReset})
	if cv := Project(g, flatChars{g}, 0, nil).Players[0].Battlefield[0]; cv.AttackingPlayer != nil || cv.BlockedBy != nil {
		t.Fatalf("combat state survived EndCombatReset: %+v", cv)
	}
}

func TestZoneListsSkipFacelessObjects(t *testing.T) {
	// A resolved triggered ability is parked in exile (CR 608.2m has no
	// "ceases to exist" zone here); it is not a card and must not appear as
	// a nameless entry in anyone's exile list.
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.TriggerPush, Player: 0, Obj: id, Amount: 0})
	ab := g.Stack[0]
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: ab, From: state.ZStack, To: state.ZExile})
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Players[0].Exile) != 0 {
		t.Fatalf("exile shows a faceless object: %+v", v.Players[0].Exile)
	}
	if len(v.Stack) != 0 {
		t.Fatalf("stack still shows the resolved ability: %+v", v.Stack)
	}
}

func TestTargetLabelIsEmptyWhenNothingDeclaresOne(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	events.Apply(g, events.Event{Kind: events.TargetsChosen, Obj: id, Player: 1, Amount: 1})
	v := Project(g, flatChars{g}, 1, nil)
	if got := v.Stack[0].Targets[0].Label; got != "" {
		t.Fatalf("label %q for a creature spell with no ValidTgts", got)
	}
}
