package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 508.1d's last sentence: a player is never required to pay a cost to
// attack, even to obey a requirement. A creature whose only offered pair
// carries a mana tax (Ghostly Prison) is therefore never Required: declaring
// it is always the controller's choice, and the empty declaration stays
// legal. Each test below pins one requirement kind (goad, a Broad MustAttack
// static, a Named one beside a free seat) on the same two-Mountain board,
// first asserting its preconditions (the requirement is live, the taxed pair
// is actually offered at its price) so a mis-set-up board fails loudly.
//
// The empty declaration is what the old reading rejected: it counted charged
// pairs as discharging a duty (attackDutyDischargeable, and
// attackOffersCompute's maximal-satisfaction best), so the {2} pair came out
// Required, RequiredQuota was 1, and Submit rejected
// "must attack with as many required creatures as possible (required 1,
// declared 0)".

// ghostlyPrisonTaxBoard is the repro board: Ghostly Prison on seat 0,
// two untapped Mountains on seat 1, seat 1 active at the declare-attackers
// step. The attacker is placed by the caller.
func ghostlyPrisonTaxBoard(t *testing.T) *Engine {
	t.Helper()
	e := New(Config{Seed: 716, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	onBoardCard(t, e, 0, corpusAlternativeCard(t, "Ghostly Prison"))
	for i := 0; i < 2; i++ {
		onBoardCard(t, e, 1, card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"))
	}
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	return e
}

// emptyAttackDeclaration submits the no-attack answer and fails the test when
// the engine rejects it.
func emptyAttackDeclaration(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if err := e.Submit(decision.Intent{Player: d.Player, Seq: d.Seq, Choices: []int{}}); err != nil {
		t.Fatalf("empty attack declaration rejected: %v", err)
	}
}

func TestGoadedCreatureMayDeclineGhostlyPrisonTax(t *testing.T) {
	t.Parallel()
	e := ghostlyPrisonTaxBoard(t)
	bear := onBoardReady(t, e, 1, "Name:Tax Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Goad, Obj: bear, Player: 0})
	// Precondition: the goad is live and the taxed pair is actually offered
	// at its {2} price (so "not Required" is a claim about a real option).
	rs := e.attackRequirements(bear)
	if !rs.Goad {
		t.Fatalf("precondition: bear has no active goad: %+v", rs)
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	var taxed *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Obj != bear {
			t.Fatalf("unexpected option for another object: %+v", o)
		}
		if o.Player == 0 && o.Value == 2 {
			taxed = o
		} else {
			t.Fatalf("unexpected pair shape, want exactly one {2}@seat0 option: %+v", o)
		}
	}
	if taxed == nil {
		t.Fatalf("precondition: no {2} pair at the taxed seat was offered: %+v", d.Options)
	}
	if taxed.Required {
		t.Fatalf("the goaded bear's only pair is taxed, so CR 508.1d's last sentence leaves the empty declaration legal and the option must not be Required: %+v", taxed)
	}
	if q := d.RequiredQuota(); q != 0 {
		t.Fatalf("RequiredQuota = %d, want 0: the goad cannot force a taxed attack", q)
	}
	emptyAttackDeclaration(t, e)
}

func TestMustAttackCreatureMayDeclineGhostlyPrisonTax(t *testing.T) {
	t.Parallel()
	e := ghostlyPrisonTaxBoard(t)
	dasher := onBoardReadyCard(t, e, 1, corpusAlternativeCard(t, "Valley Dasher"))
	// Precondition: the Dasher's face carries a BROAD MustAttack requirement
	// and the taxed pair is actually offered at its {2} price.
	rs := e.attackRequirements(dasher)
	if !rs.Broad {
		t.Fatalf("precondition: Valley Dasher's MustAttack static is not broad: %+v", rs)
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	var taxed *decision.Option
	for i := range d.Options {
		o := &d.Options[i]
		if o.Obj != dasher {
			t.Fatalf("unexpected option for another object: %+v", o)
		}
		if o.Player == 0 && o.Value == 2 {
			taxed = o
		} else {
			t.Fatalf("unexpected pair shape, want exactly one {2}@seat0 option: %+v", o)
		}
	}
	if taxed == nil {
		t.Fatalf("precondition: no {2} pair at the taxed seat was offered: %+v", d.Options)
	}
	if taxed.Required {
		t.Fatalf("the Dasher's only pair is taxed, so CR 508.1d's last sentence leaves the empty declaration legal and the option must not be Required: %+v", taxed)
	}
	if q := d.RequiredQuota(); q != 0 {
		t.Fatalf("RequiredQuota = %d, want 0: a MustAttack requirement cannot force a taxed attack", q)
	}
	emptyAttackDeclaration(t, e)
}

func TestNamedDutyAtTaxedSeatKeepsFreePair(t *testing.T) {
	t.Parallel()
	e := New(Config{Seed: 716, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	// Seat 2 taxes attacks against itself (Ghostly Prison) and names seat 1's
	// creatures as MustAttack$ You.
	onBoardCard(t, e, 2, corpusAlternativeCard(t, "Ghostly Prison"))
	onBoardCard(t, e, 2, card(t, "Name:Duty Totem\nManaCost:2\nTypes:Creature Golem\nPT:1/1\n"+
		"S:Mode$ MustAttack | ValidCreature$ Creature.OppCtrl | MustAttack$ You\nOracle:x\n"))
	for i := 0; i < 2; i++ {
		onBoardCard(t, e, 1, card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"))
	}
	bear := onBoardReady(t, e, 1, "Name:Tax Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	// Precondition: the bear carries a NAMED duty against seat 2 only, and
	// the taxed seat-2 pair is actually offered at its {2} price.
	rs := e.attackRequirements(bear)
	if rs.Named[2] != 1 || rs.Broad || rs.Goad {
		t.Fatalf("precondition: want exactly one named duty against seat 2, got %+v", rs)
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	free, taxed := 0, 0
	for i := range d.Options {
		o := &d.Options[i]
		if o.Obj != bear {
			t.Fatalf("unexpected option for another object: %+v", o)
		}
		switch {
		case o.Player == 0 && o.Value == 0:
			free++
		case o.Player == 2 && o.Value == 2:
			taxed++
		default:
			t.Fatalf("unexpected pair shape, want one free@seat0 and one {2}@seat2: %+v", o)
		}
		if o.Required {
			t.Fatalf("option %+v is Required: the free pair exists, so no requirement is charged, and CR 508.1d's last sentence keeps the empty declaration legal", o)
		}
	}
	if free != 1 || taxed != 1 {
		t.Fatalf("precondition: want exactly the free@seat0 and {2}@seat2 pairs, got %d free / %d taxed of %+v", free, taxed, d.Options)
	}
	if q := d.RequiredQuota(); q != 0 {
		t.Fatalf("RequiredQuota = %d, want 0: the named duty names only a taxed seat", q)
	}
	emptyAttackDeclaration(t, e)
}
