package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyStackObjects returns the CR 707.10 copy objects on the stack in stack
// order. The copy's Controller is the player the Controller$ selector named,
// so this is the direct read of the defect these tests pin.
func copyStackObjects(e *Engine) []*state.Object {
	var out []*state.Object
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil && o.IsCopy {
			out = append(out, o)
		}
	}
	return out
}

// firstCopyTargetAsk passes priority until a copy's MayChooseTarget election
// is the pending decision. Every card below carries MayChooseTarget$ True or
// a CopyForEachCanTarget$ replacement target, so the first copy always poses
// this ask before it resolves.
func firstCopyTargetAsk(t *testing.T, e *Engine, limit int) *decision.Decision {
	t.Helper()
	d := passUntilNonPriority(t, e, limit)
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "copy_targets" {
		t.Fatalf("expected a copy target election, got %+v", d)
	}
	return d
}

// waitForCopyOnStack passes priority until a copy stands on the stack, before
// the priority round lets it resolve. CopyForEachCanTarget$ copies carry a
// fixed replacement target, so they pose no target election of their own and
// must be caught here.
func waitForCopyOnStack(t *testing.T, e *Engine, limit int) *state.Object {
	t.Helper()
	for i := 0; i < limit; i++ {
		if copies := copyStackObjects(e); len(copies) > 0 {
			return copies[0]
		}
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("expected priority while waiting for the copy, got %+v", d)
		}
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
	}
	t.Fatalf("no copy reached the stack within %d priority passes", limit)
	return nil
}

// copyControllerCarrier asserts the corpus card really carries a
// CopySpellAbility with the Controller$ spelling the test exists to cover.
// Without this precondition a look-alike card would make the controller
// assertion meaningless.
func copyControllerCarrier(t *testing.T, c *cards.Card, selector string) {
	t.Helper()
	if c == nil || len(c.Faces) == 0 {
		t.Fatalf("corpus card is missing its face")
	}
	face := c.Faces[0]
	for _, ab := range face.Abilities {
		for _, sa := range spellAbilityChain(ab) {
			if sa.API == "CopySpellAbility" && sa.Params["Controller"] == selector {
				return
			}
		}
	}
	// A trigger's Execute$ body is compiled from the face's SVar table, so
	// the CopySpellAbility line itself stays a raw SVar string.
	want := "Controller$ " + selector
	for _, raw := range face.SVars {
		if strings.Contains(raw, "CopySpellAbility") && strings.Contains(raw, want) {
			return
		}
	}
	t.Fatalf("%s has no CopySpellAbility with %q", face.Name, want)
}

// TestHiveMindCopiesForEachOtherPlayer pins CR 707.10/707.10c for Hive Mind's
// Controller$ NonTriggeredCardController: a spell cast while Hive Mind is in
// play is copied once for EACH OTHER player, and each copy belongs to that
// player. Before the selector was modelled every copy fell back to the
// resolving controller (Hive Mind's controller), so a three-player table made
// one copy for the wrong seat instead of two.
func TestHiveMindCopiesForEachOtherPlayer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	copyControllerCarrier(t, lookup(t, reg, "Hive Mind"), "NonTriggeredCardController")
	e := corpusEngineThree(t, reg,
		[]*cards.Card{lookup(t, reg, "Hive Mind"), lookup(t, reg, "Lightning Bolt")}, nil, nil)
	hive := moveByName(t, e, 0, "Hive Mind", state.ZBattlefield)
	bolt := moveByName(t, e, 0, "Lightning Bolt", state.ZHand)
	// PRECONDITION: the trigger source stands on the battlefield, so the
	// SpellCast trigger actually fires.
	if o := e.G.Obj(hive); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hive Mind is not on the battlefield: %+v", o)
	}
	addMana(t, e, 0, "R")
	submitChoices(t, e, miscCastOption(t, e, bolt))
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target ask, got %+v", d)
	}
	submitChoices(t, e, optForPlayer(t, d, 1))

	// Hive Mind's trigger resolves first. The first copy's target election is
	// posed while the trigger's loop is still mid-flight, so only that copy
	// exists yet; assert it belongs to a non-caster, then answer it and reach
	// the second copy's election, where both copies stand on the stack.
	d = firstCopyTargetAsk(t, e, 80)
	first := e.G.Obj(d.Source)
	if first == nil || !first.IsCopy {
		t.Fatalf("first copy target ask source %d is not a copy", d.Source)
	}
	if first.Controller == 0 {
		t.Fatalf("first Hive Mind copy is controlled by the caster (seat 0); want a non-caster")
	}
	submitChoices(t, e, d.Options[0].Index)
	d = firstCopyTargetAsk(t, e, 80)

	copies := copyStackObjects(e)
	if len(copies) != 2 {
		t.Fatalf("Hive Mind made %d copies, want 2 (one per other player)", len(copies))
	}
	got := make([]state.PlayerID, 0, len(copies))
	for _, o := range copies {
		got = append(got, o.Controller)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []state.PlayerID{1, 2}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Hive Mind copy controllers = %v, want %v", got, want)
	}
	submitChoices(t, e, d.Options[0].Index)
	drainToEnd(t, e, 40)
}

