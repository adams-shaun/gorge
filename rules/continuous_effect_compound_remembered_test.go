package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCompoundRememberedRestrictionDoesNotOverapply is fix F2: a restriction
// whose valid-spec carries IsRemembered combined with an extra predicate
// (Card.IsRemembered+Creature, a + AND or a , OR list) cannot be resolved by
// the remembered-set match alone -- the extra predicate would be silently
// dropped and the restriction would apply to a remembered object that fails
// it. Measured, no corpus CantTarget/CantRegenerate static carries one (all
// 18 are a bare Card.IsRemembered), so the code is given the right edge:
// REJECT the compound spec and let the restriction not apply, rather than
// silently over-apply. The test registers a restriction that remembers an
// object that is NOT a creature and asserts the restriction does not bite it.
func TestCompoundRememberedRestrictionDoesNotOverapply(t *testing.T) {
	e := handEngine(t)
	// A remembered permanent that fails the compound's extra "Creature"
	// predicate: an artifact, not a creature.
	artifact := onBoard(t, e, 0, "Name:Bauble\nManaCost:0\nTypes:Artifact\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{
		Source: artifact, Controller: 0,
		Restriction: "CantTarget",
		RestrictParams: map[string]string{
			"ValidTarget": "Card.IsRemembered+Creature",
			"Activator":   "Player.Opponent",
		},
		Remembered: []state.ObjID{artifact},
		Duration:   "UntilTheEndOfYourNextTurn",
	})
	// The object is remembered, so a naive remembered-set match would return
	// true (over-applying: it is not a creature). The compound rejection must
	// make the restriction not apply.
	if e.restrictionBlocksTarget(artifact, 1) {
		t.Fatal("compound IsRemembered spec over-applied to a remembered non-creature")
	}
	// And a bare IsRemembered spec (the only shape the corpus uses) must keep
	// working: a remembered object of ANY type is blocked by it.
	e.AddContinuous(ContinuousEffect{
		Source: artifact, Controller: 0,
		Restriction:    "CantTarget",
		RestrictParams: map[string]string{"ValidTarget": "Card.IsRemembered", "Activator": "Player.Opponent"},
		Remembered:     []state.ObjID{artifact},
		Duration:       "UntilTheEndOfYourNextTurn",
	})
	if !e.restrictionBlocksTarget(artifact, 1) {
		t.Fatal("bare IsRemembered spec should still block a remembered object")
	}
}

// TestCompoundRememberedEffectStaticFallsThroughToNote is the registration
// side of F2: a DB$ Effect whose CantTarget static carries a compound
// IsRemembered spec is treated as unsupported (a Note, no registration)
// rather than registered as a restriction that would over-apply. No corpus
// card reaches this, so it is purely a guard.
func TestCompoundRememberedEffectStaticFallsThroughToNote(t *testing.T) {
	guard := card(t, "Name:Guard\nManaCost:G\nTypes:Instant\n"+
		"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ +2 | NumDef$ +2 | SubAbility$ DBEffect\n"+
		"SVar:DBEffect:DB$ Effect | Defined$ Targeted | Duration$ UntilTheEndOfYourNextTurn | StaticAbilities$ Guard | RememberObjects$ Targeted\n"+
		"SVar:Guard:Mode$ CantTarget | ValidTarget$ Card.IsRemembered+Creature | Activator$ Player.Opponent\nOracle:x\n")
	e := handEngine(t, guard)
	e.G.Players[0].Pool[state.MG] = 1
	creature := onBoard(t, e, 0, "Name:Elf\nManaCost:G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")

	e.askPriority(0)
	castFirst(t, e, "cast")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Obj == creature {
			idx = o.Index
		}
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)
	// The compound IsRemembered static must NOT have registered a
	// restriction: it falls through to the honest Note instead.
	for _, ce := range e.continuous {
		if ce.Restriction == "CantTarget" {
			t.Fatal("compound IsRemembered static was registered; it should have fallen through to the Note")
		}
	}
}
