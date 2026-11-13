package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCostPayableRestrictedSource pins the unless ask's offer gate over
// a RESTRICTED mana source the payment may spend (CR 106.6, We Say Thee Nay!).
// Hydraulic Helper's "{T}: Add {U}" carries RestrictValid$
// CantCastNonArtifactSpells: it bars casting a nonartifact spell, not the
// "counter target spell unless its controller pays {2}" tax, so the {U} joins
// the unless census beside a plain {C} land and the {2} becomes payable. The
// gate must not promise a restriction the payment does not admit: mana
// spendable only to cast spells stays out of the census.
func TestUnlessCostPayableRestrictedSource(t *testing.T) {
	t.Parallel()
	helper := "Name:Restricted Helper\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U | RestrictValid$ CantCastNonArtifactSpells\nOracle:x\n"
	kitchen := "Name:Plain Kitchen\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C\nOracle:x\n"
	spellOnly := "Name:Spell-Only Source\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U | RestrictValid$ Spell\nOracle:x\n"

	// Precondition for the negative control: the Spell-only restriction really
	// is one a PurposeOther payment does not admit, read through the same
	// predicate the census consults.
	e0 := handEngine(t)
	spellOnly0 := onBoard(t, e0, 0, spellOnly)
	o0 := e0.G.Obj(spellOnly0)
	if o0 == nil || o0.Zone != state.ZBattlefield || o0.Tapped {
		t.Fatalf("Spell-Only Source precondition failed: %+v", o0)
	}
	mas0 := e0.availableManaAbilitiesForWindow(0, spellOnly0, false)
	if len(mas0) == 0 {
		t.Fatal("Spell-Only Source has no payment-window mana ability — premise false")
	}
	if pay.UnlessRestrictionAdmitsOther(asPayer(e0), 0, effects.ManaOf(mas0[0]).RestrictValid, spellOnly0) {
		t.Fatal("the Spell-only restriction admitted a PurposeOther payment — premise false")
	}

	// The one-admitting-source case: the restricted {U} alone cannot reach a
	// {2} tax, so the gate stays closed.
	e := handEngine(t)
	e.G.Players[0].Pool = state.Mana{}
	helperID := onBoard(t, e, 0, helper)
	if o := e.G.Obj(helperID); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("Helper precondition failed: %+v", o)
	}
	if e.UnlessCostPayable(0, "2") {
		t.Fatal("the restricted {U} alone made a {2} tax payable")
	}

	// The payment case: Helper's admitted {U} beside the plain {C} reaches it.
	kitchenID := onBoard(t, e, 0, kitchen)
	if o := e.G.Obj(kitchenID); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("Kitchen precondition failed: %+v", o)
	}
	if !e.UnlessCostPayable(0, "2") {
		t.Fatal("Helper's CantCastNonArtifactSpells {U} beside a plain {C} must make a {2} tax payable (CR 106.6: the tax is not a spell cast)")
	}

	// The {C} land alone does not reach it: the restricted source is what
	// closed the gap, not an over-wide census.
	e2 := handEngine(t)
	e2.G.Players[0].Pool = state.Mana{}
	onBoard(t, e2, 0, kitchen)
	if e2.UnlessCostPayable(0, "2") {
		t.Fatal("the plain {C} alone made a {2} tax payable -- the positive case is not the restricted source's work")
	}

	// A restriction the unless payment does NOT admit keeps its source out:
	// spell-only {U} beside the plain {C} is still short of {2}.
	e3 := handEngine(t)
	e3.G.Players[0].Pool = state.Mana{}
	onBoard(t, e3, 0, spellOnly)
	onBoard(t, e3, 0, kitchen)
	if e3.UnlessCostPayable(0, "2") {
		t.Fatal("spell-only mana made the {2} tax payable; a PurposeOther payment cannot spend it")
	}
}
