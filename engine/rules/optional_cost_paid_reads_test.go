// optional_cost_paid_reads_test.go — the three resolution-time readers of a
// self-spell OptionalCost static's payment (CR 601.2b: the optional
// additional cost is announced and paid as the spell is cast; CR 601.2f/h),
// pinned on REAL corpus carriers:
//
//   - Behind the Mask's `Z:Collected$Amount` (CR 701.59a, collect evidence):
//     "If evidence was collected, it has base power and toughness 1/1 ...
//     instead" -- the count ref must enumerate the cast's own evidence;
//   - Vitu-Ghazi Inspector's intervening-if ETB gate
//     `CheckSVar$ Z` with `Z:CastSA>Collected$Amount` (CR 603.4): the trigger
//     fires only when the cast collected evidence;
//   - Ruinous Waterbending's `Condition$ OptionalCost |
//     ConditionOptionalPaid$ True` rider: "If this spell's additional cost was
//     paid, whenever a creature dies this turn, you gain 1 life".
//
// Were the Collected ref unknown to the count evaluator, both counts would
// read zero and the paid cast would behave as unpaid; and the bare
// OptionalCost condition was classified unmodelled (the rider failed closed).
// None of the three cards is in a legacy golden deck, so no chain head moves.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// drainAnswering passes priority until the stack is empty, answering every
// target ask with the option naming want (or the first option when want is
// zero or not offered) and failing on any other ask.
func drainAnswering(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	for i := 0; i < 60 && !e.G.Over && (len(e.G.Stack) > 0 || e.Pending() == nil || e.Pending().Kind != decision.KPriority); i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while draining the stack")
		}
		switch d.Kind {
		case decision.KPriority:
			passOnceP(t, e)
		case decision.KTarget:
			pick := d.Options[0].Index
			for _, o := range d.Options {
				if want != 0 && o.Obj == want {
					pick = o.Index
				}
			}
			submitChoices(t, e, pick)
		default:
			t.Fatalf("unexpected ask while draining: %+v", d)
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not drain (depth %d)", len(e.G.Stack))
	}
}

// behindTheMaskCast casts Behind the Mask at the foe's Bear, collecting
// Colossal Dreadmaw (mana value 6) as evidence when paid, and resolves it.
func behindTheMaskCast(t *testing.T, seed uint64, mode string) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, []string{"Behind the Mask", "Colossal Dreadmaw"}, nil, []string{altBearSrc})
	spell := findCardObj(t, e, 0, "Behind the Mask", state.ZHand)
	colossal := evidenceToGraveyard(t, e)
	foe := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
	addMana(t, e, 0, "U")
	submitChoices(t, e, castModeOption(t, e, spell, mode))
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		switch d.Kind {
		case decision.KTarget:
			drainTarget(t, e, d, foe)
		case decision.KChoose:
			choosePendingObject(t, e, colossal)
		default:
			t.Fatalf("unexpected cast ask %+v", d)
		}
	}
	drainAnswering(t, e, foe)
	return e, cfg, foe, colossal
}

// evidenceToGraveyard puts seat 0's Colossal Dreadmaw (mana value 6, exactly
// evidence 6) into its graveyard from wherever the deal left it, through the
// event the replay folds.
func evidenceToGraveyard(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face().Name == "Colossal Dreadmaw" {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
				return id
			}
		}
	}
	t.Fatal("Colossal Dreadmaw not dealt to seat 0")
	return 0
}

// drainTarget answers the target ask d with the option naming want.
func drainTarget(t *testing.T, e *Engine, d *decision.Decision, want state.ObjID) {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target ask did not offer %d: %+v", want, d.Options)
}

// TestBehindTheMaskCollectedEvidenceIsOneOne: CR 701.59a + the card's
// "instead" clause -- with evidence collected the target's base P/T is 1/1.
func TestBehindTheMaskCollectedEvidenceIsOneOne(t *testing.T) {
	t.Parallel()
	e, cfg, foe, colossal := behindTheMaskCast(t, 731, "optionalcost")
	if o := e.G.Obj(colossal); o == nil || o.Zone != state.ZExile {
		t.Fatal("precondition: the evidence card was not exiled")
	}
	assertDerivedPT(t, e, foe, 1, 1)
	replayCheck(t, e, cfg)
}

