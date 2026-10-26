package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestLagrellaReturnAbilityCounters(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Lagrella, the Magpie"), lookup(t, reg, "Grizzly Bears")}, mountainDeck(t, 38)...),
			append([]*cards.Card{lookup(t, reg, "Grizzly Bears")}, mountainDeck(t, 39)...),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	b0 := findAndMoveToHand(t, e, 0, "Grizzly Bears")
	moveToBattlefield(t, e, b0)
	b1 := findAndMoveToHand(t, e, 1, "Grizzly Bears")
	moveToBattlefield(t, e, b1)
	l := findAndMoveToHand(t, e, 0, "Lagrella, the Magpie")
	moveToBattlefield(t, e, l)
	e.Advance()
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && i > 2 {
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
		ch := []int{0}
		if d.Kind == decision.KTarget && len(d.Options) >= 2 {
			ch = []int{0, 1}
		}
		submitChoices(t, e, ch...)
	}
	if e.G.Obj(b0).Zone != state.ZExile || e.G.Obj(b1).Zone != state.ZExile {
		t.Fatalf("precondition: both bears should be exiled by Lagrella (zones %v %v)", e.G.Obj(b0).Zone, e.G.Obj(b1).Zone)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: l, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && i > 2 {
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
		submitChoices(t, e, 0)
	}
	mine, theirs := e.G.Obj(b0), e.G.Obj(b1)
	if mine.Zone != state.ZBattlefield || theirs.Zone != state.ZBattlefield {
		t.Fatalf("both bears should have returned (zones %v %v)", mine.Zone, theirs.Zone)
	}
	if got := mine.Counter("P1P1"); got != 2 {
		t.Fatalf("own creature returned with %d +1/+1 counters, want 2", got)
	}
	if theirs.Controller != 1 {
		t.Fatalf("opponent's creature returned under seat %d, want its owner", theirs.Controller)
	}
	if got := theirs.Counter("P1P1"); got != 0 {
		t.Fatalf("opponent's creature returned with %d counters, want 0 (ReturnValid YouCtrl)", got)
	}
}
