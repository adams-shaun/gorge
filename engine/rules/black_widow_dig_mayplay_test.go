package rules

// black_widow_dig_mayplay_test.go pins Black Widow, Super Spy's dig-and-play
// grant (CR 400.14a, CR 611.2): whenever Black Widow deals combat damage to a
// player, that player exiles cards from the top of their library until they
// exile a nonland card; if Black Widow's controller declines the optional
// +1/+1 counter, they may cast the exiled nonland card until end of turn and
// mana of any type can be spent to cast that spell. The two tests pin the
// offer read the std-msh pair-341 divergence priced against (gorge posed no
// cast of the exiled Futurist Forge the kernel posed): the floating pool
// alone casts the granted card, so an empty pool offers nothing and a pool
// that pays the rider-converted cost offers the cast.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func widowGame(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, ids := mpRiderGame(t,
		[]string{"Futurist Forge"}, []string{"We Say Thee Nay!", "Cerulean Drake"}, nil,
		[]string{"Black Widow, Super Spy", "Island", "Training Compound"})
	widow := ids[1]["Black Widow, Super Spy"][0]
	forge := ids[0]["Futurist Forge"][0]
	gg := ids[1]["We Say Thee Nay!"][0]
	return e, cfg, widow, forge, gg
}

// forgeToLibraryTop moves forge from seat 0's hand to the top of its library,
// logged like the harness's other moves.
func forgeToLibraryTop(t *testing.T, e *Engine, forge state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.MoveZone, Obj: forge, From: state.ZHand, To: state.ZLibrary, Player: 0})
	lib := slices.DeleteFunc(slices.Clone(e.G.Zone(state.ZLibrary, 0)), func(id state.ObjID) bool { return id == forge })
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: slices.Concat([]state.ObjID{forge}, lib)})
	if e.G.Obj(forge).Zone != state.ZLibrary || e.G.Zone(state.ZLibrary, 0)[0] != forge {
		t.Fatalf("precondition: Forge not on seat 0's library top (zone %s)", e.G.Obj(forge).Zone)
	}
}

// TestBlackWidowDigGrantOffer walks the real dig: the trigger exiles seat 0's
// Forge (a nonland card, so the dig stops), the counter ask is declined, and
// seat 1 holds the cast permission. At priority in seat 1's own main 2 the
// exiled Forge is NOT offered while the floating pool is empty -- gorge's
// may-play offer gate (mayPlayAffordable: castable over the floating pool,
// the rider inside its feasibility read) prices no source but the pool -- and
// the pool that pays the rider-converted {1}{U} offers it.
func TestBlackWidowDigGrantOffer(t *testing.T) {
	e, cfg, widow, forge, _ := widowGame(t)
	mpRiderDrive(t, e, 4, 1, state.StepMain1)
	// Seat 0 has drawn past its other cards by now: seat the Forge on top of
	// its library so the dig exiles it at the first step.
	forgeToLibraryTop(t, e, forge)

	// Attack with Black Widow; seat 0 blocks with nothing, so the combat
	// damage reaches the player and fires the dig trigger.
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("precondition: no attackers decision (%+v)", d)
	}
	var atk *decision.Option
	for i := range d.Options {
		if d.Options[i].Obj == widow && d.Options[i].Battle == 0 {
			atk = &d.Options[i]
			break
		}
	}
	if atk == nil {
		t.Fatalf("precondition: Black Widow not offered an attack: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{atk.Index}}); err != nil {
		t.Fatalf("submit attack: %v", err)
	}
	e.dealCombatDamage()

	// Settle the dig: it exiles seat 0's Forge (nonland: the dig stops), then
	// the optional +1/+1 counter ask goes to seat 1. Decline it.
	digged := false
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			for _, ev := range e.L.Events[max(0, len(e.L.Events)-25):] {
				t.Logf("ev %+v", ev)
			}
			t.Fatalf("no decision while settling the dig (iter %d): forge zone %s stack %d over %v", i, e.G.Obj(forge).Zone, len(e.G.Stack), e.G.Over)
		}
		if d.Kind == decision.KPriority {
			if e.G.Obj(forge).Zone == state.ZExile {
				digged = true
				break
			}
			castFirst(t, e, "pass")
			continue
		}
		var no *decision.Option
		for j := range d.Options {
			if d.Options[j].Label == "No" || d.Options[j].Label == "no" {
				no = &d.Options[j]
				break
			}
		}
		if no == nil {
			if e.G.Obj(forge).Zone == state.ZExile {
				digged = true
				break
			}
			// A discard ask (the harness's ordinary cleanup answer): first
			// option, then keep settling.
			if len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Label, "Discard") {
				if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
					t.Fatalf("discard: %v", err)
				}
				continue
			}
			t.Fatalf("unhandled decision %s options %v (want a decline)", d.Kind, mpRiderLabels(e))
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{no.Index}}); err != nil {
			t.Fatalf("decline: %v", err)
		}
	}
	if !digged {
		t.Fatalf("the dig did not exile Forge (zone %s)", e.G.Obj(forge).Zone)
	}
	if got := e.G.Obj(widow).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: counter was put (%d), the grant must not exist", got)
	}
	if c := e.G.Obj(forge); c.Zone != state.ZExile {
		t.Fatalf("Forge left exile: %s", c.Zone)
	}

	// Seat 1 at priority in its own main 2, the floating pool empty: the
	// granted cast is priced against the pool alone, so only the ordinary
	// actions are offered -- no cast of the exiled Forge.
	mpRiderDrive(t, e, 4, 1, state.StepMain2)
	if ianOffers(e, forge) {
		t.Fatalf("empty pool offered the exiled Forge: %v", mpRiderLabels(e))
	}
	// The pool that pays the rider-converted {1}{U} (Island's {U} plus
	// Training Compound's {C}) offers the cast: the pool alone casts it.
	addManaQuiet(t, e, 1, "UC")
	e.priorityRound()
	if !ianOffers(e, forge) {
		t.Fatalf("floated {U}{C} still withheld the exiled Forge: %v", mpRiderLabels(e))
	}
	replayCheck(t, e, cfg)
}

