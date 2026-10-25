package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const prototypeSeraphSrc = `Name:Steel Seraph
ManaCost:6
Types:Artifact Creature Angel
PT:5/5
K:Flying
K:Prototype:1 W W:3:3
Oracle:x
`

const prototypeBoltSrc = `Name:Proto Bolt
ManaCost:4 R
Types:Instant
K:Prototype:R:1:1
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ x
Oracle:x
`

func prototypeCastOption(t *testing.T, e *Engine, id state.ObjID, mode string) decision.Option {
	t.Helper()
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return o
		}
	}
	t.Fatalf("no cast option for %d in mode %q: %+v", id, mode, e.legalActions(0))
	return decision.Option{}
}

func prototypeOnStack(t *testing.T, e *Engine, id state.ObjID) *state.Object {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZStack {
		t.Fatalf("object %d is in %v, want the stack", id, o)
	}
	return o
}

func TestPrototypeOffersInsetCast(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 71, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	prototypeCastOption(t, e, id, "prototype")
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "" {
			t.Fatalf("Steel Seraph offered its normal {6} cast with only {1}{W}{W}: %+v", o)
		}
	}
	addMana(t, e, 0, "CCC")
	prototypeCastOption(t, e, id, "prototype")
	prototypeCastOption(t, e, id, "")
}

func TestPrototypeCharacteristicsOnStackAndBattlefield(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 72, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)

	o := prototypeOnStack(t, e, id)
	if mv := state.ManaValue(e.G, o); mv != 3 {
		t.Errorf("prototyped Steel Seraph on the stack has mana value %d, want 3", mv)
	}
	if p, tt := e.Power(id), e.Toughness(id); p != 3 || tt != 3 {
		t.Errorf("prototyped Steel Seraph on the stack is %d/%d, want 3/3", p, tt)
	}
	if got := e.Colors(id); got != "W" {
		t.Errorf("prototyped Steel Seraph on the stack is %q, want W", got)
	}
	if !e.HasKeyword(id, "Flying") {
		t.Error("prototyped Steel Seraph lost flying")
	}
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Errorf("prototype cast did not pay {1}{W}{W}: pool %+v", p)
	}

	e.resolveTop()
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Steel Seraph resolved to %v, want battlefield", o.Zone)
	}
	if mv := state.ManaValue(e.G, o); mv != 3 {
		t.Errorf("prototyped Steel Seraph permanent has mana value %d, want 3", mv)
	}
	if p, tt := e.Power(id), e.Toughness(id); p != 3 || tt != 3 {
		t.Errorf("prototyped Steel Seraph permanent is %d/%d, want 3/3", p, tt)
	}
	if got := e.Colors(id); got != "W" {
		t.Errorf("prototyped Steel Seraph permanent is %q, want W", got)
	}
	replayCheck(t, e, cfg)
}

func TestPrototypeNormalCastStaysPrinted(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 73, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "CCCCCC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "").Index)

	o := prototypeOnStack(t, e, id)
	if mv := state.ManaValue(e.G, o); mv != 6 {
		t.Errorf("normally cast Steel Seraph has mana value %d, want 6", mv)
	}
	if p, tt := e.Power(id), e.Toughness(id); p != 5 || tt != 5 {
		t.Errorf("normally cast Steel Seraph on the stack is %d/%d, want 5/5", p, tt)
	}
	if got := e.Colors(id); got != "" {
		t.Errorf("normally cast Steel Seraph is %q, want colourless", got)
	}
	e.resolveTop()
	if p, tt := e.Power(id), e.Toughness(id); p != 5 || tt != 5 {
		t.Errorf("normally cast Steel Seraph permanent is %d/%d, want 5/5", p, tt)
	}
	replayCheck(t, e, cfg)
}

