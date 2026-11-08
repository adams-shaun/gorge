package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDepartedAuraEnchantedSkipsFlickeredBearer pins CR 400.7 / 608.2h for
// Defined$ Enchanted off the battlefield: Uneasy Alliance is sacrificed as a
// cost ("Exile enchanted creature"), and in response the bearer is flickered
// (exiled and returned). The returned Giant is a new object, so the Aura's
// last-known bearer no longer exists and the exile does nothing.
func TestDepartedAuraEnchantedSkipsFlickeredBearer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	forest := searchCorpusCard(t, reg, "Forest")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Uneasy Alliance"), searchCorpusCard(t, reg, "Hill Giant")}
	for len(deck) < 40 {
		deck = append(deck, forest, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = forest
	}
	cfg := seatZeroStart(Config{Seed: 9411, Names: []string{"protagonist", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	aura := searchMoveByName(t, e, "Uneasy Alliance", state.ZHand)
	giant := searchMoveByName(t, e, "Hill Giant", state.ZBattlefield)

	addMana(t, e, 0, "1W")
	castNamed(t, e, "Uneasy Alliance")
	attachDrain(t, e, 40)
	if ao := e.G.Obj(aura); ao.Zone != state.ZBattlefield || ao.AttachedTo != giant {
		t.Fatalf("precondition: Aura must be attached to the Giant, got %+v", ao)
	}

	addMana(t, e, 0, "CCCCC")
	submitChoices(t, e, abilityOption(t, e, aura, 0).Index)
	// Flicker the bearer in response.
	e.emit(events.Event{Kind: events.MoveZone, Obj: giant, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: giant, From: state.ZExile, To: state.ZBattlefield})
	attachDrain(t, e, 40)

	if ao := e.G.Obj(aura); ao.Zone != state.ZGraveyard {
		t.Fatalf("Aura zone = %s, want graveyard (sacrificed as a cost)", ao.Zone)
	}
	if gz := e.G.Obj(giant).Zone; gz != state.ZBattlefield {
		t.Fatalf("Hill Giant zone = %s, want battlefield (a new object the Aura never enchanted)", gz)
	}
	replayCheck(t, e, cfg)
}

// TestAttachedByReturnedSkipsFlickeredBearer pins CR 400.7 / 608.2h for
// Defined$ AttachedBy off the battlefield: Krasis Incubation is returned to
// hand as a cost ("Put two +1/+1 counters on enchanted creature"), and in
// response the bearer is flickered. The returned Giant is a new object the
// Aura never enchanted, so it gets no counters.
func TestAttachedByReturnedSkipsFlickeredBearer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	forest := searchCorpusCard(t, reg, "Forest")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Krasis Incubation"), searchCorpusCard(t, reg, "Hill Giant")}
	for len(deck) < 40 {
		deck = append(deck, forest, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = forest
	}
	cfg := seatZeroStart(Config{Seed: 9412, Names: []string{"protagonist", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	aura := searchMoveByName(t, e, "Krasis Incubation", state.ZHand)
	giant := searchMoveByName(t, e, "Hill Giant", state.ZBattlefield)

	addMana(t, e, 0, "CCGU")
	castNamed(t, e, "Krasis Incubation")
	attachDrain(t, e, 40)
	if ao := e.G.Obj(aura); ao.Zone != state.ZBattlefield || ao.AttachedTo != giant {
		t.Fatalf("precondition: Aura must be attached to the Giant, got %+v", ao)
	}

	addMana(t, e, 0, "CGU")
	submitChoices(t, e, abilityOption(t, e, aura, 0).Index)
	if ao := e.G.Obj(aura); ao.Zone != state.ZHand || ao.LastBearer != giant {
		t.Fatalf("precondition: Aura must be returned with LastBearer = Giant, got zone %s bearer %d", ao.Zone, ao.LastBearer)
	}
	// Flicker the bearer in response.
	e.emit(events.Event{Kind: events.MoveZone, Obj: giant, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: giant, From: state.ZExile, To: state.ZBattlefield})
	attachDrain(t, e, 40)

	if ao := e.G.Obj(aura); ao.Zone != state.ZHand {
		t.Fatalf("Aura zone = %s, want hand (returned as a cost)", ao.Zone)
	}
	g := e.G.Obj(giant)
	if g.Zone != state.ZBattlefield {
		t.Fatalf("Hill Giant zone = %s, want battlefield", g.Zone)
	}
	if n := g.Counter("P1P1"); n != 0 {
		t.Fatalf("Hill Giant P1P1 = %d, want 0 (a new object the Aura never enchanted)", n)
	}
	replayCheck(t, e, cfg)
}