// TestBehindTheMaskNoEvidenceIsFourThree pins the unpaid branch: 4/3.
func TestBehindTheMaskNoEvidenceIsFourThree(t *testing.T) {
	t.Parallel()
	e, cfg, foe, _ := behindTheMaskCast(t, 732, "")
	assertDerivedPT(t, e, foe, 4, 3)
	replayCheck(t, e, cfg)
}

// vituGhaziCast casts Vitu-Ghazi Inspector (mode "optionalcost" collects
// Colossal Dreadmaw) and resolves it and its ETB trigger, targeting itself.
func vituGhaziCast(t *testing.T, seed uint64, mode string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, _ := altCostEngine(t, seed, []string{"Vitu-Ghazi Inspector", "Colossal Dreadmaw"}, nil, []string{altBearSrc})
	spell := findCardObj(t, e, 0, "Vitu-Ghazi Inspector", state.ZHand)
	colossal := evidenceToGraveyard(t, e)
	addMana(t, e, 0, "CG")
	submitChoices(t, e, castModeOption(t, e, spell, mode))
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		choosePendingObject(t, e, colossal)
	}
	drainAnswering(t, e, spell)
	return e, cfg, spell
}

// TestVituGhaziInspectorEvidenceGateFires: CR 603.4 -- the intervening-if
// "if evidence was collected" holds for the paid cast, so the trigger puts a
// +1/+1 counter and gains 2 life.
func TestVituGhaziInspectorEvidenceGateFires(t *testing.T) {
	t.Parallel()
	e, cfg, inspector := vituGhaziCast(t, 733, "optionalcost")
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("life = %d, want 22 (the evidence-gated gain)", got)
	}
	if got := e.G.Obj(inspector).Counter("P1P1"); got != 1 {
		t.Fatalf("+1/+1 counters = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestVituGhaziInspectorNoEvidenceNoTrigger pins the unpaid cast: the gate
// fails, no counter, no life.
func TestVituGhaziInspectorNoEvidenceNoTrigger(t *testing.T) {
	t.Parallel()
	e, cfg, inspector := vituGhaziCast(t, 734, "")
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("life = %d, want 20", got)
	}
	if got := e.G.Obj(inspector).Counter("P1P1"); got != 0 {
		t.Fatalf("+1/+1 counters = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestRuinousWaterbendingPaidRiderGainsOnDeath: the bare `Condition$
// OptionalCost | ConditionOptionalPaid$ True` rider runs when the waterbend
// was paid (CR 601.2b), so the foe's Bear dying to -2/-2 gains 1 life; the
// plain cast gains nothing.
func TestRuinousWaterbendingPaidRiderGainsOnDeath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode string
		pool string
		life int32
	}{{"optionalcost", "CBBCCCC", 21}, {"", "CBB", 20}} {
		e, cfg, _ := altCostEngine(t, 735, []string{"Ruinous Waterbending"}, nil, []string{altBearSrc})
		spell := findCardObj(t, e, 0, "Ruinous Waterbending", state.ZHand)
		foe := findCardObj(t, e, 1, "Bear", state.ZBattlefield)
		addMana(t, e, 0, tc.pool)
		submitChoices(t, e, castModeOption(t, e, spell, tc.mode))
		drainAnswering(t, e, 0)
		e.checkStateBased()
		if o := e.G.Obj(foe); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("mode %q: precondition: the Bear did not die", tc.mode)
		}
		drainAnswering(t, e, 0)
		if got := e.G.Players[0].Life; got != tc.life {
			t.Fatalf("mode %q: life = %d, want %d", tc.mode, got, tc.life)
		}
		replayCheck(t, e, cfg)
	}
}
