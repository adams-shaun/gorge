package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const prototypeBlueSrc = `Name:Proto Blue
ManaCost:6
Types:Artifact Creature
PT:5/5
K:Prototype:1 U U:3:3
Oracle:x
`

func prototypeHasCastOption(e *Engine, id state.ObjID, mode string) bool {
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return true
		}
	}
	return false
}

// TestPrototypeCostModifierUsesInsetManaValue: Krosan Drover's reduction is
// keyed to mana value 6 or greater. A prototyped Steel Seraph has inset mana
// value 3, so the reduction must not apply: {1}{W}{W} is the full price.
func TestPrototypeCostModifierUsesInsetManaValue(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 8101, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	onBoardCard(t, e, 0, corpusCard(t, "Krosan Drover"))
	addMana(t, e, 0, "WW")
	if prototypeHasCastOption(e, id, "prototype") {
		t.Fatal("Krosan Drover reduced a prototype cast priced at inset mana value 3")
	}
	addMana(t, e, 0, "C")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	o := prototypeOnStack(t, e, id)
	if mv := state.ManaValue(e.G, o); mv != 3 {
		t.Errorf("prototyped Steel Seraph has mana value %d, want 3", mv)
	}
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Errorf("prototype cast did not pay {1}{W}{W}: pool %+v", p)
	}
}

// TestPrototypeCantBeCastUsesInsetManaValue: Void Winnower stops even-mana-
// value spells. The printed Steel Seraph is 6 (even), but the prototype cast
// is 3 (odd), so only the prototype mode is offered.
func TestPrototypeCantBeCastUsesInsetManaValue(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 8102, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	onBoardCard(t, e, 1, corpusCard(t, "Void Winnower"))
	addMana(t, e, 0, "WWC")
	prototypeCastOption(t, e, id, "prototype")
	addMana(t, e, 0, "CCCCCC")
	if prototypeHasCastOption(e, id, "") {
		t.Fatal("Void Winnower allowed a printed mana value 6 cast")
	}
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	if mv := state.ManaValue(e.G, e.G.Obj(id)); mv != 3 {
		t.Errorf("prototyped Steel Seraph has mana value %d, want 3", mv)
	}
}

// TestPrototypeCantBeCastUsesInsetColour: Llawan stops opponents' blue
// creature spells. The printed Proto Blue is colourless, but its prototype
// cost contains {U}, so the prototype cast is blue and withheld while the
// colourless printed cast is not.
func TestPrototypeCantBeCastUsesInsetColour(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 8103, prototypeBlueSrc)
	addToHand(t, e, 0, prototypeBlueSrc)
	onBoardCard(t, e, 1, corpusCard(t, "Llawan, Cephalid Empress"))
	addMana(t, e, 0, "UUC")
	if prototypeHasCastOption(e, id, "prototype") {
		t.Fatal("Llawan allowed a blue prototype creature spell")
	}
	addMana(t, e, 0, "CCCCCC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "").Index)
	o := prototypeOnStack(t, e, id)
	if got := e.Colors(id); got != "" {
		t.Errorf("normally cast Proto Blue is %q, want colourless", got)
	}
	if mv := state.ManaValue(e.G, o); mv != 6 {
		t.Errorf("normally cast Proto Blue has mana value %d, want 6", mv)
	}
}

// TestPrototypeCopyExceptOverridesPT: a copy of a prototyped permanent whose
// CopyExcept sets P/T uses the exception's P/T, not the prototype's 3/3,
// while keeping the prototype's mana value and colour.
func TestPrototypeCopyExceptOverridesPT(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 8104, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	e.resolveTop()
	e.emit(events.Event{Kind: events.CopyToken, Player: 0, Obj: id})
	tokenID := e.G.NextID - 1
	e.emit(events.Event{Kind: events.MoveZone, Obj: tokenID, From: state.ZLibrary, To: state.ZBattlefield})
	x := events.CopyExceptions{PTSet: true, Power: 4, Toughness: 4}
	e.emit(events.Event{Kind: events.CopyExcept, Obj: tokenID, Text: x.Text()})
	if p, tt := e.Power(tokenID), e.Toughness(tokenID); p != 4 || tt != 4 {
		t.Errorf("PTSet copy is %d/%d, want 4/4", p, tt)
	}
	if mv := state.ManaValue(e.G, e.G.Obj(tokenID)); mv != 3 {
		t.Errorf("PTSet copy has mana value %d, want the prototype's 3", mv)
	}
	if got := e.Colors(tokenID); got != "W" {
		t.Errorf("PTSet copy is %q, want W", got)
	}
	replayCheck(t, e, cfg)
}