// TestBonusRoundCopyControlledByTriggeredCardController pins Bonus Round's
// Controller$ TriggeredCardController: "that player copies it" means the
// player who CAST the triggering spell, not the controller of the Bonus Round
// effect that watches for it. Seat 0 casts Bonus Round, seat 1 casts the Bolt.
func TestBonusRoundCopyControlledByTriggeredCardController(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	copyControllerCarrier(t, lookup(t, reg, "Bonus Round"), "TriggeredCardController")
	e, _ := miscHandsEngine(t, reg,
		[]string{"Bonus Round"}, []string{"Lightning Bolt"}, nil, nil)
	bonus := miscHandObj(t, e, 0, "Bonus Round")
	bolt := miscHandObj(t, e, 1, "Lightning Bolt")

	addMana(t, e, 0, "RRR")
	submitChoices(t, e, miscCastOption(t, e, bonus))
	passUntilStackEmpty(t, e, 30)

	addMana(t, e, 1, "R")
	submitChoices(t, e, passToCast(t, e, bolt))
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target ask, got %+v", d)
	}
	submitChoices(t, e, optForPlayer(t, d, 0))

	d = firstCopyTargetAsk(t, e, 80)
	copyObj := e.G.Obj(d.Source)
	if copyObj == nil || !copyObj.IsCopy {
		t.Fatalf("copy target ask source %d is not a copy", d.Source)
	}
	if d.Player != 1 || copyObj.Controller != 1 {
		t.Fatalf("Bonus Round copy asked seat %d and is controlled by seat %d; want seat 1 (the caster)",
			d.Player, copyObj.Controller)
	}
	submitChoices(t, e, d.Options[0].Index)
	drainToEnd(t, e, 40)
}

// TestMirrorwingDragonCopyControlledByTriggeredActivator pins Mirrorwing
// Dragon's Controller$ TriggeredActivator: the player who cast the targeting
// spell gets the copies, not the Dragon's controller. Seat 0 casts the Bolt
// at seat 1's Dragon; seat 0's Grizzly Bears is the other legal target, so
// exactly one copy is made and it belongs to seat 0.
func TestMirrorwingDragonCopyControlledByTriggeredActivator(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	copyControllerCarrier(t, lookup(t, reg, "Mirrorwing Dragon"), "TriggeredActivator")
	e, _ := miscHandsEngine(t, reg,
		[]string{"Lightning Bolt"}, nil,
		[]string{"Grizzly Bears"}, []string{"Mirrorwing Dragon"})
	bolt := miscHandObj(t, e, 0, "Lightning Bolt")
	dragon := miscBoardObj(t, e, 1, "Mirrorwing Dragon")
	bears := miscBoardObj(t, e, 0, "Grizzly Bears")

	addMana(t, e, 0, "R")
	submitChoices(t, e, miscCastOption(t, e, bolt))
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target ask, got %+v", d)
	}
	submitChoices(t, e, optForObj(t, d, dragon))

	// CopyForEachCanTarget$ fixes each copy's replacement target, so the copy
	// reaches the stack without a target election; catch it before priority
	// lets it resolve.
	copyObj := waitForCopyOnStack(t, e, 80)
	if copyObj.Controller != 0 {
		t.Fatalf("Mirrorwing copy is controlled by seat %d; want seat 0 (the caster)", copyObj.Controller)
	}
	// The replacement target is the caster's other creature: that is the
	// CopyForEachCanTarget reads running under the same controller.
	if len(copyObj.Targets) != 1 || copyObj.Targets[0].Obj != bears {
		t.Fatalf("Mirrorwing copy targets = %+v; want seat 0's Grizzly Bears", copyObj.Targets)
	}
	drainToEnd(t, e, 40)
}

// TestPrecursorGolemCopyControlledByTriggeredSpellAbilityController pins
// Precursor Golem's Controller$ TriggeredSpellAbilityController: the caster
// of the triggering spell controls the copy, not the Golem's controller.
// Seat 0 casts the Bolt at seat 1's Golem; seat 0's Obsianus Golem is the
// other legal target, so the copy belongs to seat 0 and aims there.
func TestPrecursorGolemCopyControlledByTriggeredSpellAbilityController(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	copyControllerCarrier(t, lookup(t, reg, "Precursor Golem"), "TriggeredSpellAbilityController")
	e, _ := miscHandsEngine(t, reg,
		[]string{"Lightning Bolt"}, nil,
		[]string{"Obsianus Golem"}, []string{"Precursor Golem"})
	bolt := miscHandObj(t, e, 0, "Lightning Bolt")
	golem := miscBoardObj(t, e, 1, "Precursor Golem")
	other := miscBoardObj(t, e, 0, "Obsianus Golem")

	addMana(t, e, 0, "R")
	submitChoices(t, e, miscCastOption(t, e, bolt))
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Lightning Bolt's target ask, got %+v", d)
	}
	submitChoices(t, e, optForObj(t, d, golem))

	copyObj := waitForCopyOnStack(t, e, 80)
	if copyObj.Controller != 0 {
		t.Fatalf("Precursor Golem copy is controlled by seat %d; want seat 0 (the caster)", copyObj.Controller)
	}
	if len(copyObj.Targets) != 1 || copyObj.Targets[0].Obj != other {
		t.Fatalf("Precursor Golem copy targets = %+v; want seat 0's Obsianus Golem", copyObj.Targets)
	}
	drainToEnd(t, e, 40)
}