// TestBlackWidowDigGrantHandGrant checks the same read on the grant as
// DBEffect would register it, added by hand over the same exiled Forge: the
// empty pool offers nothing (and the plain hand casts beside it neither),
// and the pool that pays the rider-converted cost offers the cast.
func TestBlackWidowDigGrantHandGrant(t *testing.T) {
	e, cfg, widow, forge, gg := widowGame(t)
	var ls state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 1) {
		if e.G.Obj(id).Face().Name == "Cerulean Drake" {
			ls = id
		}
	}
	if ls == 0 {
		t.Fatalf("precondition: Cerulean Drake not in seat 1's hand")
	}
	mpRiderDrive(t, e, 4, 1, state.StepMain2)
	// Skip the whole chain by moving Forge into exile by hand (an exile is an
	// exile) and adding the grant the DBEffect would register.
	e.emit(events.Event{Kind: events.MoveZone, Obj: forge, From: state.ZHand, To: state.ZExile, Player: 0, Counter: "exiled_by"})
	e.AddContinuous(state.ContinuousEffect{
		Source: widow, Controller: 1, UntilEOT: true,
		Affects: "Card.nonLand+IsRemembered", AffectedZone: "Exile",
		MayPlay: true, MayPlayIgnoreType: true,
		Remembered: []state.ObjID{forge},
		Duration:   "",
	})
	e.priorityRound()
	if ianOffers(e, forge) || ianOffers(e, gg) || ianOffers(e, ls) {
		t.Fatalf("empty pool offered a cast (forge=%v, hand We Say Thee Nay=%v, hand Cerulean Drake=%v): %v",
			ianOffers(e, forge), ianOffers(e, gg), ianOffers(e, ls), mpRiderLabels(e))
	}
	// Float {U}{C} (Island + Training Compound's tap mana): the granted cast
	// is offered once the floating pool alone pays it.
	addManaQuiet(t, e, 1, "UC")
	e.priorityRound()
	if !ianOffers(e, forge) {
		t.Fatalf("floated {U}{C} still withheld the granted cast: %v", mpRiderLabels(e))
	}
	replayCheck(t, e, cfg)
}