// TestPrototypeCopyExceptNoManaCost: a copy with no mana cost has mana value
// 0 even when the copied permanent is prototyped.
func TestPrototypeCopyExceptNoManaCost(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 8105, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	e.resolveTop()
	e.emit(events.Event{Kind: events.CopyToken, Player: 0, Obj: id})
	tokenID := e.G.NextID - 1
	e.emit(events.Event{Kind: events.MoveZone, Obj: tokenID, From: state.ZLibrary, To: state.ZBattlefield})
	x := events.CopyExceptions{NoManaCost: true}
	e.emit(events.Event{Kind: events.CopyExcept, Obj: tokenID, Text: x.Text()})
	if mv := state.ManaValue(e.G, e.G.Obj(tokenID)); mv != 0 {
		t.Errorf("no-mana-cost copy has mana value %d, want 0", mv)
	}
	if p, tt := e.Power(tokenID), e.Toughness(tokenID); p != 3 || tt != 3 {
		t.Errorf("no-mana-cost copy is %d/%d, want 3/3", p, tt)
	}
	replayCheck(t, e, cfg)
}

// TestPrototypeMyriadTokenKeepsPrototype: a Myriad token copies the
// prototype designation and therefore the inset characteristics.
func TestPrototypeMyriadTokenKeepsPrototype(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureDeck(t, 8106, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, id, "prototype").Index)
	e.resolveTop()
	e.emit(events.Event{Kind: events.MyriadCopy, Player: 1, Obj: id})
	tokenID := e.G.NextID - 1
	e.emit(events.Event{Kind: events.MoveZone, Obj: tokenID, From: state.ZLibrary, To: state.ZBattlefield})
	token := e.G.Obj(tokenID)
	if token == nil {
		t.Fatal("Myriad token was not minted")
	}
	if token.CastFlags&state.FlagPrototype == 0 {
		t.Errorf("Myriad token CastFlags %#x, want FlagPrototype", token.CastFlags)
	}
	if p, tt := e.Power(tokenID), e.Toughness(tokenID); p != 3 || tt != 3 {
		t.Errorf("Myriad token is %d/%d, want 3/3", p, tt)
	}
	if mv := state.ManaValue(e.G, token); mv != 3 {
		t.Errorf("Myriad token has mana value %d, want 3", mv)
	}
	if got := e.Colors(tokenID); got != "W" {
		t.Errorf("Myriad token is %q, want W", got)
	}
	replayCheck(t, e, cfg)
}

// TestPrototypePlannerUsesInsetManaValue: the payment planner composes a
// prototype cast with the same CR 601.2f cost modifiers as the offer gate, so
// Krosan Drover cannot make {1}{W}{W} look payable with only {W}{W}.
func TestPrototypePlannerUsesInsetManaValue(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 8107, prototypeSeraphSrc)
	addToHand(t, e, 0, prototypeSeraphSrc)
	onBoardCard(t, e, 0, corpusCard(t, "Krosan Drover"))
	addMana(t, e, 0, "WW")
	opt := decision.Option{Kind: "cast", Obj: id, Mode: "prototype"}
	if got := e.potentialModeCastPlan(0, opt); got.Plan != nil || got.Reason != "insufficient" {
		t.Fatalf("prototype plan with only {W}{W} = %+v, want insufficient", got)
	}
	addMana(t, e, 0, "C")
	if got := e.potentialModeCastPlan(0, opt); got.Plan == nil || got.Reason != "" {
		t.Fatalf("prototype plan with {1}{W}{W} = %+v, want a witness", got)
	}
}
