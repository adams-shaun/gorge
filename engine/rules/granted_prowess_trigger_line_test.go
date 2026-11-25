package rules

// A granted keyword trigger belongs to the RECIPIENT, not to whatever T: line
// its printed face happens to hold at index 0. Sokka, Tenacious Tactician's
// "Other Allies you control have menace and prowess" synthesizes a Prowess
// trigger for Momo, Playful Pet, whose only printed T: line is its
// leaves-the-battlefield trigger. The synthesized trigger must carry its own
// line for the ordering label and the CR 603.5 optionality read (a granted
// Prowess is mandatory), never Momo's printed line 0.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const grantedProwessSpell = "Name:Study\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// addCardToHand puts a freshly built card into p's hand. It is the hand-side
// twin of onBoardCard: a direct fixture placement for a card the test needs to
// cast, staling the derived memos the way an emitted event would.
func addCardToHand(t *testing.T, e *Engine, p state.PlayerID, c *cards.Card) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, p)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, p, append(e.G.Zone(state.ZHand, p), o.ID))
	e.staticEpoch = -1
	e.activeEpoch = -1
	return o.ID
}

// submitGrantedProwessCast answers the current priority decision with the cast option
// for id.
func submitGrantedProwessCast(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision before the cast")
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
}

// grantedProwessBoard seats Sokka and the given granted-keyword recipient on
// seat 0's battlefield with a one-mana noncreature spell in hand, and proves
// the grant reached the layer system before returning.
func grantedProwessBoard(t *testing.T, recipientCard *cards.Card) (e *Engine, recipient, spell state.ObjID) {
	t.Helper()
	e = newSeats(t, 2)
	sokka := onBoardCard(t, e, 0, corpusCard(t, "Sokka, Tenacious Tactician"))
	recipient = onBoardCard(t, e, 0, recipientCard)
	if sokka == 0 || recipient == 0 {
		t.Fatalf("setup cards missing: Sokka %d recipient %d", sokka, recipient)
	}
	if !e.HasKeyword(recipient, "Prowess") {
		t.Fatalf("precondition: recipient keywords %v lack Sokka's granted Prowess", e.Derived(recipient).Keywords)
	}
	spell = addCardToHand(t, e, 0, card(t, grantedProwessSpell))
	addMana(t, e, 0, "U")
	return e, recipient, spell
}

// TestGrantedProwessTriggerLabel is the xproof find's label binding: Sokka
// grants Momo Prowess, and the KTriggerOrder option for Momo's granted
// trigger must carry the Prowess line, not Momo's printed leaves-the-
// battlefield TriggerDescription$ (which the Forge driver cannot bind).
func TestGrantedProwessTriggerLabel(t *testing.T) {
	t.Parallel()
	e, momo, spell := grantedProwessBoard(t, corpusCard(t, "Momo, Playful Pet"))
	// Precondition: Momo's printed line 0 is the leaves-the-battlefield
	// trigger -- the wrong line the granted trigger used to borrow.
	trig := e.G.Obj(momo).Face().Triggers
	if len(trig) == 0 || !strings.Contains(trig[0].ParamStr(cards.PKTriggerDescription), "leaves the battlefield") {
		t.Fatalf("precondition: Momo's printed trigger 0 is not the leaves-the-battlefield line: %+v", trig)
	}
	submitGrantedProwessCast(t, e, spell)
	od := e.Pending()
	if od == nil || od.Kind != decision.KTriggerOrder {
		t.Fatalf("want the trigger-order ask after casting a noncreature spell, got %+v", od)
	}
	var labels []string
	for _, o := range od.Options {
		if o.Obj == momo {
			labels = append(labels, o.Label)
		}
	}
	if len(labels) != 1 {
		t.Fatalf("Momo's granted Prowess has %d ordering options, want 1: %+v", len(labels), od.Options)
	}
	const want = "Momo, Playful Pet: Prowess"
	if labels[0] != want {
		t.Fatalf("granted Prowess label = %q, want %q (Momo's printed T: line 0 is the leaves-the-battlefield trigger)", labels[0], want)
	}
}

// TestGrantedProwessNotOptionalOnOptionalRecipient pins CR 603.5's
// mandatory shape: a granted Prowess is not optional, even when the
// recipient's printed T: line 0 carries OptionalDecider$ You.
func TestGrantedProwessNotOptionalOnOptionalRecipient(t *testing.T) {
	t.Parallel()
	const optionalAllySrc = "Name:Optional Ally\nTypes:Creature Ally\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | OptionalDecider$ You | Execute$ OptFixtureDraw | TriggerDescription$ Fixture optional reminder.\n" +
		"SVar:OptFixtureDraw:DB$ Draw | NumCards$ 1\n" +
		"Oracle:x\n"
	e, ally, spell := grantedProwessBoard(t, card(t, optionalAllySrc))
	// Precondition: the recipient's printed line 0 really is optional.
	trig := e.G.Obj(ally).Face().Triggers
	if len(trig) == 0 || trig[0].ParamStr(cards.PKOptionalDecider) != "You" {
		t.Fatalf("precondition: recipient's printed trigger 0 is not OptionalDecider$ You: %+v", trig)
	}
	submitGrantedProwessCast(t, e, spell)
	var found bool
	for _, pt := range e.PendingTriggers() {
		if pt.Source != ally {
			continue
		}
		found = true
		if pt.Optional {
			t.Fatalf("granted Prowess on an optional printed line 0 is posed optional (decider %d), want mandatory", pt.Decider)
		}
		if pt.Label != "Optional Ally: Prowess" {
			t.Fatalf("granted Prowess label = %q, want %q", pt.Label, "Optional Ally: Prowess")
		}
	}
	if !found {
		t.Fatalf("no pending trigger for the granted-Prowess recipient: %+v", e.PendingTriggers())
	}
	// Drain to the empty stack; a mandatory trigger never poses
	// KTriggerOptional on the way.
	for i := 0; i < 50 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack depth %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			t.Fatalf("mandatory granted Prowess posed a trigger-optional ask: %+v", d)
		case decision.KTriggerOrder:
			idx := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				idx = append(idx, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: idx}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected %v decision while draining: %+v", d.Kind, d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack never emptied (depth %d)", len(e.G.Stack))
	}
}
