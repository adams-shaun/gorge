package rules

// Kai Winn, Faithless Leader: "As Kai Winn enters, choose a nonland card
// name. Whenever an opponent casts a spell with the chosen name or activates
// an ability of a source with the chosen name, that player loses 3 life."
// (script params: ETBReplacement NameCard with ValidCards Card.nonLand;
// Mode$ SpellAbilityCast with ValidCard Card.NamedCard, ValidSA Spell,Activated,
// ValidActivatingPlayer Opponent; LoseLife Defined TriggeredActivator.)
//
// The shapes pinned here are the ones the kernel's lockstep (dirac/oracle
// TestKaiWinnLockstep) must agree with: the name is chosen from nonland names
// as the permanent enters; an opponent's cast of the named spell triggers
// once; an opponent's activation of a named permanent's ability triggers once;
// an unrelated spell, the controller's own named spell and a copy of a named
// spell (CR 707.10: a copy is not cast) do not; and the choice does not
// survive Kai Winn leaving the battlefield (CR 400.7), so the next entry asks
// anew. A mana ability of a named source is out of scope: gorge fires
// AbilityCast/SpellAbilityCast only for abilities that use the stack.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kaiDrain answers whatever is pending until the stack is empty and no ask
// is outstanding: the name ask with name (counting it), a priority with pass,
// a target ask with its first option, any other ask with option 0. It
// returns how many name asks it answered.
func kaiDrain(t *testing.T, e *Engine, name string) int {
	t.Helper()
	asks := 0
	for i := 0; i < 80 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "name" {
			idx := labelIndex(d, name)
			if idx < 0 {
				t.Fatalf("name ask did not offer %q", name)
			}
			if labelIndex(d, "Mountain") >= 0 || labelIndex(d, "Swamp") >= 0 {
				t.Fatalf("a nonland name ask offered a basic land name")
			}
			submitChoices(t, e, idx)
			asks++
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				return asks
			}
			passIdx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					passIdx = o.Index
				}
			}
			if passIdx < 0 {
				t.Fatalf("priority with no pass: %+v", d.Options)
			}
			submitChoices(t, e, passIdx)
			continue
		}
		if d.Kind == decision.KTriggerOrder {
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			submitChoices(t, e, idx...)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	t.Fatalf("stack never settled (depth %d)", len(e.G.Stack))
	return asks
}

// kaiCast casts the hand card id as its owner and drains the stack.
func kaiCast(t *testing.T, e *Engine, id state.ObjID, name string) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != e.G.Obj(id).Owner {
		t.Fatalf("owner of %d does not hold priority: %+v", id, d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	kaiDrain(t, e, name)
}

func kaiEngine(t *testing.T, reg *cards.Registry) *Engine {
	t.Helper()
	pad := func(extra ...string) []*cards.Card {
		var deck []*cards.Card
		for _, n := range extra {
			deck = append(deck, mustCorpusCard(t, reg, n))
		}
		for len(deck) < 40 {
			deck = append(deck, lookup(t, reg, "Mountain"))
		}
		return deck
	}
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			pad("Kai Winn, Faithless Leader", "Divination"),
			pad("Divination", "Divination", "Millstone", "Reverberate"),
		},
		NameUniverse: reg.Universe()})
	e := New(cfg)
	if e.G.NameUniverse.Len() == 0 {
		t.Fatal("precondition: NameUniverse not wired from Config")
	}
	e.Advance()
	return e
}

