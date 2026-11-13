package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBotRequiredAttackDeclarationFitsCombinedPhyrexianTax is a
// reproduction: the combined Phyrexian attack tax must be
// priced on the REQUIRED path too, not just the optional one. With Norn's
// Annex out beside seat 1 and three creatures that each MUST attack (CR
// 508.1d, their own `S:Mode$ MustAttack | ValidCreature$ Card.Self` static)
// at four life, the engine's RequiredQuota used to count all three required
// pairs -- it priced only Option.Value/MaxSum, never the wire's
// CostPhyrexian -- so the production policy's charge guard trimmed the
// declaration to two and FitRequired restored the third, and Submit rejected
// the whole declaration ("... 3 Phyrexian, unpriceable=false) is not
// payable"). host.runMatch turns that rejection into a crashed table. The
// quota and FitRequired derive from the same charge-feasible required core,
// so the policy's declaration is accepted.
//
// The board is three seats so the fixture reads CR 508.1d's last sentence
// from the required side too: each bear is offered both a pip-taxed pair at
// the Annex's seat 1 and a FREE pair at untaxed seat 2. All pairs stay
// Required (a required creature's charged pairs remain Required -- the free
// duty-discharging pair exists), the quota is charge-feasible at zero
// Phyrexian, and the production bot's answer is accepted by Submit while
// covering the quota at a summed CostPhyrexian that the four-life bound can
// carry. The test first asserts its preconditions (all six pairs offered and
// Required, the Annex pairs carrying one pip each, seat 0 at four life), so a
// mis-set-up board fails loudly instead of passing vacuously.
func TestBotRequiredAttackDeclarationFitsCombinedPhyrexianTax(t *testing.T) {
	t.Parallel()
	cfg := seatZeroStart(Config{Seed: 11, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	onBoardCard(t, e, 1, corpusCard(t, "Norn's Annex"))
	var bears []state.ObjID
	for i := 0; i < 3; i++ {
		id := onBoard(t, e, 0, "Name:Tax Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\n"+
			"S:Mode$ MustAttack | ValidCreature$ Card.Self\nOracle:x\n")
		e.G.Obj(id).SummonSick = false
		bears = append(bears, id)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 4 - e.G.Players[0].Life})
	passToKind(t, e, decision.KAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's attackers", d)
	}
	// Precondition: three pip-taxed seat-1 pairs and three free seat-2 pairs,
	// every one of them Required (the free pair discharges the requirement, so
	// the creature is required; its taxed pair stays Required with it).
	perBear := make(map[state.ObjID]int)
	annexed := 0
	for _, o := range d.Options {
		if !o.Required {
			t.Fatalf("option %+v is not Required, want the MustAttack requirement on every pair", o)
		}
		perBear[o.Obj]++
		if o.Player == 1 {
			if o.CostPhyrexian != 1 {
				t.Fatalf("seat-1 option %d CostPhyrexian = %d, want 1 (the wire must publish the pip)", o.Index, o.CostPhyrexian)
			}
			annexed++
		}
	}
	if annexed != 3 {
		t.Fatalf("%d pip-taxed seat-1 pairs offered, want 3 (Norn's Annex beside seat 1)", annexed)
	}
	for _, b := range bears {
		if perBear[b] != 2 {
			t.Fatalf("bear %d has %d offered pairs, want 2 (taxed seat 1 and free seat 2): %+v", b, perBear[b], d.Options)
		}
	}
	if got := e.G.Players[0].Life; got != 4 {
		t.Fatalf("seat 0 life = %d, want 4 (the bound only binds there)", got)
	}
	if d.PayerLife != 4 {
		t.Fatalf("decision.PayerLife = %d, want 4 (the engine must publish the bound)", d.PayerLife)
	}
	q := d.RequiredQuota()
	in := newTestBot(7).answer(e, d)
	n := d.RequiredChosen(in.Choices)
	if err := e.Submit(in); err != nil {
		t.Fatalf("production policy declared %v (%d required covered, life %d): Submit rejected it: %v",
			in.Choices, n, e.G.Players[0].Life, err)
	}
	if n < q {
		t.Fatalf("policy declared %v covering %d required creatures, the engine demands %d", in.Choices, n, q)
	}
	byIndex := make(map[int]*decision.Option)
	for i := range d.Options {
		byIndex[d.Options[i].Index] = &d.Options[i]
	}
	pips := 0
	for _, c := range in.Choices {
		if o := byIndex[c]; o == nil {
			t.Fatalf("declaration names unknown option %d: %+v", c, in.Choices)
		} else {
			pips += int(o.CostPhyrexian)
		}
	}
	if pips > 2 {
		t.Fatalf("policy declared %v for %d Phyrexian pips; at 4 life the charge-feasible quota must be coverable at <= 2 (2 pips x 2 life)", in.Choices, pips)
	}
}
