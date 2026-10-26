package rules

// CR 702.26 -- "can't phase in" / "can't phase out" (task cr-g159). The
// Effect-delivered Mode$ CantPhaseOut (Ertai's Familiar, Spatial Binding)
// and Mode$ CantPhaseIn (The Phasing of Zhalfir) bodies register a
// restriction that phaseBlocked (engine/rules/cantphase.go) reads at the emit
// choke point, so neither the untap-step phasing (finishUntapStep) nor an
// api:Phases effect (effPhases) changes the permanent's phasing. Disciple of
// Caelus Nin's printed S:Mode$ CantPhaseIn is read through the same walk.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cantPhaseEvents counts the PhaseOut events for id in the given direction
// (out: Amount > 0, in: Amount < 0).
func cantPhaseEvents(e *Engine, id state.ObjID, out bool) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.PhaseOut && ev.Obj == id && (ev.Amount > 0) == out && ev.Amount != 0 {
			n++
		}
	}
	return n
}

// cantPhaseRegistered reports whether a live continuous restriction of mode
// is registered, and fails the test on the unimplemented-mode Note.
func cantPhaseRegistered(t *testing.T, e *Engine, mode string) bool {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, mode+" unimplemented") {
			t.Fatalf("%s Effect fell to the unimplemented Note: %q", mode, ev.Text)
		}
	}
	for _, ce := range e.active() {
		if ce.Restriction == mode {
			return true
		}
	}
	return false
}

// cantPhaseTarget answers a pending target ask with the option naming id.
func cantPhaseTarget(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", id, d.Options)
}

// TestCantPhaseOutErtaisFamiliar is probe 1: "{U}: Until your next upkeep,
// Ertai's Familiar can't phase out." activated on turn 1 keeps the Phasing
// carrier phased in through seat 0's next untap step (and its "when it phases
// out, mill three" never fires); after the upkeep expiry it phases out at the
// following untap step as normal.
func TestCantPhaseOutErtaisFamiliar(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1591, "Ertai's Familiar")
	fam := moveSeededCard(t, e, 0, cr702corpusCard(t, "Ertai's Familiar"), state.ZBattlefield)
	if !e.HasKeyword(fam, "Phasing") {
		t.Fatal("precondition: Ertai's Familiar does not carry Phasing")
	}
	addMana(t, e, 0, "U")
	submitChoices(t, e, abilityOption(t, e, fam, 0).Index)
	passUntilStackEmpty(t, e, 20)
	if !cantPhaseRegistered(t, e, effects.ModeCantPhaseOut) {
		t.Fatal("precondition: no CantPhaseOut restriction registered by the activation")
	}
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	start := e.G.Turn

	driveToStepAll(t, e, start+2, 0, state.StepUpkeep)
	if o := e.G.Obj(fam); o == nil || o.Zone != state.ZBattlefield || o.PhasedOut {
		t.Fatalf("CR 702.26: Ertai's Familiar phased out at its controller's untap step despite \"can't phase out\": %+v", o)
	}
	if n := cantPhaseEvents(e, fam, true); n != 0 {
		t.Fatalf("CR 702.26: %d phase-out events for a permanent that can't phase out", n)
	}
	// The blocked phase-out fired no "phases out" mill. Seat 0's draw on
	// turn start+2 comes after this upkeep, and seat 1 draws from its own
	// library, so seat 0's library is unchanged.
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore {
		t.Fatalf("library %d -> %d: the blocked phase-out milled", libBefore, got)
	}

	// The restriction ends at this upkeep; at the following untap step the
	// Phasing keyword phases it out normally.
	driveToStepAll(t, e, start+4, 0, state.StepUpkeep)
	if o := e.G.Obj(fam); o == nil || !o.PhasedOut {
		t.Fatal("Ertai's Familiar did not phase out at the untap step after the restriction expired")
	}
	if n := cantPhaseEvents(e, fam, true); n != 1 {
		t.Fatalf("want exactly one phase-out event after expiry, got %d", n)
	}
	replayCheck(t, e, cfg)
}

// TestCantPhaseOutBlocksPhasesEffect: Spatial Binding's "target permanent
// can't phase out" on Grizzly Bears, then Talon Gates of Madara's api:Phases
// ETB targeting the Bears: no PhaseOut event, the Bears stay phased in.
func TestCantPhaseOutBlocksPhasesEffect(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1592, "Spatial Binding", "Grizzly Bears", "Talon Gates of Madara")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	binding := moveSeededCard(t, e, 0, cr702corpusCard(t, "Spatial Binding"), state.ZBattlefield)
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, binding, 0).Index)
	cantPhaseTarget(t, e, bears)
	passUntilStackEmpty(t, e, 20)
	if !cantPhaseRegistered(t, e, effects.ModeCantPhaseOut) {
		t.Fatal("precondition: no CantPhaseOut restriction registered by Spatial Binding")
	}

	moveSeededCard(t, e, 0, cr702corpusCard(t, "Talon Gates of Madara"), state.ZBattlefield)
	addMana(t, e, 0, "")
	cantPhaseTarget(t, e, bears)
	passUntilStackEmpty(t, e, 20)
	if n := cantPhaseEvents(e, bears, true); n != 0 {
		t.Fatalf("CR 702.26: api:Phases emitted %d phase-out events for a permanent that can't phase out", n)
	}
	if o := e.G.Obj(bears); o == nil || o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("Bears phased out despite Spatial Binding: %+v", o)
	}
	replayCheck(t, e, cfg)
}

