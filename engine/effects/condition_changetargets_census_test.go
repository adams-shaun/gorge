package effects

import (
	"strings"
	"testing"
)

// TestEmissaryOfGrudgesStaysUnmodelled: Emissary of Grudges' ChangeTargets
// ability carries the two target-shape keys this build now evaluates AND
// ConditionPlayerDefined$/ConditionPlayerContains$, which stay unmodelled, so
// the gate must keep failing CLOSED on it (unmodelledConditionKey returns the
// first sorted bad key). Meddle, which carries only the two evaluated keys,
// no longer reports unmodelled.
func TestEmissaryOfGrudgesStaysUnmodelled(t *testing.T) {
	_, em := corpusSA(t, "Emissary of Grudges", "")
	if em.API != "ChangeTargets" {
		t.Fatalf("precondition: Emissary ability API = %q, want ChangeTargets", em.API)
	}
	if _, ok := em.Params["ConditionPlayerDefined"]; !ok {
		t.Fatal("precondition: Emissary ability carries no ConditionPlayerDefined$")
	}
	if _, ok := em.Params["ConditionTargetValidTargeting"]; !ok {
		t.Fatal("precondition: Emissary ability carries no ConditionTargetValidTargeting$")
	}
	// unmodelledConditionKey returns the FIRST SORTED bad key: of the two
	// player keys Emissary carries, ConditionPlayerContains sorts first.
	if detail, bad := UnmodelledCondition(em); !bad || !strings.HasPrefix(detail, "ConditionPlayer") {
		t.Fatalf("UnmodelledCondition(Emissary) = (%q, %v), want a still-unmodelled player key", detail, bad)
	}

	_, meddle := corpusSA(t, "Meddle", "")
	if meddle.API != "ChangeTargets" {
		t.Fatalf("precondition: Meddle ability API = %q, want ChangeTargets", meddle.API)
	}
	if _, ok := meddle.Params["ConditionTargetValidTargeting"]; !ok {
		t.Fatal("precondition: Meddle ability carries no ConditionTargetValidTargeting$")
	}
	if _, ok := meddle.Params["ConditionTargetsSingleTarget"]; !ok {
		t.Fatal("precondition: Meddle ability carries no ConditionTargetsSingleTarget$")
	}
	if detail, bad := UnmodelledCondition(meddle); bad {
		t.Fatalf("Meddle's ChangeTargets is still unmodelled: %s", detail)
	}
}
