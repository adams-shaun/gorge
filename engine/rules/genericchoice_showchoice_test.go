package rules

// ShowChoice$ on api:GenericChoice only picks who is told which option was
// chosen (Forge ChooseGenericEffect); the choice is already a public
// ModeChosen answer. It must not log the "ignores unread parameter(s)" Note,
// while a genuinely unread key on the same ability still must.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// unreadNotes returns the log's "ignores unread parameter(s)" Note texts.
func unreadNotes(e *Engine) []string {
	var out []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unread parameter") {
			out = append(out, ev.Text)
		}
	}
	return out
}

// passToNonPriority passes priority until a non-priority decision is pending.
func passToNonPriority(t *testing.T, e *Engine, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind != decision.KPriority {
			return d
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d.Options)
		}
		submitChoices(t, e, idx)
	}
	t.Fatal("no non-priority decision within the pass budget")
	return nil
}

func TestGenericChoiceShowChoiceIsNotUnread(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	siege := mustCorpusCard(t, reg, "Windcrag Siege")
	cfg := seatZeroStart(Config{Seed: 91, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{siege}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	id := placeInDeck(t, e, 0, siege, state.ZHand)
	addMana(t, e, 0, "1RW")
	if z := e.G.Obj(id).Zone; z != state.ZHand {
		t.Fatalf("Windcrag Siege zone = %s, want Hand", z)
	}
	submitSeizeCast(t, e, id)
	d := passToNonPriority(t, e, 40)
	// Precondition: the as-enters Mardu/Jeskai ask is the one pending.
	mardu := -1
	for _, o := range d.Options {
		if o.Label == "Mardu" {
			mardu = o.Index
		}
	}
	if mardu < 0 {
		t.Fatalf("pending ask has no Mardu option: %+v", d)
	}
	submitChoices(t, e, mardu)
	passUntilStackEmpty(t, e, 40)

	if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
		t.Fatalf("Windcrag Siege zone = %s, want Battlefield (the resolution ran)", z)
	}
	chosen := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && strings.Contains(ev.Text, "Mardu") {
			chosen = true
		}
	}
	if !chosen {
		t.Fatal("no ModeChosen event naming Mardu")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "ShowChoice") {
			t.Fatalf("ShowChoice$ logged as unread: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}

func TestGenericChoiceOtherUnreadStillNotes(t *testing.T) {
	t.Parallel()
	src := "Name:Pick\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ GenericChoice | Choices$ A,B | ShowChoice$ ExceptSelf | NoSuchKey$ 1 | SpellDescription$ pick\n" +
		"SVar:A:DB$ GainLife | LifeAmount$ 1\nSVar:B:DB$ GainLife | LifeAmount$ 2\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 92, src)
	addMana(t, e, 0, "R")
	castFirst(t, e, "cast")
	d := passToNonPriority(t, e, 40)
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 40)

	notes := unreadNotes(e)
	if len(notes) == 0 {
		t.Fatal("no unread-parameter Note for NoSuchKey$ (precondition: the Note path ran)")
	}
	for _, n := range notes {
		if strings.Contains(n, "ShowChoice") {
			t.Fatalf("Note names ShowChoice$: %q", n)
		}
	}
	if !strings.Contains(strings.Join(notes, "\n"), "NoSuchKey") {
		t.Fatalf("notes %q do not name NoSuchKey$", notes)
	}
	replayCheck(t, e, cfg)
}
