package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The bounded mid-resolution ChooseNumber ask (task bounded1, absorbing
// agent-20260922T091245Z-d777f1b6): a card's own Max$ bound, resolved against
// the current resolution context, and its ListTitle$ prompt now shape the
// number ask -- no more hard-coded 0..12 offer under a literal "Choose a
// number". The primitive-level pins live effects-side
// (effects/choose_number_bounds_test.go); these are the REAL-corpus end-to-end
// pins on the carriers the brief names: Pia Nalaar, Chief Mechanic (direct
// Max$ Count$YourCountersEnergy, driving the full trigger -> ask -> pay ->
// X/X token chain), Localized Destruction and Aether Refinery (the Creative
// Energy may-pay carriers, absorbing the d777f1b6 ticket: one KChoose bounded
// by the current energy with each card's exact ListTitle$ prompt, the
// answered value folded into one Choose{number} event), and Rampaging
// Aetherhood (the SVar-indirect Max$ Max spelling).

// numberEventsIn counts the Choose events a log carries whose Counter is
// "number" -- the one event the answered ask records the pick with.
func numberEventsIn(e *Engine) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "number" {
			out = append(out, ev)
		}
	}
	return out
}

// TestPiaNalaarBoundedEnergyChoiceMakesTheToken is the flagship end-to-end
// pin: Pia Nalaar, Chief Mechanic's end-step trigger poses its ChooseNumber
// ask bounded by the controller's ACTUAL energy (Max$
// Count$YourCountersEnergy) under the card's ListTitle$ prompt; answering 2
// with 3 {E} on the table pays 2 {E} through the token body's UnlessCost$
// Mandatory PayEnergy<X> and creates the 2/2 Nalaar Aetherjet. The answered
// number (2) is within the OLD fixed 0..12 list too, so the token half alone
// cannot prove the fix -- the option-shape assertions (exactly 0..3, the
// ListTitle prompt) are what the old list fails.
func TestPiaNalaarBoundedEnergyChoiceMakesTheToken(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Pia Nalaar, Chief Mechanic")
	pia := ateotFind(t, e, "Pia Nalaar, Chief Mechanic", 0)
	ateotTo(t, e, pia, state.ZLibrary, state.ZBattlefield)
	if o := e.G.Obj(pia); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Pia Nalaar is not on the battlefield: %+v", o)
	}
	// The real face carries exactly the parameters under test.
	face := e.G.Obj(pia).Face()
	askSA := resolveSVarOf(t, face, "TrigChooseNumber")
	if askSA.Params["Max"] != "Count$YourCountersEnergy" {
		t.Fatalf("precondition: TrigChooseNumber Max$ = %q", askSA.Params["Max"])
	}
	if askSA.Params["ListTitle"] != "amount of energy to pay" {
		t.Fatalf("precondition: TrigChooseNumber ListTitle$ = %q", askSA.Params["ListTitle"])
	}
	seedPlayerCounter(t, e, 0, "ENERGY", 3)
	ateotDriveToStep(t, e, 1, 0, state.StepEnd)

	// Pass priority until the trigger's number ask is pending.
	d := passUntilAskKind(t, e, decision.KChoose, 400)
	if d == nil || d.ResumeKind != "choosenumber" {
		t.Fatalf("expected the bounded number ask, got %+v", d)
	}
	if d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("ask meta = player %d min %d max %d, want seat 0 and a one-pick ask", d.Player, d.Min, d.Max)
	}
	want := e.G.Players[0].Counter("ENERGY")
	if want != 3 {
		t.Fatalf("precondition: seat 0's energy at ask time = %d, want the seeded 3", want)
	}
	if got := d.Prompt; got != "amount of energy to pay" {
		t.Fatalf("prompt = %q, want the card's ListTitle$ verbatim", got)
	}
	if len(d.Options) != int(want)+1 {
		t.Fatalf("options = %+v (%d), want exactly 0..%d -- the card's own bound, not the fixed 0..12", d.Options, len(d.Options), want)
	}
	for i, o := range d.Options {
		if o.Kind != "number" || o.Amount != i {
			t.Fatalf("option %d = %+v, want the ascending number %d", i, o, i)
		}
	}
	two := -1
	for _, o := range d.Options {
		if o.Kind == "number" && o.Amount == 2 {
			two = o.Index
		}
	}
	if two < 0 {
		t.Fatalf("precondition: the bounded list offers no 2: %+v", d.Options)
	}
	submitChoices(t, e, two)

	// The chain continues to DBToken: its UnlessCost$ Mandatory PayEnergy<X>
	// poses the unless-pay election ("Pay ..." is option 0); answer pay.
	for i := 0; i < 100; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending while waiting for the unless-pay ask")
		}
		if d.ResumeKind == "unless_pay" {
			if e.G.Players[0].Counter("ENERGY") < 2 {
				t.Fatalf("precondition: the payer holds %d {E}, cannot pay 2", e.G.Players[0].Counter("ENERGY"))
			}
			submitChoices(t, e, 0)
			break
		}
		if d.Kind == decision.KPriority {
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
			continue
		}
		t.Fatalf("unexpected decision while waiting for the unless-pay ask: %+v", d)
	}

	// Drain the rest of the stack, then assert the outcome.
	passUntilStackEmpty(t, e, 100)
	var jet state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Nalaar Aetherjet" {
			jet = id
		}
	}
	if jet == 0 {
		t.Fatalf("answered 2 of 3 {E}: no Nalaar Aetherjet token was created (battlefield %+v)", e.G.Zone(state.ZBattlefield, 0))
	}
	if pt := e.Derived(jet); pt.Power != 2 || pt.Toughness != 2 {
		t.Fatalf("Nalaar Aetherjet P/T = %d/%d, want the answered 2/2", pt.Power, pt.Toughness)
	}
	if got := e.G.Players[0].Counter("ENERGY"); got != 1 {
		t.Fatalf("energy after the pay = %d, want 3-2=1", got)
	}
	evs := numberEventsIn(e)
	if len(evs) != 1 || evs[0].Amount != 2 {
		t.Fatalf("Choose{number} events = %+v, want exactly one recording the answered 2", evs)
	}
	replayCheck(t, e, cfg)
}

