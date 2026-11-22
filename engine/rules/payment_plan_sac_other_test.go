package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A mana ability whose cost sacrifices ANOTHER permanent (Diamond
// Kaleidoscope's "Sacrifice a Prism token: Add one mana of any color", CR
// 605.3a, 601.2g) is a payment-plan last-resort shape the planner prices: the
// witness discloses that the activation consumes a permanent, the census
// offers the ability once per candidate victim (a no-tap ability may be
// activated once per victim its cost can take), and the executor elects the
// victim through the ordinary mana-cost election. Measured from the lockstep
// pair behind dirac/oracle/testdata/repro/sac_other_mana_source.repro.json:
// the {2}{B}{R}, {T}, sacrifice-self Tramway draw was withheld although the
// two Prism tokens beside the Kaleidoscope fund the pips.

const (
	// psoKaleidoscope is the Diamond Kaleidoscope shape: a sac-other,
	// no-tap, any-colour mana ability.
	psoKaleidoscope = "Name:Test Kaleidoscope\nManaCost:4\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ Sac<1/Prism.token/Prism token> | Produced$ Any | SpellDescription$ Add one mana of any color.\nOracle:x\n"
	// psoTramway is the Tramway Station shape: a {2}{B}{R}, {T},
	// sacrifice-self draw beside a plain mana tap.
	psoTramway = "Name:Test Tramway\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add colourless.\n" +
		"A:AB$ Draw | Cost$ 2 B R T Sac<1/CARDNAME> | SpellDescription$ Draw a card.\nOracle:x\n"
	// psoTramwayCheap is psoTramway with a {1}{B} draw so a UNIQUE victim's
	// one Kaleidoscope activation funds it.
	psoTramwayCheap = "Name:Test Tramway\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add colourless.\n" +
		"A:AB$ Draw | Cost$ 1 B T Sac<1/CARDNAME> | SpellDescription$ Draw a card.\nOracle:x\n"
	psoPlains = "Name:Test Plains\nTypes:Land Plains\nA:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Add white.\nOracle:x\n"
	psoIsland = "Name:Test Island\nTypes:Land Island\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ Add blue.\nOracle:x\n"
	// psoPrism is the Prism token the Kaleidoscope's cost sacrifices. The
	// token itself has no abilities.
	psoPrism            = "Name:Prism Token\nTypes:Artifact Creature Prism\nPT:0/1\nOracle:x\n"
	psoPrismTokenScript = "c_0_1_a_prism"
)

// psoDeck builds a seat-0 engine holding the named cards in its deck and the
// Prism token script in its token registry, stopped at seat-0 priority.
func psoDeck(t *testing.T, seed uint64, fixtureSrc string, extras ...string) (*Engine, Config) {
	t.Helper()
	fixture := card(t, fixtureSrc)
	name := fixture.Faces[0].Name
	deck := append([]*cards.Card{fixture}, mountainDeck(t, 1)...)
	for _, extra := range extras {
		deck = append(deck, card(t, extra))
	}
	build := func(s uint64) Config {
		return Config{Seed: s, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{
				append(deck, mountainDeck(t, 40-len(deck))...),
				mountainDeck(t, 40),
			},
			Tokens: map[string]*cards.Card{psoPrismTokenScript: card(t, psoPrism)},
		}
	}
	cfg := seatZeroStart(build(seed))
	e := New(cfg)
	e.Advance()
	var id state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(cand).Face().Name == name {
			id = cand
		}
	}
	if id == 0 {
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == name {
				id = cand
			}
		}
		if id == 0 {
			t.Fatalf("fixture %q not found in seat 0's hand or library", name)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}
	return e, cfg
}

// psoBoard places every named card on seat 0's battlefield untapped and
// unsick, and returns their object IDs in the order given.
func psoBoard(t *testing.T, e *Engine, srcs ...string) []state.ObjID {
	t.Helper()
	var out []state.ObjID
	for _, s := range srcs {
		id := onBoard(t, e, 0, s)
		e.G.Obj(id).SummonSick = false
		out = append(out, id)
	}
	return out
}

