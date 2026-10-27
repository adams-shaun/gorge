package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A GainsAbilitiesOf$ grant's ManaConversion$ (Drana and Linvala, Scheming
// Fence; CR 609.4b) lets the controller spend mana as though it were mana of
// any color to activate exactly the abilities that grant gave -- never the
// recipient's own abilities, never a spell.

// gainedOption returns the pending priority decision's gained "ability"
// option on obj, if offered.
func gainedOption(e *Engine, obj state.ObjID) (decision.Option, bool) {
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == obj && o.GainedSource != 0 {
				return o, true
			}
		}
	}
	return decision.Option{}, false
}

func dranaBoard(t *testing.T, extraSeat0 ...string) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	drana := tokenReplCorpusCard(t, "Drana and Linvala")
	dragon := tokenReplCorpusCard(t, "Shivan Dragon")
	seat0 := []*cards.Card{drana}
	for _, src := range extraSeat0 {
		seat0 = append(seat0, card(t, src))
	}
	e, cfg := tokenReplGameSeats(t, 9201, seat0, []*cards.Card{dragon})
	dranaID := moveSeededCard(t, e, 0, drana, state.ZBattlefield)
	dragonID := moveSeededCard(t, e, 1, dragon, state.ZBattlefield)
	return e, cfg, dranaID, dragonID
}

func TestGainedManaConversionDranaPaysRedWithBlue(t *testing.T) {
	t.Parallel()
	e, cfg, dranaID, dragonID := dranaBoard(t)
	addMana(t, e, 0, "U")
	if o := e.G.Obj(dragonID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: Shivan Dragon = %+v, want on seat 1's battlefield", o)
	}
	if got := e.G.Players[0].Pool[state.MU]; got != 1 {
		t.Fatalf("precondition: pool blue = %d, want 1", got)
	}
	opt, ok := gainedOption(e, dranaID)
	if !ok {
		t.Fatalf("Drana does not offer the gained {R} pump with only {U}: %+v", e.Pending().Options)
	}
	if opt.GainedSource != dragonID {
		t.Fatalf("gained anchor = %d, want the dragon %d", opt.GainedSource, dragonID)
	}
	before := e.Power(dranaID)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if got := e.Power(dranaID); got != before+1 {
		t.Fatalf("Drana power %d -> %d, want +1", before, got)
	}
	if got := e.G.Players[0].Pool[state.MU]; got != 0 {
		t.Fatalf("pool blue = %d after activation, want the {U} spent", got)
	}
	replayCheck(t, e, cfg)
}

func TestGainedManaConversionScopedToGainedAbilities(t *testing.T) {
	t.Parallel()
	e, _, dranaID, _ := dranaBoard(t,
		"Name:Red Spark\nManaCost:R\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n")
	spell := moveSeededCard(t, e, 0, card(t, "Name:Red Spark\nManaCost:R\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"), state.ZHand)
	addMana(t, e, 0, "U")
	if _, ok := gainedOption(e, dranaID); !ok {
		t.Fatal("precondition: the gained ability is not offered, so the conversion is not on the board")
	}
	for _, o := range e.Pending().Options {
		if o.Obj == spell && o.Kind == "cast" {
			t.Fatalf("{R} spell offered with only {U}: %+v", o)
		}
	}
	// With {R} the same spell is castable: the probe is live.
	addMana(t, e, 0, "R")
	found := false
	for _, o := range e.Pending().Options {
		if o.Obj == spell && o.Kind == "cast" {
			found = true
		}
	}
	if !found {
		t.Fatalf("precondition: {R} spell not castable with {R}{U}: %+v", e.Pending().Options)
	}
}

func TestGainedManaConversionSchemingFenceChosenPermanent(t *testing.T) {
	t.Parallel()
	fence := tokenReplCorpusCard(t, "Scheming Fence")
	rock := card(t, "Name:Green Rock\nManaCost:2\nTypes:Artifact\nA:AB$ Draw | Cost$ 1 G | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, 9202, []*cards.Card{fence}, []*cards.Card{rock})
	fenceID := moveSeededCard(t, e, 0, fence, state.ZBattlefield)
	rockID := moveSeededCard(t, e, 1, rock, state.ZBattlefield)
	e.emit(events.Event{Kind: events.Choose, Obj: fenceID, Counter: "chosen", IDs: []state.ObjID{rockID}})
	e.pending = nil
	addMana(t, e, 0, "UU")
	opt, ok := gainedOption(e, fenceID)
	if !ok {
		t.Fatalf("Fence does not offer the chosen artifact's {1}{G} ability with {U}{U}: %+v", e.Pending().Options)
	}
	if opt.GainedSource != rockID {
		t.Fatalf("gained anchor = %d, want the artifact %d", opt.GainedSource, rockID)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand %d -> %d, want +1 from the gained draw", hand, got)
	}
	replayCheck(t, e, cfg)
}

func TestGainsAbilitiesWithoutConversionUnchanged(t *testing.T) {
	t.Parallel()
	idris := tokenReplCorpusCard(t, "Idris, Soul of the TARDIS")
	artifact := card(t, "Name:Red Gains\nManaCost:2\nTypes:Artifact\nA:AB$ Draw | Cost$ R | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n")
	e, _ := tokenReplGame(t, 9203, idris, artifact)
	idrisID := moveSeededCard(t, e, 0, idris, state.ZBattlefield)
	artifactID := moveSeededCard(t, e, 0, artifact, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifactID, From: state.ZBattlefield,
		To: state.ZExile, IDs: []state.ObjID{idrisID}})
	e.pending, e.pendingTriggers = nil, nil
	addMana(t, e, 0, "U")
	if _, ok := gainedOption(e, idrisID); ok {
		t.Fatal("a gain without ManaConversion$ offered the {R} ability with only {U}")
	}
	addMana(t, e, 0, "R")
	if _, ok := gainedOption(e, idrisID); !ok {
		t.Fatalf("precondition: the gained {R} ability is not offered even with {R}: %+v", e.Pending().Options)
	}
}