func TestPrototypeNormalElsewhere(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 74, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	e.resolveTop()

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("Steel Seraph in %v, want graveyard", o.Zone)
	}
	if mv := state.ManaValue(e.G, e.G.Obj(id)); mv != 6 {
		t.Errorf("prototyped Steel Seraph in the graveyard has mana value %d, want 6", mv)
	}
	if p, tt := e.Power(id), e.Toughness(id); p != 5 || tt != 5 {
		t.Errorf("prototyped Steel Seraph in the graveyard is %d/%d, want 5/5", p, tt)
	}
	if got := e.Colors(id); got != "" {
		t.Errorf("prototyped Steel Seraph in the graveyard is %q, want colourless", got)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZHand})
	addMana(t, e, 0, "CCCCCC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "").Index)
	e.resolveTop()
	if p, tt := e.Power(id), e.Toughness(id); p != 5 || tt != 5 {
		t.Errorf("recast Steel Seraph permanent is %d/%d, want 5/5", p, tt)
	}
	replayCheck(t, e, cfg)
}

func TestPrototypeCopyIsPrototyped(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 75, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	prototypeOnStack(t, e, id)

	e.emit(events.Event{Kind: events.StackCopy, Obj: id, Player: 0})
	if len(e.G.Stack) < 2 {
		t.Fatalf("StackCopy did not add a copy: stack %v", e.G.Stack)
	}
	copyID := e.G.Stack[len(e.G.Stack)-1]
	copied := e.G.Obj(copyID)
	if !copied.IsCopy {
		t.Fatalf("top stack object %d is not a copy", copyID)
	}
	if copied.CastFlags&state.FlagPrototype == 0 {
		t.Errorf("copied prototyped spell has CastFlags %#x, want FlagPrototype", copied.CastFlags)
	}
	if mv := state.ManaValue(e.G, copied); mv != 3 {
		t.Errorf("copied prototyped spell has mana value %d, want 3", mv)
	}
	if p, tt := e.Power(copyID), e.Toughness(copyID); p != 3 || tt != 3 {
		t.Errorf("copied prototyped spell is %d/%d, want 3/3", p, tt)
	}
	if got := e.Colors(copyID); got != "W" {
		t.Errorf("copied prototyped spell is %q, want W", got)
	}

	e.resolveTop()
	e.resolveTop()
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("prototyped Steel Seraph resolved to %v, want battlefield", o.Zone)
	}
	e.emit(events.Event{Kind: events.CopyToken, Player: 0, Obj: id})
	tokenID := e.G.NextID - 1
	e.emit(events.Event{Kind: events.MoveZone, Obj: tokenID, From: state.ZLibrary, To: state.ZBattlefield})
	token := e.G.Obj(tokenID)
	if token == nil || token.Zone != state.ZBattlefield {
		t.Fatalf("token copy did not reach the battlefield: %+v", token)
	}
	if mv := state.ManaValue(e.G, token); mv != 3 {
		t.Errorf("token copy of a prototyped permanent has mana value %d, want 3", mv)
	}
	if p, tt := e.Power(tokenID), e.Toughness(tokenID); p != 3 || tt != 3 {
		t.Errorf("token copy of a prototyped permanent is %d/%d, want 3/3", p, tt)
	}
	if got := e.Colors(tokenID); got != "W" {
		t.Errorf("token copy of a prototyped permanent is %q, want W", got)
	}
	replayCheck(t, e, cfg)
}

func TestPrototypeReversalClears(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 76, prototypeBoltSrc)
	addToHand(t, e, 0, prototypeBoltSrc)
	addMana(t, e, 0, "R")
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id, Mode: "prototype"})
	o := prototypeOnStack(t, e, id)
	if o.CastFlags&state.FlagPrototype == 0 {
		t.Fatalf("prototyped cast was not stamped before payment: CastFlags %#x", o.CastFlags)
	}
	if mv := state.ManaValue(e.G, o); mv != 1 {
		t.Errorf("prototyped Proto Bolt on the stack has mana value %d, want 1", mv)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("prototyped Proto Bolt did not ask for a target: %+v", d)
	}
	e.emit(events.Event{Kind: events.ManaClear, Player: 0})
	submitChoices(t, e, d.Options[0].Index)

	o = e.G.Obj(id)
	if o.Zone != state.ZHand {
		t.Fatalf("reversed prototyped cast left the card in %v, want hand", o.Zone)
	}
	if o.CastFlags&state.FlagPrototype != 0 {
		t.Errorf("reversed prototyped cast kept CastFlags %#x", o.CastFlags)
	}
	if mv := state.ManaValue(e.G, o); mv != 5 {
		t.Errorf("Proto Bolt back in hand has mana value %d, want 5", mv)
	}
	replayCheck(t, e, cfg)
}
