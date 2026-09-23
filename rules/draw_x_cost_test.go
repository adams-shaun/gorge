package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// cost:Draw — the X-form Draw cost (Forge's Cost$ Draw<X/Spec>).
//
// The literal form Draw<N/Spec> was already modelled; the dynamic form
// Draw<X/Spec> (9 corpus carriers, all Draw<X/You>, all with a card-level
// SVar:X body) used to fall through nonManaCost to the unrecognised-symbol
// fallback, which substituted one generic mana and reported the head as the
// parameter census's `cost:Draw` label. It is now parsed as a Draw part whose
// Dyn names the source SVar, and resolved at payment by drawCostCount /
// drawCostCountTrig (the twin of fixLifeXCost for PayLife<X>).
//
// These tests pin the three legs the ticket names:
//
//   - the parse (no Unknown, no generic mana, one Dyn part) -- also the
//     param-census row {"Draw<X/You>", nil} in paramcensus_test.go;
//   - the fold (a resolvable SVar:X resolves to its count) and the
//     fail-closed withhold (an absent/unresolvable body is not offered);
//   - the end-to-end real-corpus trigger window on Titan of Littjara, the
//     ticket's canonical carrier.
//
// Champion of Wits (SVar:X:Count$CardPower) is the fold's positive control:
// its body resolves to the permanent's own power. Titan of Littjara
// (SVar:X:Count$Valid Creature.YouCtrl+Other+sharesCreatureTypeWith) is the
// trigger-window pin. Neither card is in a repo deck, so no chain head or
// acceptance-ratchet row depends on these tests.

// drawCostPart is the parsed shape every Draw<X/...> carrier produces.
func drawCostPart() CostPart { return CostPart{Dyn: "X", Spec: "You"} }

// TestDrawXCostSVarFoldsAndDraws pins the fold: a Draw<X/You> part on a face
// whose SVar:X resolves yields exactly that count, and the trigger window
// offers the pay election (so the handler is provably reached).
func TestDrawXCostSVarFoldsAndDraws(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Champion of Wits")
	id := searchMoveByName(t, e, "Champion of Wits", state.ZBattlefield)

	// Precondition: the moved permanent really is on the battlefield with a
	// resolvable SVar:X body. A vacuous fixture (wrong zone, absent body)
	// must fail here, not pass the assertion below silently.
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		t.Fatalf("Champion of Wits has no face after the move")
	}
	if !onBattlefield(e, id) {
		t.Fatalf("Champion of Wits is not on the battlefield; the fold's source is absent")
	}
	if body, ok := o.Face().SVars["X"]; !ok || body == "" {
		t.Fatalf("Champion of Wits has no SVar:X body; the fold has nothing to resolve")
	}

	n, ok := e.drawCostCount(id, 0, drawCostPart())
	if !ok {
		t.Fatalf("drawCostCount refused Champion of Wits' resolvable SVar:X")
	}
	if want := int32(e.Derived(id).Power); n != want {
		t.Fatalf("drawCostCount = %d, want Champion of Wits' derived power %d", n, want)
	}
	if n <= 0 {
		t.Fatalf("drawCostCount = %d; the fixture must resolve to a positive count", n)
	}

	// The window must actually offer the election, or the fold is unreachable.
	pay := witsPayAsk(t, e)
	if witsPayOption(pay) < 0 {
		t.Fatalf("the resolvable Draw<X/You> cost was not offered as payable: %+v", pay.Options)
	}
}

// TestDrawXUnresolvableWithheld pins the fail-closed direction: a cost whose
// source face does NOT define SVar:X is refused by the fold, and the trigger
// window then offers the decline only (never a silent zero draw).
func TestDrawXUnresolvableWithheld(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Grizzly Bears")
	id := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)

	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !onBattlefield(e, id) {
		t.Fatalf("Grizzly Bears is not a live battlefield source")
	}
	if _, present := o.Face().SVars["X"]; present {
		t.Fatalf("Grizzly Bears unexpectedly defines SVar:X; pick a bodyless source")
	}
	if _, ok := e.drawCostCount(id, 0, drawCostPart()); ok {
		t.Fatalf("drawCostCount accepted a Draw<X/You> part with no SVar:X body")
	}

	// A nil/unknown object is refused too (the same fail-closed contract).
	if _, ok := e.drawCostCount(state.ObjID(0), 0, drawCostPart()); ok {
		t.Fatalf("drawCostCount accepted a Draw<X/You> part for a nil source")
	}
}

