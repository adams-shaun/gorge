// Count$YourCounters<KIND> — the player-counter count heads (ticket
// count-yourcounters). Before the fix every YourCounters head degraded to 0
// through Num's "present but unresolvable degrades to zero" rule.
//
// The end-to-end pin runs on the REAL corpus card Razorfield Ripper (whose
// trigger body is `DB$ Pump | Defined$ TriggeredAttackerLKICopy | NumAtt$ +X
// | NumDef$ +X` with `SVar:X:Count$YourCountersEnergy`); the compiled-SVar
// pins run Localized Destruction's and Aether Refinery's
// `DB$ ChooseNumber | Max$ Count$YourCountersEnergy` Max expressions and the
// YourCountersExperience / YourCountersRAD spellings (Otharri, Mariposa
// Military Base) through the real cards' own SVar tables.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// seedPlayerCounter grants p n counters of kind through the event the engine
// folds, so the read under test is event-backed exactly as in play.
func seedPlayerCounter(t *testing.T, e *Engine, p state.PlayerID, kind string, n int32) {
	t.Helper()
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: p, Counter: kind, Amount: n})
}

// yourCountersCard looks a corpus card up by name.
func yourCountersCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus missing %s", name)
	}
	return c
}

// resolveSVarOf resolves one named SVar body from a face; a nil result means
// the brief's premise about that SVar is stale. DB$/T: bodies resolve here;
// bare Count$ bodies go through svarBodyOf instead.
func resolveSVarOf(t *testing.T, f *cards.Face, name string) *cards.SA {
	t.Helper()
	sa := cards.ResolveSVar(f.SVars, name)
	if sa == nil {
		t.Fatalf("%s: SVar %s unresolved", f.Name, name)
	}
	return sa
}

// svarBodyOf reads one named SVar's raw body off a face; a bare `Count$…`
// body (unlike a `DB$ …` SA body) does not resolve through cards.ResolveSVar,
// so the expression-level pins read the body and hand it to EvalCountOK.
func svarBodyOf(t *testing.T, f *cards.Face, name string) string {
	t.Helper()
	body, ok := f.SVars[name]
	if !ok || body == "" {
		t.Fatalf("%s: SVar %s missing", f.Name, name)
	}
	return body
}

// TestRazorfieldRipperAttackPumpEqualsEnergyTotal is the end-to-end pin: with
// 2 {E} seeded, the attack trigger grants 1 more (3 total) and pumps the
// attacking Ripper +3/+3, so it hits for 6. Without the head the pump degrades
// to +0/+0 and the power stays 3.
func TestRazorfieldRipperAttackPumpEqualsEnergyTotal(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	rip := onBoardCard(t, e, 0, yourCountersCard(t, "Razorfield Ripper"))

	// Precondition: the trigger's pump is the YourCounters-driven body and the
	// seeded energy differs from the answer the buggy read would produce.
	trig := resolveSVarOf(t, e.G.Obj(rip).Face(), "TrigPutCounter")
	if trig.Sub == nil || trig.Sub.API != "Pump" {
		t.Fatalf("test precondition: TrigPutCounter's sub is %+v, want the DB$ Pump chain", trig.Sub)
	}
	pump := resolveSVarOf(t, e.G.Obj(rip).Face(), "DBPump")
	if pump.Params["NumAtt"] != "+X" {
		t.Fatalf("test precondition: DBPump NumAtt = %q, want +X", pump.Params["NumAtt"])
	}
	seedPlayerCounter(t, e, 0, "ENERGY", 2)
	if got := e.G.Players[0].Counter("ENERGY"); got != 2 {
		t.Fatalf("test precondition: seeded energy = %d, want 2", got)
	}

	resolveAttackPump(t, e, rip)

	// The trigger's own PutCounter (Defined$ You, ENERGY, 1) landed first.
	if got := e.G.Players[0].Counter("ENERGY"); got != 3 {
		t.Fatalf("energy after the attack trigger = %d, want 3 (2 seeded + the trigger's 1)", got)
	}
	// X = 3 at pump time: base 3/3 + 3/+3.
	if got := e.Power(rip); got != 6 {
		t.Fatalf("Ripper power = %d, want 6 (+X with X = the 3 {E} it holds)", got)
	}
	if got := e.Toughness(rip); got != 6 {
		t.Fatalf("Ripper toughness = %d, want 6 (NumDef$ +X too)", got)
	}
}

