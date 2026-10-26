package rules

// mayplay_case_uneaten_feast_test.go pins Case of the Uneaten Feast's solved
// ability: "Creature cards in your graveyard gain 'You may cast this card from
// your graveyard' until end of turn." The grant is an Animate staticAbilities$
// body (a self Mode$ Continuous MayPlay$ True static) registered per affected
// card, so it follows the animation's own until-end-of-turn lifetime and stays
// with the card only while it stays in the graveyard.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestCaseOfTheUneatenFeastGrantsGraveyardCasts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Case of the Uneaten Feast"), lookup(t, reg, "Ambush Viper"), lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Mountain")}, mountainDeck(t, 36)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	c := findAndMoveToHand(t, e, 0, "Case of the Uneaten Feast")
	moveToBattlefield(t, e, c)
	v := findAndMoveToHand(t, e, 0, "Ambush Viper")
	b := findAndMoveToHand(t, e, 0, "Grizzly Bears")
	land := findAndMoveToHand(t, e, 0, "Mountain")
	e.pending = nil
	e.Advance()
	// Seat 1's turn: the solved ability has no timing restriction, so seat 0
	// activates it there and Ambush Viper's flash makes the cast real.
	driveToStep(t, e, 4, 1, state.StepUpkeep)
	e.emit(events.Event{Kind: events.AlterAttribute, Obj: c, Text: "Solved", Amount: 1})
	for _, id := range []state.ObjID{v, b, land} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	}
	e.pending = nil
	e.Advance()
	castFirst(t, e, "pass")
	if e.G.Active != 1 || e.G.Priority != 0 {
		t.Fatalf("precondition: active %d priority %d, want seat 1's turn with seat 0 priority", e.G.Active, e.G.Priority)
	}
	addManaQuiet(t, e, 0, "GGGG")
	// Precondition: nothing is castable from the graveyard before the sacrifice.
	if got := e.mayPlaySpellIds(0); len(got) != 0 {
		t.Fatalf("precondition: graveyard casts offered before the Case resolves: %+v", got)
	}
	marker := len(e.L.Events)
	submitChoices(t, e, abilityOption(t, e, c, 0).Index)
	for i := 0; i < 10 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
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
	if d := e.Pending(); d != nil && d.Player != 0 {
		castFirst(t, e, "pass")
	}
	if d := e.Pending(); d == nil || d.Player != 0 || e.G.Active != 1 {
		t.Fatalf("precondition: want seat 0 at priority on seat 1's turn, got %+v", d)
	}
	if e.G.Obj(c).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the Case was not sacrificed (zone %v)", e.G.Obj(c).Zone)
	}
	for _, ev := range e.L.Events[marker:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "not implemented") {
			t.Fatalf("unexpected Note: %q", ev.Text)
		}
	}

	offered := map[state.ObjID]bool{}
	for _, off := range e.mayPlaySpellIds(0) {
		if off.zone != state.ZGraveyard {
			t.Fatalf("offer from %v, want graveyard: %+v", off.zone, off)
		}
		offered[off.id] = true
	}
	if !offered[v] || !offered[b] {
		t.Fatalf("Ambush Viper %d / Grizzly Bears %d not both castable from the graveyard: %v", v, b, offered)
	}
	if offered[land] || offered[c] {
		t.Fatalf("land %d or the sacrificed Case %d offered: %v", land, c, offered)
	}
	if ids := e.mayPlayLandIds(0); len(ids) != 0 {
		t.Fatalf("graveyard land offered for play: %v", ids)
	}

	// Instant speed on the opponent's turn: Viper (flash) is offered as a cast
	// from the graveyard and resolves; Bears (no flash) is not castable now.
	if _, ok := mpRiderOffer(e, b); ok {
		t.Fatalf("Grizzly Bears offered at instant speed: %v", mpRiderLabels(e))
	}
	mpRiderCastResolve(t, e, v)
	if got := e.G.Obj(v).Zone; got != state.ZBattlefield {
		t.Fatalf("Ambush Viper zone %v after the cast, want battlefield", got)
	}

	// The permission is gone at the next turn (CR 514.2): Bears, still in the
	// graveyard, is no longer offered.
	if e.G.Obj(b).Zone != state.ZGraveyard {
		t.Fatalf("precondition: Bears left the graveyard (%v)", e.G.Obj(b).Zone)
	}
	mpRiderDrive(t, e, 5, 0, state.StepMain1)
	addManaQuiet(t, e, 0, "GG")
	if got := e.mayPlaySpellIds(0); len(got) != 0 {
		t.Fatalf("permission outlived the turn: %+v", got)
	}
}