// TestEnergyMayPayCarriersPoseTheBoundedAsk absorbs agent-20260922T091245Z-d777f1b6
// for Localized Destruction and Aether Refinery: each card's
// `DB$ ChooseNumber | Max$ Count$YourCountersEnergy` poses a REAL ask bounded
// by the asking controller's current energy (3 {E} offer exactly 0..3, never
// the fixed 0..12), under the card's exact ListTitle$, and the answered value
// is recorded by ONE Choose{number} event and read back by the card's own
// Count$ChosenNumber X (the SVar every downstream UnlessCost$ PayEnergy<X>
// folds). The unbound Count$ChosenNumber verdict (0 before any answer) is
// asserted before the resolve.
func TestEnergyMayPayCarriersPoseTheBoundedAsk(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		prompt string
		seed   int32 // energy granted before the ask; the Refinery's doubling replacement fires on the grant
	}{
		{name: "Localized Destruction", prompt: "Choose amount of energy to pay", seed: 3},
		{name: "Aether Refinery", prompt: "amount of energy to pay", seed: 5}, // its doubling replacement makes this 10
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One engine per card: the Refinery's own R:Event$ AddCounter
			// doubling replacement would couple the two boards' totals.
			e := layerEngine(t)
			id := onBoardCard(t, e, 0, corpusCard(t, tc.name))
			face := e.G.Obj(id).Face()
			sa := resolveSVarOf(t, face, "DBChooseNumber")
			if sa.Params["Max"] != "Count$YourCountersEnergy" {
				t.Fatalf("precondition: %s DBChooseNumber Max$ = %q", tc.name, sa.Params["Max"])
			}
			if sa.Params["ListTitle"] != tc.prompt {
				t.Fatalf("precondition: %s DBChooseNumber ListTitle$ = %q, want %q", tc.name, sa.Params["ListTitle"], tc.prompt)
			}
			// The unbound verdict: before any answer the card's own X reads
			// the source's zero.
			if n, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0, Source: id}, "Count$ChosenNumber"); !ok || n != 0 {
				t.Fatalf("%s: unbound Count$ChosenNumber = %d (ok %v), want 0", tc.name, n, ok)
			}
			seedPlayerCounter(t, e, 0, "ENERGY", tc.seed)
			want := e.G.Players[0].Counter("ENERGY")
			if want != tc.seed && want != 2*tc.seed {
				t.Fatalf("precondition: %s board energy = %d, want %d (plain) or %d (Refinery doubled)", tc.name, want, tc.seed, 2*tc.seed)
			}
			if want <= 0 || want > 12 {
				t.Fatalf("precondition: bound %d cannot distinguish the card's own list from the fixed 0..12", want)
			}
			ctx := &effects.Ctx{Controller: 0, Source: id, SVars: face.SVars}
			effects.Resolve(e, ctx, sa)
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosenumber" {
				t.Fatalf("%s: expected the bounded number ask, got %+v", tc.name, d)
			}
			if d.Player != 0 || d.Min != 1 || d.Max != 1 {
				t.Fatalf("%s: ask meta = player %d min %d max %d", tc.name, d.Player, d.Min, d.Max)
			}
			if got := d.Prompt; got != tc.prompt {
				t.Fatalf("%s: prompt = %q, want the card's ListTitle$ verbatim", tc.name, got)
			}
			if len(d.Options) != int(want)+1 {
				t.Fatalf("%s: options = %+v (%d), want exactly 0..%d -- the controller's own energy, not the fixed 0..12", tc.name, d.Options, len(d.Options), want)
			}
			for i, o := range d.Options {
				if o.Kind != "number" || o.Amount != i {
					t.Fatalf("%s: option %d = %+v, want the ascending number %d", tc.name, i, o, i)
				}
			}
			// Answer the TOP value (want, provably not the fallback 0) and
			// assert the one Choose event and the recorded answer.
			top := -1
			for _, o := range d.Options {
				if o.Kind == "number" && o.Amount == int(want) {
					top = o.Index
				}
			}
			if top < 0 {
				t.Fatalf("%s: the bound's top value %d is not offered: %+v", tc.name, want, d.Options)
			}
			submitChoices(t, e, top)
			evs := numberEventsIn(e)
			if len(evs) != 1 || evs[0].Amount != want {
				t.Fatalf("%s: Choose{number} events = %+v, want exactly one recording the answered %d", tc.name, evs, want)
			}
			if o := e.G.Obj(id); o.ChosenNumber != want {
				t.Fatalf("%s: recorded answer = %d, want the answered %d", tc.name, o.ChosenNumber, want)
			}
			if n, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0, Source: id, SVars: face.SVars}, "Count$ChosenNumber"); !ok || n != want {
				t.Fatalf("%s: Count$ChosenNumber after the answer = %d (ok %v), want %d", tc.name, n, ok, want)
			}
			// The chained body (DBPumpAll / DBToken) consumed the answer: its
			// UnlessCost$ PayEnergy<X> folds X to the answered value and asks
			// the payer to cover it.
			d2 := e.Pending()
			if d2 == nil || d2.ResumeKind != "unless_pay" {
				t.Fatalf("%s: the chained body did not pose the unless-pay ask, got %+v", tc.name, d2)
			}
			if d2.Player != 0 {
				t.Fatalf("%s: unless-pay ask posed to player %d, want the UnlessPayer$ You seat", tc.name, d2.Player)
			}
		})
	}
}

