package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Luminous Rebuke shape: a spell whose cost is {3} cheaper when it
// targets a tapped creature, so an UNTAPPED target reprices the spell to
// {4}{W} while a tapped one keeps it at {1}{W}.
const rebukeMenuSpellSrc = "Name:Test Rebuke\nManaCost:4 W\nTypes:Instant\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 3 | EffectZone$ All | ValidTarget$ Creature.tapped\n" +
	"A:SP$ Destroy | ValidTgts$ Creature\nOracle:x\n"

const rebukeMenuBearSrc = "Name:Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// rebukeMenuGame builds a hand with the spell, two untapped Plains on the
// battlefield (the whole CR 601.2g window: {W}{W}) and two creature targets,
// one untapped and one tapped. It returns the engine, the spell, and the two
// bear ids (untapped first).
func rebukeMenuGame(t *testing.T) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	spell := card(t, rebukeMenuSpellSrc)
	e := handEngine(t, spell)
	plains := "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
	battlePerm(t, e, 0, plains)
	battlePerm(t, e, 0, plains)
	untapped := battlePerm(t, e, 0, rebukeMenuBearSrc)
	tapped := battlePerm(t, e, 0, rebukeMenuBearSrc)
	e.G.Obj(tapped).Tapped = true
	return e, e.G.Zone(state.ZHand, 0)[0], untapped, tapped
}

// TestRebukeMenuWithholdsTargetThatRepricesUnpayable is the regression for
// the 20000-game census finding "cast aborted: cost no longer payable" at
// Luminous Rebuke (17 games): the target menu kept an untapped creature on
// the strength of an arbitrary untapped mana source, but its selection
// reprices the spell to {4}{W} and the window can produce only {W}{W}, so
// the payment was certain to abort at CR 601.2h. CR 601.2c: selecting a
// target whose final cost is unpayable is not a legal choice, so the menu
// must withhold the untapped target and offer the tapped one, whose repriced
// {1}{W} the window reaches exactly.
func TestRebukeMenuWithholdsTargetThatRepricesUnpayable(t *testing.T) {
	t.Parallel()
	e, spellID, untapped, tapped := rebukeMenuGame(t)
	// Preconditions: the reduction static really fires per candidate, and the
	// window really is only {W}{W} -- without both the menu below would be
	// trivial and the assertion vacuous.
	untappedTarget := state.Target{Obj: untapped}
	tappedTarget := state.Target{Obj: tapped}
	if got := e.costModifiersForTargets(0, spellID, spellScope(""), []state.Target{untappedTarget}).Apply(e.parseCost("4 W")).Generic; got != 4 {
		t.Fatalf("precondition: untapped-target price generic = %d, want 4 (no reduction)", got)
	}
	if got := e.costModifiersForTargets(0, spellID, spellScope(""), []state.Target{tappedTarget}).Apply(e.parseCost("4 W")).Generic; got != 1 {
		t.Fatalf("precondition: tapped-target price generic = %d, want 1 ({3} less)", got)
	}
	if !e.hasUntappedManaSource(0) {
		t.Fatal("precondition: no untapped mana source (the window reach is zero)")
	}

	e.cast = &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1,
		cost: e.parseCost("4 W")}
	e.continueCast()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want target decision, got %+v", d)
	}
	offered := map[state.ObjID]bool{}
	for _, o := range d.Options {
		if o.Obj == untapped {
			t.Errorf("untapped creature offered: its repriced {4}{W} is beyond the {W}{W} window (CR 601.2c): %+v", d.Options)
		}
		offered[o.Obj] = true
	}
	if !offered[tapped] {
		t.Errorf("tapped creature not offered; menu = %+v", d.Options)
	}

	// And the offered cast actually completes at the tapped target's price,
	// paid from the window: the pool is empty, the two Plains pay {1}{W}.
	targetObject(t, e, tapped)
	for i := 0; i < 60; i++ {
		dd := e.Pending()
		if dd == nil {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				break
			}
			e.priorityRound()
			continue
		}
		if dd.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				break
			}
			idx := -1
			for _, o := range dd.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", dd.Options)
			}
			submitChoices(t, e, idx)
			continue
		}
		if len(dd.Options) == 0 {
			t.Fatalf("empty decision: %+v", dd)
		}
		submitChoices(t, e, dd.Options[0].Index)
	}
	if o := e.G.Obj(tapped); o.Zone != state.ZGraveyard {
		t.Fatalf("tapped creature %s, want graveyard (the cast resolved)", o.Zone)
	}
	if o := e.G.Obj(untapped); o.Zone != state.ZBattlefield {
		t.Fatalf("untapped creature %s, want battlefield", o.Zone)
	}
	if e.G.Obj(spellID).Zone != state.ZGraveyard {
		t.Fatalf("spell %s, want graveyard", e.G.Obj(spellID).Zone)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the cast = %d, want 0", got)
	}
	if hasNote(e, "no longer payable") {
		t.Fatal("payment abort Note: the cast the menu filtered should have resolved")
	}
}

