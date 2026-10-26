package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mosswoodDiesEngine returns an engine where seat 0's Mosswood Dreadknight has
// died in main phase 1 of turn 3 and its trigger has resolved, leaving the
// "cast it from your graveyard as an Adventure" grant registered.
func mosswoodDiesEngine(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Mosswood Dreadknight")}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	})
	e := New(cfg)
	e.Advance()
	m := findAndMoveToHand(t, e, 0, "Mosswood Dreadknight")
	moveToBattlefield(t, e, m)
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: m, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority && len(e.G.Stack) == 0 && i > 0 {
			break
		}
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					submitChoices(t, e, o.Index)
					break
				}
			}
			continue
		}
		submitChoices(t, e, 0)
	}
	if o := e.G.Obj(m); o.Zone != state.ZGraveyard {
		t.Fatalf("Mosswood Dreadknight zone = %s, want graveyard", o.Zone)
	}
	return e, cfg, m
}

// TestMosswoodDreadknightGraveyardAdventure: after it dies, the grant `Affected$
// Card.IsRemembered+Adventure` lets the card be cast from the graveyard as the
// Adventure ONLY (Dread Whispers, {1}{B}); resolving it exiles the card, from
// where the creature is castable (CR 715.4); the grant ends after the
// controller's next turn. The two halves of the defect: "+Adventure" read as a
// type test on the graveyard card's main face (no offer at all), and the
// may-play walk having no Adventure-face route (the creature offered instead).
func TestMosswoodDreadknightGraveyardAdventure(t *testing.T) {
	t.Run("cast Dread Whispers then the creature", func(t *testing.T) {
		e, cfg, m := mosswoodDiesEngine(t)
		addMana(t, e, 0, "GGBB")
		if adventureOption(t, e, m, "adventure_alt") == nil {
			t.Fatalf("Dread Whispers not offered from the graveyard: %+v", castOptions(t, e))
		}
		for _, o := range castOptions(t, e) {
			if o.Obj == m && o.Mode != "adventure_alt" {
				t.Fatalf("a non-Adventure cast of the graveyard card is offered: %+v", o)
			}
		}
		if opt := adventureOption(t, e, m, "adventure_alt"); opt.Label != "Cast Dread Whispers" {
			t.Fatalf("label = %q, want Cast Dread Whispers", opt.Label)
		}
		opt := adventureOption(t, e, m, "adventure_alt")
		hand := len(e.G.Zone(state.ZHand, 0))
		life := e.G.Players[0].Life
		submitChoices(t, e, opt.Index)
		if e.G.Obj(m).Zone != state.ZStack {
			t.Fatalf("after casting, zone = %s, want stack", e.G.Obj(m).Zone)
		}
		if got := e.G.Players[0].Pool.Total(); got != 2 {
			t.Fatalf("mana left after paying {1}{B} = %d, want 2", got)
		}
		resolveStack(t, e)
		o := e.G.Obj(m)
		if o.Zone != state.ZExile || o.FaceIdx != 0 {
			t.Fatalf("after Dread Whispers resolved: zone=%s face=%d, want exile/0", o.Zone, o.FaceIdx)
		}
		if len(e.G.Zone(state.ZHand, 0)) != hand+1 || e.G.Players[0].Life != life-1 {
			t.Fatalf("Dread Whispers did not draw and lose 1 life: hand %d->%d life %d->%d",
				hand, len(e.G.Zone(state.ZHand, 0)), life, e.G.Players[0].Life)
		}
		addMana(t, e, 0, "GG")
		re := adventureOption(t, e, m, "adventure_recast")
		if re == nil {
			t.Fatalf("creature not castable from exile: %+v", castOptions(t, e))
		}
		submitChoices(t, e, re.Index)
		resolveStack(t, e)
		if o := e.G.Obj(m); o.Zone != state.ZBattlefield {
			t.Fatalf("after the recast zone = %s, want battlefield", o.Zone)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("grant ends after the controller's next turn", func(t *testing.T) {
		e, _, m := mosswoodDiesEngine(t)
		driveToStep(t, e, 5, 0, state.StepMain1)
		addMana(t, e, 0, "GGBB")
		if adventureOption(t, e, m, "adventure_alt") == nil {
			t.Fatalf("grant ended before the end of the controller's next turn: %+v", castOptions(t, e))
		}
		driveToStep(t, e, 7, 0, state.StepMain1)
		addMana(t, e, 0, "GGBB")
		for _, o := range castOptions(t, e) {
			if o.Obj == m {
				t.Fatalf("graveyard card still castable after the grant ended: %+v", o)
			}
		}
	})
}