func TestKaiWinnGorge(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := kaiEngine(t, reg)
	kai := moveCorpusCard(t, e, "Kai Winn, Faithless Leader", 0, state.ZHand)
	if o := e.G.Obj(kai); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Kai Winn not in hand")
	}
	life := func(p int) int { return int(e.G.Players[p].Life) }
	triggers := func() int { return pushCount(e, kai) }

	// --- turn 1: as it enters, a nonland card name is chosen (CR 614.12) ---
	addMana(t, e, 0, "BB")
	kaiCast(t, e, kai, "Millstone")
	if o := e.G.Obj(kai); o.Zone != state.ZBattlefield || o.ChosenName != "Millstone" {
		t.Fatalf("Kai Winn zone %s name %q, want battlefield / Millstone", o.Zone, o.ChosenName)
	}
	if triggers() != 0 {
		t.Fatalf("Kai Winn triggered on its own entry")
	}

	// --- turn 2, seat 1 (an opponent) ---
	driveToStep(t, e, 2, 1, state.StepMain1)
	// An unrelated spell does not trigger.
	div := moveCorpusCard(t, e, "Divination", 1, state.ZHand)
	addMana(t, e, 1, "UUU")
	kaiCast(t, e, div, "")
	if life(1) != 20 || triggers() != 0 {
		t.Fatalf("unrelated spell: life %d triggers %d, want 20 / 0", life(1), triggers())
	}
	// The named spell triggers once; its controller loses 3.
	stone := moveCorpusCard(t, e, "Millstone", 1, state.ZHand)
	addMana(t, e, 1, "CC")
	kaiCast(t, e, stone, "")
	if o := e.G.Obj(stone); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Millstone zone %s, want battlefield", o.Zone)
	}
	if life(1) != 17 || life(0) != 20 || triggers() != 1 {
		t.Fatalf("named spell: life %d/%d triggers %d, want 17/20 / 1", life(1), life(0), triggers())
	}
	// An ability of a source with the chosen name triggers once.
	addMana(t, e, 1, "CC")
	submitChoices(t, e, abilityOption(t, e, stone, 0).Index)
	kaiDrain(t, e, "")
	if life(1) != 14 || triggers() != 2 {
		t.Fatalf("named source's ability: life %d triggers %d, want 14 / 2", life(1), triggers())
	}
	if e.G.Obj(stone).ActivatedThisTurn < 1 {
		t.Fatalf("precondition: Millstone's ability was not activated")
	}

	// --- Kai Winn leaves: the choice does not survive (CR 400.7) ---
	e.emit(events.Event{Kind: events.MoveZone, Obj: kai, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(kai).ChosenName; got != "" {
		t.Fatalf("Kai Winn kept the chosen name %q after leaving the battlefield", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: kai, From: state.ZGraveyard, To: state.ZHand})
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)

	// --- turn 3, seat 0: it re-enters and names anew; its controller's own
	// cast of the new name does not trigger ("an opponent") ---
	addMana(t, e, 0, "BB")
	kaiCast(t, e, kai, "Divination")
	if o := e.G.Obj(kai); o.Zone != state.ZBattlefield || o.ChosenName != "Divination" {
		t.Fatalf("re-entered Kai Winn zone %s name %q, want battlefield / Divination", o.Zone, o.ChosenName)
	}
	before := triggers()
	own := moveCorpusCard(t, e, "Divination", 0, state.ZHand)
	addMana(t, e, 0, "UUU")
	kaiCast(t, e, own, "")
	if triggers() != before || life(0) != 20 {
		t.Fatalf("controller's own named spell: triggers %d (was %d), life %d", triggers(), before, life(0))
	}

	// --- turn 4, seat 1: the named spell is cast once; a copy of it is not
	// cast and does not trigger ---
	driveToStep(t, e, 4, 1, state.StepMain1)
	div2 := moveCorpusCard(t, e, "Divination", 1, state.ZHand)
	rev := moveCorpusCard(t, e, "Reverberate", 1, state.ZHand)
	lifeBefore := life(1)
	addMana(t, e, 1, "UUURR")
	d := e.Pending()
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == div2 {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for Divination: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)
	// Divination is on the stack, with Kai Winn's trigger above it; the
	// active player holds priority and casts Reverberate at it.
	if o := e.G.Obj(div2); o.Zone != state.ZStack {
		t.Fatalf("precondition: Divination zone %s, want stack", o.Zone)
	}
	d = e.Pending()
	revIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == rev {
			revIdx = o.Index
		}
	}
	if revIdx < 0 {
		t.Fatalf("no cast option for Reverberate: %+v", d.Options)
	}
	submitChoices(t, e, revIdx)
	for i := 0; i < 3; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			idx := d.Options[0].Index
			for _, o := range d.Options {
				if o.Obj == div2 {
					idx = o.Index
				}
			}
			submitChoices(t, e, idx)
			break
		}
	}
	kaiDrain(t, e, "")
	copies := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.StackCopy {
			copies++
		}
	}
	if copies == 0 {
		t.Fatalf("precondition: Reverberate made no copy of Divination")
	}
	if got := triggers() - before; got != 1 {
		t.Fatalf("named spell plus its copy: %d triggers, want 1 (the cast only)", got)
	}
	if got := lifeBefore - life(1); got != 3 {
		t.Fatalf("life lost %d, want 3", got)
	}
}
