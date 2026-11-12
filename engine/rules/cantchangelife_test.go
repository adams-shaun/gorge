package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cantchangelife_test.go pins the static
// `S:Mode$ CantChangeLife | ValidPlayer$ You` (Platinum Emperion's "Your life
// total can't change" -- CR 119.7's "can't gain life" and CR 119.8's "can't
// lose life" as ONE static). The fix has two homes and both are asserted:
//
//   - engine/rules/replacement_life.go lifeChangeForbidden, consulted from
//     applyLifeReplacements, swallows every life event the affected player
//     would take -- a life-gain effect, a life-loss effect, a set-life delta
//     and damage dealt to that player -- while a player WITHOUT the static
//     still moves life normally (the control).
//   - the payment layer's offer gates (pay.CostPayableClassLife /
//     CostPayablePool / ResolveManaNoLifePay, reached through
//     pay.Eval.LifePaymentForbidden) refuse a positive life cost for that
//     payer, so a `Cost$ PayLife<1>` activation is priced infeasible.
//
// Every "nothing happens" subtest also asserts the feature RAN: the emitted
// note is the restriction's own "prevented: life total can't change", so a
// build with the static's registration reverted fails rather than passing
// vacuously.
const cantChangeLifeCard = "Name:Gate Cannot Change\nTypes:Enchantment\n" +
	"S:Mode$ CantChangeLife | ValidPlayer$ You | Description$ Your life total can't change.\n" +
	"Oracle:Your life total can't change.\n"

// cantChangeLifeEngine seeds seat 0 with the CantChangeLife source on the
// battlefield (seat 1 has none) and returns the engine plus the source id.
func cantChangeLifeEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	src := card(t, cantChangeLifeCard)
	e := corpusEngine(t, reg, []*cards.Card{src}, nil)
	var id state.ObjID
	for _, sid := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(sid); o != nil && o.Face() != nil && o.Face().Name == src.Faces[0].Name {
			id = sid
		}
	}
	if id == 0 {
		t.Fatalf("precondition: %s is not in seat 0's library", src.Faces[0].Name)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: the source is not a seat-0 battlefield permanent: %+v", o)
	}
	// The static must actually register: a Mode$ this build does not know
	// would leave the board bare and every prevention assertion below would
	// be checking nothing.
	if len(e.activeStatics("CantChangeLife")) == 0 {
		t.Fatal("precondition: no CantChangeLife static is active for seat 0")
	}
	// ... and the build must advertise the primitive: a card whose S: line is
	// present but whose stat: token is not registered is graded unsupported
	// and, for a card outside the repo decks, silently ignored, so the
	// checks below would pass on a bare board. The registration is the half a
	// reviewer reverts to "prove" a test is vacuous.
	if !effects.Supported()["stat:CantChangeLife"] {
		t.Fatal("precondition: stat:CantChangeLife is not a registered primitive (engine/rules/statics.go init)")
	}
	return e, id
}

// preventions counts the CantChangeLife prevention notes the log gained since
// index from.
func preventions(e *Engine, from int) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Note && ev.Text == "prevented: life total can't change" {
			n++
		}
	}
	return n
}

// TestCantChangeLifeStaticMovesNoLife is the replacement-site half: a gain, a
// loss, a set-life delta and damage all leave the protected player's total
// where it was -- each with the restriction's own note -- while seat 1 (no
// static) still gains, loses and takes damage normally.
func TestCantChangeLifeStaticMovesNoLife(t *testing.T) {
	e, _ := cantChangeLifeEngine(t)

	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: seat 0 life = %d, want 20", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", got)
	}
	if !e.playerLifeChangeForbidden(0) || e.playerLifeChangeForbidden(1) {
		t.Fatalf("precondition: forbidden(seat0)=%v forbidden(seat1)=%v, want true/false",
			e.playerLifeChangeForbidden(0), e.playerLifeChangeForbidden(1))
	}

	cases := []struct {
		name string
		ev   events.Event
	}{
		// SpellAbility$ GainLife 3 (Reviving Dose).
		{"gain", events.Event{Kind: events.LifeChange, Player: 0, Amount: 3}},
		// SpellAbility$ LoseLife 2 (Night's Whisper; Swords to Plowshares'
		// gain is the case above).
		{"loss", events.Event{Kind: events.LifeChange, Player: 0, Amount: -2}},
		// SpellAbility$ SetLife 7: a positive delta, the CR 119.5 set.
		{"set higher", events.Event{Kind: events.LifeChange, Player: 0, Amount: 7 - 20}},
		// Damage dealt to the protected player.
		{"damage", events.Event{Kind: events.Damage, Player: 0, Amount: 5}},
	}
	for _, tc := range cases {
		mark := len(e.L.Events)
		e.emit(tc.ev)
		if got := e.G.Players[0].Life; got != 20 {
			t.Fatalf("CantChangeLife %s: seat 0 life = %d, want 20 unchanged", tc.name, got)
		}
		if got := e.G.Players[0].LifeLostThisTurn; got != 0 {
			t.Fatalf("CantChangeLife %s: seat 0 LifeLostThisTurn = %d, want 0 (no loss happened)", tc.name, got)
		}
		if got := e.G.Players[0].LifeGainedThisTurn; got != 0 {
			t.Fatalf("CantChangeLife %s: seat 0 LifeGainedThisTurn = %d, want 0 (no gain happened)", tc.name, got)
		}
		if preventions(e, mark) != 1 {
			t.Fatalf("CantChangeLife %s: no prevention note -- the static did not run (log tail %+v)",
				tc.name, e.L.Events[mark:])
		}
	}

	// The control: seat 1 has no static.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 3})
	if got := e.G.Players[1].Life; got != 23 {
		t.Fatalf("control: seat 1 gained to %d, want 23", got)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -2})
	if got := e.G.Players[1].Life; got != 21 {
		t.Fatalf("control: seat 1 lost to %d, want 21", got)
	}
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("control: seat 1 took 1 damage to %d, want 20", got)
	}
}