// TestTitanOfLittjaraDrawXCost drives the ticket's canonical carrier end to
// end through the real trigger window: Titan's ETB trigger executes
// SVar:TrigDraw (AB$ Discard | Cost$ Draw<X/You>), so the window must pose the
// pay/decline election, paying must run the discard body, and declining must
// do neither.
func TestTitanOfLittjaraDrawXCost(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Titan of Littjara")

	// A second creature so the board is not degenerate; the chosen-type
	// machinery is independent of this test's assertions.
	searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	titan := searchMoveByName(t, e, "Titan of Littjara", state.ZBattlefield)

	// Precondition: Titan is on the battlefield and its trigger names a real
	// source with the X body this test is about.
	o := e.G.Obj(titan)
	if o == nil || o.Face() == nil || !onBattlefield(e, titan) {
		t.Fatalf("Titan of Littjara is not a live battlefield source")
	}
	if body, ok := o.Face().SVars["X"]; !ok || body == "" {
		t.Fatalf("Titan of Littjara has no SVar:X body; the Draw<X/You> cost cannot resolve")
	}

	// The ETB trigger must have pushed and posed the cost election. A
	// decline-only ask would mean the Draw part was withheld, and a priority
	// ask would mean the trigger window never opened.
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected Titan's trigger-cost pay ask, got %+v", d)
	}
	if d.Source != titan {
		t.Fatalf("the pay ask's source is %d, want Titan %d", d.Source, titan)
	}
	payIdx := -1
	declineIdx := -1
	for _, op := range d.Options {
		switch op.Kind {
		case "trigger_cost_pay":
			payIdx = op.Index
		case "trigger_cost_decline":
			declineIdx = op.Index
		}
	}
	if payIdx < 0 {
		t.Fatalf("Titan's Draw<X/You> cost was not offered as payable: %+v", d.Options)
	}
	if declineIdx < 0 {
		t.Fatalf("Titan's cost ask has no decline option: %+v", d.Options)
	}

	// Paying: the window settles the draw (its resolved count may be 0 in this
	// fixture, which is still a real payment) and then runs the Discard body,
	// which poses its TgtChoose discard ask.
	mark := len(e.L.Events)
	payDraws := 0
	submitChoices(t, e, payIdx)
	// Answer any residual pay asks (the window may re-pose for a second
	// trigger instance before the body runs) and require the discard to
	// arrive exactly once the payment has settled.
	discardSeen := false
	for i := 0; i < 8; i++ {
		next := passUntilNonPriority(t, e, 40)
		if next == nil {
			break
		}
		if next.Kind == decision.KModes && next.ResumeKind == "discard" {
			discardSeen = true
			break
		}
		if next.Kind == decision.KChoose {
			// keep paying
			idx := -1
			for _, op := range next.Options {
				if op.Kind == "trigger_cost_pay" {
					idx = op.Index
				}
			}
			if idx < 0 {
				break
			}
			submitChoices(t, e, idx)
			continue
		}
		break
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			payDraws++
		}
	}
	if !discardSeen {
		t.Fatalf("paying Titan's Draw<X/You> cost did not run the Discard body")
	}
	// The draw count must equal the fold's own verdict, whatever the fixture's
	// type state makes it -- the pay arm and the gate may not disagree.
	folded, ok := e.drawCostCount(titan, 0, drawCostPart())
	if !ok {
		t.Fatalf("Titan's SVar:X stopped resolving after the payment")
	}
	if int32(payDraws) != folded {
		t.Fatalf("paying Titan's Draw<X/You> drew %d cards, want the folded count %d", payDraws, folded)
	}

	// Declining on a fresh Titan: no draw, no discard, and the trigger window
	// still posed the election (proving the handler ran rather than the whole
	// feature being unregistered).
	reg2 := searchTestRegistry(t)
	e2, _ := searchEngine(t, reg2, "Titan of Littjara")
	searchMoveByName(t, e2, "Titan of Littjara", state.ZBattlefield)
	d2 := passUntilNonPriority(t, e2, 40)
	if d2 == nil || d2.Kind != decision.KChoose {
		t.Fatalf("expected a pay ask on the decline fixture, got %+v", d2)
	}
	declineIdx2 := -1
	for _, op := range d2.Options {
		if op.Kind == "trigger_cost_decline" {
			declineIdx2 = op.Index
		}
	}
	if declineIdx2 < 0 {
		t.Fatalf("no decline option on the decline fixture: %+v", d2.Options)
	}
	mark2 := len(e2.L.Events)
	libBefore := len(e2.G.Zone(state.ZLibrary, 0))
	gyBefore := len(e2.G.Zone(state.ZGraveyard, 0))
	submitChoices(t, e2, declineIdx2)
	// Drain any follow-up decline asks; a declined cost leaves no body.
	for i := 0; i < 4; i++ {
		next := passUntilNonPriority(t, e2, 40)
		if next == nil || next.Kind != decision.KChoose {
			break
		}
		idx := -1
		for _, op := range next.Options {
			if op.Kind == "trigger_cost_decline" {
				idx = op.Index
			}
		}
		if idx < 0 {
			break
		}
		submitChoices(t, e2, idx)
	}
	for _, ev := range e2.L.Events[mark2:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			t.Fatalf("a declined Draw<X/You> cost drew a card")
		}
		if events.IsDiscard(ev) {
			t.Fatalf("a declined Draw<X/You> cost still ran the Discard body")
		}
	}
	if got := len(e2.G.Zone(state.ZLibrary, 0)); got != libBefore {
		t.Fatalf("library = %d after a declined cost, want unchanged %d", got, libBefore)
	}
	if got := len(e2.G.Zone(state.ZGraveyard, 0)); got != gyBefore {
		t.Fatalf("graveyard = %d after a declined cost, want unchanged %d", got, gyBefore)
	}
}
