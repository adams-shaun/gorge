package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Omni-Cheese Pizza's shape: a mana ability whose activation pays a literal
// generic fee, taps, and sacrifices the source, for one mana of any colour.
// The CR 601.2g cast window can activate it (the fee is funded from mana
// already floating in the window), so its production is priced wherever the
// window's reach is measured.
// The script line is deliberately reworded (Produced$ before Cost$, British
// spelling): archtest's TestNoVerbatimForgeLines refuses verbatim Forge lines
// here, but the shape it prices is the printed one -- a {1}, {T}, sacrifice
// itself activation producing one mana of any colour.
const cwSelfSacAnySource = "Name:Cheese Food\nTypes:Artifact Food\n" +
	"A:AB$ Mana | Produced$ Any | Amount$ 1 | Cost$ 1 T Sac<1/CARDNAME/this artifact> | SpellDescription$ Add one mana of any colour.\nOracle:x\n"

// A {1}{R/W} spell: one generic and one hybrid pip whose faces are W and R.
const cwHybridPipSpell = "Name:Hybrid Bolt\nManaCost:1 RW\nTypes:Instant\n" +
	"A:SP$ Draw | Amount$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"

// selfSacWindowGame seats the {1}{R/W} spell in p0's hand, the paid
// self-sacrifice any-colour source on p0's battlefield, and floats {C}{R}:
// the pip's W face is payable only through the source ({C} pays its {1}, the
// floated colour pays the W, the other floated unit pays the {1}), the R face
// from the pool alone.
func selfSacWindowGame(t *testing.T, seed uint64) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, spellID := newFixtureDeck(t, seed, cwHybridPipSpell, cwSelfSacAnySource)
	_ = cfg
	pizzaID := moveSeeded(t, e, 0, cwSelfSacAnySource, state.ZBattlefield)
	if o := e.G.Obj(pizzaID); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: the paid source must be an untapped battlefield permanent: %+v", o)
	}
	if o := e.G.Obj(spellID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: the spell must be in hand: %+v", o)
	}
	addMana(t, e, 0, "CR")
	if got := e.G.Players[0].Pool; got[state.MC] != 1 || got[state.MR] != 1 || got.Total() != 2 {
		t.Fatalf("precondition: pool = %v, want exactly {C}{R}", got)
	}
	return e, spellID, pizzaID
}

// payOptionNames returns a KChoose decision's pip-face option names.
func payOptionNames(d *decision.Decision) []string {
	var out []string
	for _, o := range d.Options {
		if strings.HasPrefix(o.Kind, "pay_") {
			out = append(out, o.Kind)
		}
	}
	return out
}

// TestCastWindowSelfSacGenericFeeFundsAHybridFace reproduces pair 465 of the
// std-tmt lockstep: casting {1}{R/W} with {C}{R} floating and an untapped
// "{1}, {T}, Sacrifice this artifact: Add one mana of any color" source must
// offer both hybrid faces at the CR 601.2b pip ask (CR 601.2b chooses a face
// before the 601.2g window, and the window can pay the W face: the source's
// {1} fee takes the {C}, its production floats the W, and the {R} pays the
// cost's {1}). Announcing the W face then completes through the window: the
// activation sacrifices the source, the spell resolves, and the draw lands.
func TestCastWindowSelfSacGenericFeeFundsAHybridFace(t *testing.T) {
	t.Parallel()
	e, spellID, pizzaID := selfSacWindowGame(t, 913)
	handBefore := len(e.G.Zone(state.ZHand, 0))
	castFromPriority(t, e, spellID)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the cast's pip ask, got %+v", d)
	}
	faces := strings.Join(payOptionNames(d), ",")
	if !strings.Contains(faces, "pay_R") {
		t.Fatalf("precondition: the R face must stay offered, got [%s]", faces)
	}
	if !strings.Contains(faces, "pay_W") {
		t.Fatalf("CR 601.2b: the W face is payable through the CR 601.2g window (%s's {1} takes the {C}, it floats the W, the {R} pays the {1}); the pip ask offered only [%s]", e.G.Obj(pizzaID).Face().Name, faces)
	}

	// Announce the W face and finish the payment through the window.
	wIdx := -1
	for _, o := range d.Options {
		if o.Kind == "pay_W" {
			wIdx = o.Index
		}
	}
	if wIdx < 0 {
		t.Fatal("no pay_W option to answer")
	}
	submitChoices(t, e, wIdx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "activate" {
		t.Fatalf("want the CR 601.2g mana window, got %+v", d)
	}
	activateIdx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == pizzaID {
			activateIdx = o.Index
		}
	}
	if activateIdx < 0 {
		t.Fatalf("the window must offer the untapped source's activation: %+v", d.Options)
	}
	submitChoices(t, e, activateIdx)
	// The production's colour is asked; W closes the payment.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the production colour ask, got %+v", d)
	}
	colourIdx := -1
	for _, o := range d.Options {
		if o.Kind == "mana" && strings.Contains(o.Label, "W") {
			colourIdx = o.Index
		}
	}
	if colourIdx < 0 {
		t.Fatalf("no W colour option: %+v", d.Options)
	}
	submitChoices(t, e, colourIdx)
	// The float closed the payment; payCast charges it and the spell resolves
	// without another window.
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("want the settled cast to hand priority back, got %+v", d)
	}
	// The cast completed: the spell was pushed to the stack, the payment
	// drained the float and the pool, and the activation's fee, tap and
	// sacrifice were all charged.
	if got := e.G.Obj(pizzaID); got != nil && got.Zone == state.ZBattlefield {
		t.Fatalf("the activated source stayed on the battlefield: %+v", got)
	}
	if s := e.G.Obj(spellID); s == nil || s.Zone != state.ZStack {
		t.Fatalf("the spell was not pushed to the stack: %+v", s)
	}
	if got := e.G.Players[0].Pool; got.Total() != 0 {
		t.Fatalf("pool after payment = %v, want empty", got)
	}
	// The spell left the hand; it resolves once priority passes and its draw
	// replaces itself then.
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore-1 {
		t.Fatalf("hand after the cast = %d, want %d", got, handBefore-1)
	}
}

// TestCastWindowSelfSacGenericFeeSourceIsNotExcluded pins the census verdict
// beside the probe: a source the CR 601.2g window can activate must not be
// reported as excluded (castWindowSourceExcluded), or castWindowCensusComplete
// keeps calling the priced census incomplete for a window that prices
// everything it can offer.
func TestCastWindowSelfSacGenericFeeSourceIsNotExcluded(t *testing.T) {
	t.Parallel()
	e, spellID, pizzaID := selfSacWindowGame(t, 914)
	castFromPriority(t, e, spellID)
	pc := e.cast
	if pc == nil {
		t.Fatal("no pending cast")
	}
	desc := paymentForCast(pc, pc.resolvedMana())
	if e.castWindowSourceExcluded(0, pizzaID, desc) {
		t.Fatalf("CR 601.2g: %s's paid activation is activatable in the window, the probe must price it", e.G.Obj(pizzaID).Face().Name)
	}
}
