package rules

// registration_remembered_strict_test.go pins CR 611.2 for every
// Effect-registered restriction reader: the scope of a registered restriction
// whose spec says Card.IsRemembered is the set the Effect captured, never the
// source card's later persistent memory. The measured widening shape is
// Mobilize's `RememberTokens$ True`, which appends its Warrior token to the
// source card's Remembered list after the restriction was registered; a
// non-strict union then applies the restriction (must block, doesn't untap,
// can't be targeted) to a creature the Effect never captured.
//
// Each test registers a single ContinuousEffect with Remembered: [A] and then
// seeds the SOURCE's persistent list with BOTH A and B, so the non-strict
// union would select B too. The reader must select A and not B.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// registrationStrictFixture builds the measured shape: a source permanent
// that registers a restriction capturing creature A, then later remembers B
// for an unrelated ability. Returns an attacker for the block-direction
// charge readers. The precondition at the end is the whole point of the test:
// the source's persistent list really holds the later object, so a non-strict
// IsRemembered read would see it.
func registrationStrictFixture(t *testing.T) (e *Engine, src, a, b, attacker state.ObjID) {
	t.Helper()
	e = newSeats(t, 2)
	src = onBoard(t, e, 0, "Name:Strict Source\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
	a = onBoard(t, e, 0, "Name:Captured Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b = onBoard(t, e, 1, "Name:Later Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	attacker = onBoard(t, e, 1, "Name:Beast Attacker\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source not on battlefield: %+v", o)
	}
	// The later unrelated memory (Mobilize's RememberTokens$ True appends its
	// Warrior to the source card's persistent list).
	e.G.Obj(src).Remembered = []state.Target{{Obj: a}, {Obj: b}}
	if !e.matchesSpec("Card.IsRemembered", a, e.specCtx(src, 0)) {
		t.Fatal("precondition: the non-strict union lost the captured object")
	}
	if !e.matchesSpec("Card.IsRemembered", b, e.specCtx(src, 0)) {
		t.Fatal("precondition: the non-strict union must see the source's later persistent remember")
	}
	return e, src, a, b, attacker
}

// addStrictRegistration registers one Effect-delivered restriction and
// returns it from the live registry (a precondition failure otherwise: the
// readers all walk e.active()/e.continuous, so a registration the engine did
// not keep would make every assertion below vacuous).
func addStrictRegistration(t *testing.T, e *Engine, mode string, params map[string]string, remembered []state.ObjID, src state.ObjID) *ContinuousEffect {
	t.Helper()
	e.AddContinuous(ContinuousEffect{
		Source: src, Controller: 0, FromEffect: true,
		Restriction: mode, RestrictParams: params, Remembered: remembered,
	})
	for i := range e.continuous {
		if e.continuous[i].Source == src && e.continuous[i].Restriction == mode {
			return &e.continuous[i]
		}
	}
	t.Fatalf("precondition: %s registration missing from %+v", mode, e.continuous)
	return nil
}

func TestRestrictionAppliesStrictRememberedScope(t *testing.T) {
	t.Parallel()
	e, src, a, b, _ := registrationStrictFixture(t)
	ce := addStrictRegistration(t, e, "MustBlock",
		map[string]string{"ValidCreature": "Card.IsRemembered"}, []state.ObjID{a}, src)
	if !e.restrictionApplies(ce, a) {
		t.Fatal("MustBlock restriction lost the captured creature")
	}
	if e.restrictionApplies(ce, b) {
		t.Fatal("MustBlock restriction read the source's later persistent memory")
	}
}

func TestUntapDeniedStrictRememberedScope(t *testing.T) {
	t.Parallel()
	e, src, a, b, _ := registrationStrictFixture(t)
	addStrictRegistration(t, e, "UntapDenial",
		map[string]string{"Affected": "Creature.IsRemembered"}, []state.ObjID{a}, src)
	if !e.untapDenied(a) {
		t.Fatal("UntapDenial lost the captured creature")
	}
	if e.untapDenied(b) {
		t.Fatal("UntapDenial read the source's later persistent memory")
	}
}

func TestCantAttackUnlessStrictRememberedScope(t *testing.T) {
	t.Parallel()
	e, src, a, b, _ := registrationStrictFixture(t)
	addStrictRegistration(t, e, "CantAttackUnless",
		map[string]string{"ValidCard": "Creature.IsRemembered", "Cost": "2"}, []state.ObjID{a}, src)
	if e.attackPairCharge(a, 1, 0).zero() {
		t.Fatal("CantAttackUnless did not charge the captured attacker")
	}
	if !e.attackPairCharge(b, 1, 0).zero() {
		t.Fatal("CantAttackUnless charged the source's later remembered creature")
	}
}

func TestCantBlockUnlessStrictRememberedScope(t *testing.T) {
	t.Parallel()
	e, src, a, b, attacker := registrationStrictFixture(t)
	addStrictRegistration(t, e, "CantBlockUnless",
		map[string]string{"ValidCard": "Creature.IsRemembered", "Cost": "2"}, []state.ObjID{a}, src)
	if e.blockPairCharge(a, attacker).zero() {
		t.Fatal("CantBlockUnless did not charge the captured blocker")
	}
	if !e.blockPairCharge(b, attacker).zero() {
		t.Fatal("CantBlockUnless charged the source's later remembered blocker")
	}
}

func TestRestrictionBlocksTargetStrictRememberedScope(t *testing.T) {
	t.Parallel()
	e, src, a, b, _ := registrationStrictFixture(t)
	addStrictRegistration(t, e, "CantTarget",
		map[string]string{"ValidCard": "Card.IsRemembered"}, []state.ObjID{a}, src)
	if !e.restrictionBlocksTarget(a, 1, 0, nil) {
		t.Fatal("CantTarget lost the captured creature")
	}
	if e.restrictionBlocksTarget(b, 1, 0, nil) {
		t.Fatal("CantTarget read the source's later persistent memory")
	}

	// The printed-static path shares cantTargetBlocks but not the Effect
	// binding: effectDelivered is false there, so its remembered set is the
	// static's own and the source union must stay. B is in that union.
	printed := cantTargetBodyOfStatic(staticView{
		Source: src, Controller: 0,
		Params:     map[string]string{"ValidCard": "Card.IsRemembered"},
		Remembered: []state.ObjID{a},
	})
	if !e.cantTargetBlocks(printed, b, 1, 0, nil, src, 0, e.specCtx(src, 0)) {
		t.Fatal("printed-static CantTarget stopped unioning the source's remembered list")
	}
}
