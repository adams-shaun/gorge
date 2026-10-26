package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestTishanasTidebinderSourceLosesAbilities(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Tishana's Tidebinder"), lookup(t, reg, "Prodigal Sorcerer")}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	ps := findAndMoveToHand(t, e, 0, "Prodigal Sorcerer")
	moveToBattlefield(t, e, ps)
	tb := findAndMoveToHand(t, e, 0, "Tishana's Tidebinder")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	addMana(t, e, 0, "UUU")
	opt := abilityOption(t, e, ps, 0)
	submitChoices(t, e, opt.Index)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, 0)
	}
	d := e.Pending()
	submitChoices(t, e, castOptionIdx(d, "cast", tb))
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if len(e.G.Stack) == 0 {
				break
			}
			submitChoices(t, e, pass)
			continue
		}
		submitChoices(t, e, 0)
	}
	e.emit(events.Event{Kind: events.Untap, Obj: ps})
	e.pending = nil
	e.Advance()
	n := 0
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == ps {
			n++
		}
	}
	if n != 0 {
		t.Fatalf("Prodigal Sorcerer still offers %d abilities after Tidebinder countered its ability; it should lose all abilities", n)
	}
	if _, ok := e.abilityLoss(e.G.Obj(ps)); !ok && !e.printedAbilitiesGone(e.G.Obj(ps)) {
		t.Fatal("Prodigal Sorcerer neither has an ability loss nor gone printed abilities")
	}
}