// TestRampagingAetherhoodSVarIndirectBound pins the SVar-mediated spelling:
// Rampaging Aetherhood's `DB$ ChooseNumber | Max$ Max` resolves the named
// SVar (SVar:Max:Count$YourCountersEnergy, the card's own table) against the
// controller's energy, so the offered list is exactly 0..energy under the
// card's ListTitle$ prompt -- and the answered value is recorded by one
// Choose{number} event.
func TestRampagingAetherhoodSVarIndirectBound(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id := onBoardCard(t, e, 0, corpusCard(t, "Rampaging Aetherhood"))
	face := e.G.Obj(id).Face()
	sa := resolveSVarOf(t, face, "DBChooseNumber")
	if sa.Params["Max"] != "Max" {
		t.Fatalf("precondition: DBChooseNumber Max$ = %q, want the SVar name Max", sa.Params["Max"])
	}
	if body := svarBodyOf(t, face, "Max"); body != "Count$YourCountersEnergy" {
		t.Fatalf("precondition: SVar Max = %q", body)
	}
	if sa.Params["ListTitle"] != "amount of energy to pay" {
		t.Fatalf("precondition: DBChooseNumber ListTitle$ = %q", sa.Params["ListTitle"])
	}
	seedPlayerCounter(t, e, 0, "ENERGY", 4)
	want := e.G.Players[0].Counter("ENERGY")
	if want != 4 {
		t.Fatalf("precondition: board energy = %d, want the seeded 4", want)
	}
	effects.Resolve(e, &effects.Ctx{Controller: 0, Source: id, SVars: face.SVars}, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosenumber" {
		t.Fatalf("expected the bounded number ask, got %+v", d)
	}
	if got := d.Prompt; got != "amount of energy to pay" {
		t.Fatalf("prompt = %q, want the card's ListTitle$ verbatim", got)
	}
	if len(d.Options) != int(want)+1 {
		t.Fatalf("options = %+v (%d), want exactly 0..%d -- the SVar-bound list, not the fixed 0..12", d.Options, len(d.Options), want)
	}
	for i, o := range d.Options {
		if o.Kind != "number" || o.Amount != i {
			t.Fatalf("option %d = %+v, want the ascending number %d", i, o, i)
		}
	}
	// Answer 1 and prove the SVar-indirect answer is recorded like any other.
	one := -1
	for _, o := range d.Options {
		if o.Kind == "number" && o.Amount == 1 {
			one = o.Index
		}
	}
	if one < 0 {
		t.Fatalf("precondition: the list offers no 1: %+v", d.Options)
	}
	submitChoices(t, e, one)
	evs := numberEventsIn(e)
	if len(evs) != 1 || evs[0].Amount != 1 {
		t.Fatalf("Choose{number} events = %+v, want exactly one recording the answered 1", evs)
	}
	if o := e.G.Obj(id); o.ChosenNumber != 1 {
		t.Fatalf("recorded answer = %d, want the answered 1", o.ChosenNumber)
	}
}