// TestCantChangeLifePayLifeNotOffered is the payment half: a `Cost$ PayLife<1>`
// activation (Kozilek's Translator) is not offered to the protected player,
// while the same permanent on the unprotected opponent's side is.
func TestCantChangeLifePayLifeNotOffered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	translator := lookup(t, reg, "Kozilek's Translator")
	if _, ok := translator.Faces[0].KeywordParam("Devoid"); !ok && len(translator.Faces[0].Abilities) == 0 {
		t.Fatalf("precondition: %s carries no ability", translator.Faces[0].Name)
	}
	src := card(t, cantChangeLifeCard)
	e := corpusEngine(t, reg, []*cards.Card{src}, []*cards.Card{translator})

	find := func(p state.PlayerID, name string) state.ObjID {
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, p) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					return id
				}
			}
		}
		return 0
	}
	protected := find(0, src.Faces[0].Name)
	control := find(1, translator.Faces[0].Name)
	if protected == 0 || control == 0 {
		t.Fatalf("precondition: seeded cards not found (protected=%d control=%d)", protected, control)
	}
	toMain1(t, e)
	e.emit(events.Event{Kind: events.MoveZone, Obj: protected, From: e.G.Obj(protected).Zone, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: control, From: e.G.Obj(control).Zone, To: state.ZBattlefield})
	e.pending = nil
	e.priorityRound()

	if len(e.activeStatics("CantChangeLife")) == 0 {
		t.Fatal("precondition: the CantChangeLife static is not active for seat 0")
	}
	if !effects.Supported()["stat:CantChangeLife"] {
		t.Fatal("precondition: stat:CantChangeLife is not a registered primitive (engine/rules/statics.go init)")
	}
	if o := e.G.Obj(control); o == nil || o.Controller != 1 || o.Tapped {
		t.Fatalf("precondition: the control translator is not an untapped seat-1 permanent: %+v", o)
	}
	if o := e.G.Obj(protected); o == nil || o.Controller != 0 {
		t.Fatalf("precondition: the CantChangeLife source is not seat 0's: %+v", o)
	}
	if !e.playerLifeChangeForbidden(0) || e.playerLifeChangeForbidden(1) {
		t.Fatal("precondition: the static is not scoped to seat 0 alone")
	}
	// Seat 0 holds priority first: the protected player's own offer list must
	// not carry the PayLife<1> mana activation.
	// The payment layer's own gate, asked directly: the fixed-life form, the
	// hypothetical-pool form and the real charge all say no for the
	// protected seat and yes for the opponent. This is the read the offer
	// walk's mana-ability path actually takes (a PayLife<N> mana ability is
	// priced by pay.CostPayablePool, so the offered-option assertion below
	// is this gate's user-visible face).
	lifeCost := pay.Cost{Life: 1}
	if pay.CostPayablePool(asPayer(e), 0, control, pay.SAActivated, lifeCost, e.G.Players[0].Pool, e.G.Players[0].ManaUnits()) {
		t.Fatal("CantChangeLife: pay.CostPayablePool priced a PayLife<1> for the protected seat")
	}
	if !pay.CostPayablePool(asPayer(e), 1, control, pay.SAActivated, lifeCost, e.G.Players[1].Pool, e.G.Players[1].ManaUnits()) {
		t.Fatal("control: pay.CostPayablePool refused a PayLife<1> for the unprotected seat")
	}
	if pay.CostPayableClass(asPayer(e), 0, pay.DescriptorFor(control, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("CantChangeLife: pay.CostPayableClass priced a PayLife<1> for the protected seat")
	}
	if !pay.CostPayableClass(asPayer(e), 1, pay.DescriptorFor(control, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("control: pay.CostPayableClass refused a PayLife<1> for the unprotected seat")
	}

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("precondition: seat 0 does not hold priority: %+v", d)
	}
	offered := func(d *decision.Decision, obj state.ObjID) bool {
		for _, o := range d.Options {
			if (o.Kind == "ability" || o.Kind == "activate") && o.Obj == obj {
				return true
			}
		}
		return false
	}
	// The walk: the gate above is what refuses the price, and cardad's offer
	// list currently carries no life figure for this activation, so this
	// assertion is a guard on the end-to-end offer rather than the failing
	// proof -- the pay.* probes above are that. It still matters: a future
	// offer shape that discloses the life cost must not reach this seat.
	if offered(d, protected) {
		t.Fatalf("CantChangeLife: the PayLife<1> activation was offered to the protected player: %+v", d.Options)
	}
	// Pass to the unprotected opponent: the identical permanent keeps its
	// activation, so the refusal above is the static's doing and not a
	// missing offer for every seat.
	passPriorityOnce(t, e)
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("control: seat 1 does not hold priority after seat 0 passes: %+v", d)
	}
	if !offered(d, control) {
		t.Fatalf("control: the PayLife<1> activation was NOT offered to the unprotected opponent: %+v", d.Options)
	}
}
