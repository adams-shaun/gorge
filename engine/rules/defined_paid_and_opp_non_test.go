package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDefinedDiscardedCardsNamesThePaidDiscard: Defined$ DiscardedCards is the
// card the activation's own Discard cost put into the graveyard (Forge's
// paid-cost "Discarded" list). Kogla and Yidaro discards itself and shuffles
// itself back from the graveyard; a source that stays behind and discards a
// different card must move that card, never itself.
func TestDefinedDiscardedCardsNamesThePaidDiscard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	kogla := mustCorpusCard(t, reg, "Kogla and Yidaro")
	e := handEngine(t, kogla)
	id := e.G.Zone(state.ZHand, 0)[0]
	addMana(t, e, 0, "CCRG")
	e.G.SetZone(state.ZLibrary, 0, nil)
	var opt decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id && strings.Contains(o.Cost, "Discard<1/CARDNAME>") {
			opt, found = o, true
		}
	}
	if !found {
		t.Fatal("precondition: the discard activation is not offered")
	}
	e.beginActivation(0, opt)
	drainStackPassing(t, e, 40)
	if got := e.G.Obj(id).Zone; got != state.ZLibrary && got != state.ZHand {
		t.Fatalf("Kogla and Yidaro in %s, want shuffled into the library and drawn", got)
	}
	if got := e.G.Obj(id).Zone; got == state.ZGraveyard {
		t.Fatal("Kogla and Yidaro stayed in the graveyard")
	}
}

func TestDefinedDiscardedCardsIsNotTheSource(t *testing.T) {
	t.Parallel()
	const banker = "Name:Pile Banker\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ ChangeZone | Cost$ 1 Discard<1/Card> | Origin$ Graveyard | Destination$ Library | Defined$ DiscardedCards | SpellDescription$ x\n" +
		"Oracle:x\n"
	const fodder = "Name:Spare Rock\nManaCost:1\nTypes:Artifact\nOracle:x\n"
	bank, rock := card(t, banker), card(t, fodder)
	e := handEngine(t, bank, rock)
	hand := e.G.Zone(state.ZHand, 0)
	bankID, rockID := hand[0], hand[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: bankID, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "C")
	var opt decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == bankID {
			opt, found = o, true
		}
	}
	if !found {
		t.Fatal("precondition: the banker's ability is not offered")
	}
	e.beginActivation(0, opt)
	for range 6 {
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
			submitChoices(t, e, 0)
		}
	}
	drainStackPassing(t, e, 40)
	if got := e.G.Obj(rockID).Zone; got != state.ZLibrary {
		t.Fatalf("discarded card in %s, want the library", got)
	}
	if got := e.G.Obj(bankID).Zone; got != state.ZBattlefield {
		t.Fatalf("the source moved to %s: Defined$ DiscardedCards resolved to the source", got)
	}
}

// TestOppNonRememberedOwnerDrawsForTheOpponentWhoDeclined: Kynaios and Tiro of
// Meletis' "each opponent who didn't draws a card" is Defined$
// OppNonRememberedOwner, the controller's opponents minus the owners of the
// lands that were put onto the battlefield. A 2-player game, seat 0 plays a
// land: a declining seat 1 draws; a playing seat 1 does not.
func TestOppNonRememberedOwnerDrawsForTheOpponentWhoDeclined(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	kyn := mustCorpusCard(t, reg, "Kynaios and Tiro of Meletis")
	for _, tc := range []struct {
		name      string
		seat1Play bool
		wantDelta int
	}{{"declines draws", false, 1}, {"plays no draw", true, -1}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Seed: 5, Names: []string{"a", "b"}, Tokens: map[string]*cards.Card{},
				Decks: [][]*cards.Card{append([]*cards.Card{kyn}, mountainDeck(t, 59)...), mountainDeck(t, 60)}}
			cfg = seatZeroStart(cfg)
			e := New(cfg)
			e.Advance()
			var kynID state.ObjID
			for i := range e.G.Objs {
				if o := &e.G.Objs[i]; o.Owner == 0 && o.Card == kyn {
					kynID = o.ID
				}
			}
			if o := e.G.Obj(kynID); o.Zone != state.ZBattlefield {
				e.emit(events.Event{Kind: events.MoveZone, Obj: kynID, From: o.Zone, To: state.ZBattlefield})
			}
			e.pending = nil
			e.Advance()
			driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
			d := passUntilNonPriority(t, e, 40)
			if d == nil || d.ResumeKind != "hand_move" {
				t.Fatalf("expected the first hand_move ask, got %+v", d)
			}
			submitChoices(t, e, handMoveOption(t, d, e.G.Zone(state.ZHand, 0)[0]))
			d = passUntilNonPriority(t, e, 40)
			if d == nil || d.ResumeKind != "hand_move" || d.Player != 1 {
				t.Fatalf("expected seat 1's hand_move ask, got %+v", d)
			}
			before := len(e.G.Zone(state.ZHand, 1))
			if tc.seat1Play {
				submitChoices(t, e, handMoveOption(t, d, e.G.Zone(state.ZHand, 1)[0]))
			} else {
				submitChoices(t, e)
			}
			passUntilStackEmpty(t, e, 40)
			if got := len(e.G.Zone(state.ZHand, 1)) - before; got != tc.wantDelta {
				t.Fatalf("seat 1 hand moved by %d, want %d", got, tc.wantDelta)
			}
		})
	}
}