// psoPrisms mints n Prism tokens onto seat 0's battlefield and returns their
// object IDs in mint order.
func psoPrisms(t *testing.T, e *Engine, n int) []state.ObjID {
	t.Helper()
	var out []state.ObjID
	for i := 0; i < n; i++ {
		e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: psoPrismTokenScript})
		zone := e.G.Zone(state.ZBattlefield, 0)
		out = append(out, zone[len(zone)-1])
	}
	return out
}

// psoAbilityOption finds the priority walk's non-mana "ability" option for
// src's printed ability, or nil.
func psoAbilityOption(d *decision.Decision, src state.ObjID) *decision.Option {
	for i := range d.Options {
		if o := &d.Options[i]; o.Kind == "ability" && o.Obj == src {
			return o
		}
	}
	return nil
}

// TestPaymentPlanSacsOtherManaSourceIsOffered prices the {2}{B}{R}, {T},
// sacrifice-self draw over two Kaleidoscope activations (one Prism token
// each, CR 605.3a) and two untapped basics: the priority walk must offer it
// as the source's "ability" option, exactly as the kernel did in the
// recorded pair.
func TestPaymentPlanSacsOtherManaSourceIsOffered(t *testing.T) {
	t.Parallel()
	e, _ := psoDeck(t, 4120, ppFiller, psoTramway, psoKaleidoscope, psoPlains, psoIsland)
	toMain1(t, e)
	e.paidManaCensus = true
	board := psoBoard(t, e, psoTramway, psoKaleidoscope, psoPlains, psoIsland)
	tram := board[0]
	psoPrisms(t, e, 2)
	d := ppPriority(t, e)
	if opt := psoAbilityOption(d, tram); opt == nil {
		t.Fatalf("the sac-other funded draw is not offered: %+v", d.Options)
	}
}

// TestPaymentPlanSacsOtherUniqueVictimPays runs the whole activation with a
// UNIQUE victim: the Kaleidoscope's one Prism funds the {B}, one untapped
// basic pays the {1}, and the executor's forced election sacrifices the
// Prism without an ask. The sac-other source itself must survive (CR 605.3a
// sacrifices only the Prism), and the tramway must not.
func TestPaymentPlanSacsOtherUniqueVictimPays(t *testing.T) {
	t.Parallel()
	e, _ := psoDeck(t, 4121, ppFiller, psoTramwayCheap, psoKaleidoscope, psoPlains, psoIsland)
	toMain1(t, e)
	e.paidManaCensus = true
	board := psoBoard(t, e, psoTramwayCheap, psoKaleidoscope, psoPlains, psoIsland)
	tram, kaleido, plains, island := board[0], board[1], board[2], board[3]
	prisms := psoPrisms(t, e, 1)
	d := ppPriority(t, e)
	opt := psoAbilityOption(d, tram)
	if opt == nil {
		t.Fatalf("the unique-victim draw is not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatal(err)
	}
	// The plan pays synchronously: the Prism election is forced (one
	// candidate), so no ask may strand the seat before the priority returns.
	for i := 0; ; i++ {
		if i == 40 {
			t.Fatalf("the activation did not settle: %+v", e.Pending())
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision; cast pending = %v", e.cast != nil)
		}
		if d.PaymentFallback != nil {
			t.Fatalf("payment fell back: %+v at %q", *d.PaymentFallback, d.Prompt)
		}
		if d.Kind == decision.KPriority {
			break
		}
		if d.Min == 0 && d.Max == 0 {
			t.Fatalf("asked %q with an empty answer range", d.Prompt)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
			t.Fatalf("answer %q: %v", d.Prompt, err)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != 8 {
		t.Fatalf("seat 0 holds %d cards, want the drawn 8", got)
	}
	if e.G.Obj(tram).Zone == state.ZBattlefield {
		t.Fatalf("the tramway survived its own sacrifice")
	}
	if e.G.Obj(kaleido).Zone != state.ZBattlefield {
		t.Fatalf("the sac-other source was consumed: CR 605.3a sacrifices only the Prism")
	}
	if e.G.Obj(prisms[0]).Zone == state.ZBattlefield {
		t.Fatalf("the Prism token was never sacrificed")
	}
	tapped := 0
	for _, id := range []state.ObjID{plains, island} {
		if e.G.Obj(id).Tapped {
			tapped++
		}
	}
	if tapped != 1 {
		t.Fatalf("tapped %d basics, want exactly the witness's 1", tapped)
	}
}