// TestCantPhaseInPhasingOfZhalfir is probe 2: The Phasing of Zhalfir's
// chapter phases out a target nonland permanent and it "can't phase in for as
// long as you control" the Saga, so at its controller's next untap step it
// stays phased out.
func TestCantPhaseInPhasingOfZhalfir(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1593, "The Phasing of Zhalfir", "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	saga := moveSeededCard(t, e, 0, cr702corpusCard(t, "The Phasing of Zhalfir"), state.ZBattlefield)
	if o := e.G.Obj(saga); o == nil || o.Counter("LORE") != 1 {
		t.Fatal("precondition: the Saga did not enter with one lore counter (chapter I)")
	}
	addMana(t, e, 0, "")
	cantPhaseTarget(t, e, bears)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut {
		t.Fatal("precondition: the chapter did not phase the Bears out")
	}
	if !cantPhaseRegistered(t, e, effects.ModeCantPhaseIn) {
		t.Fatal("precondition: no CantPhaseIn restriction registered by the chapter")
	}

	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut || o.Zone != state.ZBattlefield {
		t.Fatalf("CR 702.26: the Bears phased in despite \"can't phase in\": %+v", o)
	}
	if n := cantPhaseEvents(e, bears, false); n != 0 {
		t.Fatalf("CR 702.26: %d phase-in events for a permanent that can't phase in", n)
	}
	replayCheck(t, e, cfg)
}

// TestCantPhaseInDisciplePrintedStatic covers the one printed carrier,
// Disciple of Caelus Nin's "Permanents can't phase in."
// (`S:Mode$ CantPhaseIn | ValidCard$ Card.phasedOutPermanent`): a phased-out
// permanent stays phased out at its controller's untap step.
func TestCantPhaseInDisciplePrintedStatic(t *testing.T) {
	t.Parallel()
	e, cfg := phasesGame(t, 1594, "Disciple of Caelus Nin", "Grizzly Bears")
	bears := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: bears, Amount: 1})
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut {
		t.Fatal("precondition: Bears not phased out")
	}
	disc := moveSeededCard(t, e, 0, cr702corpusCard(t, "Disciple of Caelus Nin"), state.ZBattlefield)
	// Disciple's ETB asks each player to choose permanents; keep as many as
	// offered and drain the stack, so the static is all that is under test.
	addMana(t, e, 0, "")
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
			break
		}
		if d.Kind == decision.KPriority {
			passPriorityOnce(t, e)
			continue
		}
		var keep []int
		for _, o := range d.Options {
			if d.Max == 0 || len(keep) < d.Max {
				keep = append(keep, o.Index)
			}
		}
		submitChoices(t, e, keep...)
	}
	if len(e.activeStatics(effects.ModeCantPhaseIn)) == 0 {
		t.Fatal("precondition: Disciple's printed CantPhaseIn static is not active")
	}
	if o := e.G.Obj(disc); o == nil || o.Zone != state.ZBattlefield || o.PhasedOut {
		t.Fatalf("precondition: Disciple not a phased-in battlefield permanent: %+v", o)
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	if o := e.G.Obj(bears); o == nil || !o.PhasedOut {
		t.Fatal("CR 702.26: the Bears phased in under Disciple of Caelus Nin's \"Permanents can't phase in.\"")
	}
	if n := cantPhaseEvents(e, bears, false); n != 0 {
		t.Fatalf("%d phase-in events under Disciple of Caelus Nin", n)
	}
	replayCheck(t, e, cfg)
}

// TestPhasedOutIsRememberedFilter: Card.phasedOutIsRemembered matches only a
// phased-out remembered object -- not a phased-out unremembered one, not a
// phased-in remembered one.
func TestPhasedOutIsRememberedFilter(t *testing.T) {
	t.Parallel()
	e, _ := phasesGame(t, 1595, "Grizzly Bears", "Hill Giant", "Gray Ogre")
	a := moveSeededCard(t, e, 0, cr702corpusCard(t, "Grizzly Bears"), state.ZBattlefield)
	b := moveSeededCard(t, e, 0, cr702corpusCard(t, "Hill Giant"), state.ZBattlefield)
	c := moveSeededCard(t, e, 0, cr702corpusCard(t, "Gray Ogre"), state.ZBattlefield)
	e.emit(events.Event{Kind: events.PhaseOut, Obj: a, Amount: 1})
	e.emit(events.Event{Kind: events.PhaseOut, Obj: b, Amount: 1})
	if !e.G.Obj(a).PhasedOut || !e.G.Obj(b).PhasedOut || e.G.Obj(c).PhasedOut {
		t.Fatal("precondition: want a, b phased out and c phased in")
	}
	sc := effects.NewSpecContext(0, 0)
	sc.Remembered = []state.Target{{Obj: a}, {Obj: c}}
	const spec = "Card.phasedOutIsRemembered"
	for _, tc := range []struct {
		id   state.ObjID
		want bool
	}{{a, true}, {b, false}, {c, false}} {
		if got := effects.MatchesObjectCtx(e.G, spec, e.G.Obj(tc.id), sc); got != tc.want {
			t.Errorf("%s on %d = %v, want %v", spec, tc.id, got, tc.want)
		}
	}
	if !effects.MatchesObjectCtx(e.G, "Card.IsRemembered", e.G.Obj(c), sc) {
		t.Fatal("precondition: the bare IsRemembered half does not match the phased-in remembered object")
	}
}
