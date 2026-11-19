package effects

import "testing"

// TestEmissaryOfGrudgesPlayerGateIsModelled: Emissary of Grudges' ChangeTargets
// ability carries the two target-shape keys AND ConditionPlayerDefined$/
// ConditionPlayerContains$. conditionMetCore now evaluates the player pair
// through the shared player-referent resolver (EffectOwnerPlayers), so the
// gate no longer reports an unmodelled key on it, like Meddle, which carries
// only the two target-shape keys.
func TestEmissaryOfGrudgesPlayerGateIsModelled(t *testing.T) {
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
	if detail, bad := UnmodelledCondition(em); bad {
		t.Fatalf("Emissary's player gate is still unmodelled: %s", detail)
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