// TestYourCountersExperienceAndRADSpellings pins the other two spellings
// through the real cards' SVars: Otharri's SVar:X (YourCountersExperience,
// the kind its own script writes as "Experience") and Mariposa Military
// Base's SVar:X (YourCountersRAD, ReduceCost$ X on its draw ability). The
// mixed-case corpus write must match the stored kind, and the two sub-counts
// of a differently-cased same kind must sum rather than hide.
func TestYourCountersExperienceAndRADSpellings(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	otharri := onBoardCard(t, e, 0, yourCountersCard(t, "Otharri, Suns' Glory"))
	base := onBoardCard(t, e, 0, yourCountersCard(t, "Mariposa Military Base"))

	// Precondition: the SVars under test are the YourCounters heads.
	if body := svarBodyOf(t, e.G.Obj(otharri).Face(), "X"); body != "Count$YourCountersExperience" {
		t.Fatalf("test precondition: Otharri SVar:X = %q", body)
	}
	if body := svarBodyOf(t, e.G.Obj(base).Face(), "X"); body != "Count$YourCountersRAD" {
		t.Fatalf("test precondition: Mariposa SVar:X = %q", body)
	}
	seedPlayerCounter(t, e, 0, "Experience", 2)
	seedPlayerCounter(t, e, 0, "RAD", 3)
	if got := e.G.Players[0].Counter("Experience"); got != 2 || e.G.Players[0].Counter("RAD") != 3 {
		t.Fatalf("test precondition: experience/rad = %d/%d, want 2/3",
			e.G.Players[0].Counter("Experience"), e.G.Players[0].Counter("RAD"))
	}

	ctx := &effects.Ctx{Controller: 0, Source: otharri}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$YourCountersExperience"); !ok || n != 2 {
		t.Fatalf("YourCountersExperience = %d (ok %v), want 2", n, ok)
	}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$YourCountersRAD"); !ok || n != 3 {
		t.Fatalf("YourCountersRAD = %d (ok %v), want 3", n, ok)
	}
	// Case-insensitive: the corpus writes experience counters both as
	// "Experience" (Otharri) and upper-case elsewhere; differently-cased
	// entries of the same kind must sum.
	seedPlayerCounter(t, e, 0, "EXPERIENCE", 4)
	if n, ok := effects.EvalCountOK(e, ctx, "Count$YourCountersExperience"); !ok || n != 6 {
		t.Fatalf("YourCountersExperience after the upper-case grant = %d (ok %v), want 6", n, ok)
	}
	// Another player's counters never count.
	seedPlayerCounter(t, e, 1, "EXPERIENCE", 5)
	if n, _ := effects.EvalCountOK(e, ctx, "Count$YourCountersExperience"); n != 6 {
		t.Fatalf("YourCountersExperience read seat 1's counters: %d, want 6", n)
	}
}

// TestEnergyCountersBoundTheMayPayAsks pins the Creative Energy deck's two
// ChooseNumber carriers: Localized Destruction's and Aether Refinery's
// `DB$ ChooseNumber | Max$ Count$YourCountersEnergy` Max expressions resolve
// to the controller's energy total through each card's own compiled SVar —
// the bound the may-pay-{E} ask takes once the ChooseNumber ask primitive
// itself lands (its own ticket; today the ask is the silent 0 fallback).
func TestEnergyCountersBoundTheMayPayAsks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
	}{
		{"Localized Destruction"},
		{"Aether Refinery"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One engine per card: Aether Refinery's own R:Event$ AddCounter /
			// ReplaceWith$ Twice replacement doubles every energy grant on its
			// board, so a shared engine would couple the two cards' totals.
			e := layerEngine(t)
			id := onBoardCard(t, e, 0, yourCountersCard(t, tc.name))
			face := e.G.Obj(id).Face()
			sa := resolveSVarOf(t, face, "DBChooseNumber")
			if sa.Params["Max"] != "Count$YourCountersEnergy" {
				t.Fatalf("test precondition: %s DBChooseNumber Max = %q", tc.name, sa.Params["Max"])
			}
			ctx := &effects.Ctx{Controller: 0, Source: id}
			// Before any energy, the Max bound is a real 0.
			if n, ok := effects.NumResolved(e, ctx, sa, "Max", -1); !ok || n != 0 {
				t.Fatalf("%s: Max with no energy = %d (ok %v), want resolvable 0", tc.name, n, ok)
			}
			seedPlayerCounter(t, e, 0, "ENERGY", 7)
			// The bound reads the asking controller's ACTUAL total — which for
			// the Refinery board is 14, not the seeded 7: the Refinery's own
			// doubling replacement fired on the grant (real engine behaviour,
			// and a bonus pin for it). Assert Max against that total.
			want := e.G.Players[0].Counter("ENERGY")
			if want != 7 && want != 14 {
				t.Fatalf("test precondition: %s board energy = %d, want 7 (LD) or 14 (Refinery doubled)", tc.name, want)
			}
			if n, ok := effects.NumResolved(e, ctx, sa, "Max", -1); !ok || n != want {
				t.Fatalf("%s: Max = %d (ok %v), want %d (the controller's {E} total)", tc.name, n, ok, want)
			}
			// The bound is the ASKING controller's own total, not seat 0's fixed.
			ctx2 := &effects.Ctx{Controller: 1, Source: id}
			if n, ok := effects.NumResolved(e, ctx2, sa, "Max", -1); !ok || n != 0 {
				t.Fatalf("%s: Max for the other player = %d (ok %v), want 0", tc.name, n, ok)
			}
		})
	}
}