// TestRebukeMenuEmptyWithholdsAborts pins the fail-open shape: when BOTH
// targets are untapped, neither reprices into the window's reach ({4}{W}
// each, the window is {W}{W}), the menu is empty and the proposal must abort
// with the usual no-legal-target Note and clear the flow -- the filter must
// not strand the flow on an empty menu.
func TestRebukeMenuEmptyWithholdsAborts(t *testing.T) {
	t.Parallel()
	e, spellID, _, _ := rebukeMenuGame(t)
	// Both bears untapped (the helper taps the second one): neither reprices
	// into the window's reach ({4}{W} each, the window is {W}{W}), the menu
	// is empty and the proposal aborts.
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() {
			o.Tapped = false
		}
	}
	e.cast = &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1,
		cost: e.parseCost("4 W")}
	e.continueCast()
	if !hasNote(e, "cast aborted") {
		t.Fatal("no abort Note: an empty target menu did not reverse the proposal")
	}
	if e.cast != nil || e.choosing != chooseNone {
		t.Fatal("cast flow not cleared")
	}
	if e.G.Obj(spellID).Zone != state.ZHand {
		t.Fatalf("spell %s, want hand (an aborted cast moves nothing)", e.G.Obj(spellID).Zone)
	}
}

// TestRebukeMenuCensusIgnoresTapBlockedSource is the regression for the
// 20000-game census' remaining 17 "cast aborted: cost no longer payable"
// notes at Luminous Rebuke: a TAPPED third mana source (a Plains with a
// tap-required {W} activation) left the CR 601.2g window census
// incomplete -- castWindowCensusComplete counted every probe-admitted but
// unpriced source as "the unpriced ability might close the gap", even when
// the only reason it had no priced unit was that it is TAPPED. The
// incomplete census then failed the target menu open and the unpayable
// {4}{W} untapped target stayed on the menu; the payer picked it and the
// payment aborted (CR 601.2h). A tapped source with a tap-required
// activation is a complete census judgement, not an incomplete one: it
// would be priced the moment it untaps, so the menu must judge the
// candidates on the window's concrete productions -- the untapped target
// comes off, the tapped target stays.
func TestRebukeMenuCensusIgnoresTapBlockedSource(t *testing.T) {
	t.Parallel()
	spell := card(t, rebukeMenuSpellSrc)
	e := handEngine(t, spell)
	spellID := e.G.Zone(state.ZHand, 0)[0]
	plains := "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
	battlePerm(t, e, 0, plains)
	battlePerm(t, e, 0, plains)
	tappedPlains := battlePerm(t, e, 0, plains)
	e.G.Obj(tappedPlains).Tapped = true
	untapped := battlePerm(t, e, 0, rebukeMenuBearSrc)
	tapped := battlePerm(t, e, 0, rebukeMenuBearSrc)
	e.G.Obj(tapped).Tapped = true

	// Precondition for the shape this pins: the tapped Plains is
	// probe-admitted (its {W} activation passes the gate walk) but produces
	// no unit, which must not be judged census-incomplete.
	windowUnits := e.castWindowUnits(&pendingCast{player: 0, card: spellID,
		from: state.ZHand, ability: -1, cost: e.parseCost("4 W")})
	if e.castWindowCensusComplete(&pendingCast{player: 0, card: spellID,
		from: state.ZHand, ability: -1, cost: e.parseCost("4 W")}, windowUnits) != true {
		t.Fatalf("precondition: census incomplete with only a tap-blocked source unpriced (the fix regressed)")
	}

	e.cast = &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1,
		cost: e.parseCost("4 W")}
	e.continueCast()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want target decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == untapped {
			t.Errorf("untapped creature offered: its repriced {4}{W} is beyond the {W}{W} window even with the tapped Plains ignored (CR 601.2c): %+v", d.Options)
		}
	}
	offered := false
	for _, o := range d.Options {
		if o.Obj == tapped {
			offered = true
		}
	}
	if !offered {
		t.Errorf("tapped creature not offered; menu = %+v", d.Options)
	}

	// The offered cast completes at the tapped target's {1}{W}, paid from
	// the two untapped Plains, and the whole flow resolves.
	targetObject(t, e, tapped)
	for i := 0; i < 60; i++ {
		dd := e.Pending()
		if dd == nil {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				break
			}
			e.priorityRound()
			continue
		}
		if dd.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				break
			}
			idx := -1
			for _, o := range dd.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", dd.Options)
			}
			submitChoices(t, e, idx)
			continue
		}
		if len(dd.Options) == 0 {
			t.Fatalf("empty decision: %+v", dd)
		}
		submitChoices(t, e, dd.Options[0].Index)
	}
	if o := e.G.Obj(tapped); o.Zone != state.ZGraveyard {
		t.Fatalf("tapped creature %s, want graveyard (the cast resolved)", o.Zone)
	}
	if e.G.Obj(spellID).Zone != state.ZGraveyard {
		t.Fatalf("spell %s, want graveyard", e.G.Obj(spellID).Zone)
	}
	if hasNote(e, "no longer payable") {
		t.Fatal("payment abort Note: the cast the menu filtered should have resolved")
	}
}
