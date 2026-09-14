package state

import "testing"

// The brief's table test: every Forge PhaseType script name and enum name,
// the case tolerance the corpus relies on, the range/list/Main/All forms,
// and the unknown names that must be reported rather than matched or
// dropped. Expected sets are written as explicit step lists in turn order,
// which is also what Steps() must return.

func setOf(steps ...Step) StepSet {
	var s StepSet
	for _, st := range steps {
		s |= 1 << uint(st)
	}
	return s
}

func TestParsePhasesScriptNames(t *testing.T) {
	cases := []struct {
		spec string
		want []Step
	}{
		// Every Forge script name, exact case.
		{"Untap", []Step{StepUntap}},
		{"Upkeep", []Step{StepUpkeep}},
		{"Draw", []Step{StepDraw}},
		{"Main1", []Step{StepMain1}},
		{"BeginCombat", []Step{StepBeginCombat}},
		{"Declare Attackers", []Step{StepDeclareAttackers}},
		{"Declare Blockers", []Step{StepDeclareBlockers}},
		{"First Strike Damage", []Step{StepCombatDamage}},
		{"Combat Damage", []Step{StepCombatDamage}},
		{"EndCombat", []Step{StepEndCombat}},
		{"Main2", []Step{StepMain2}},
		{"End of Turn", []Step{StepEnd}},
		{"Cleanup", []Step{StepCleanup}},
		// The corpus's own case variant (24 raw lines write `End Of Turn`).
		{"End Of Turn", []Step{StepEnd}},
		// Case-insensitive in both directions.
		{"end of turn", []Step{StepEnd}},
		{"ENDCOMBAT", []Step{StepEndCombat}},
		{"begincombat", []Step{StepBeginCombat}},
		// Every Forge enum name.
		{"UNTAP", []Step{StepUntap}},
		{"COMBAT_BEGIN", []Step{StepBeginCombat}},
		{"COMBAT_DECLARE_ATTACKERS", []Step{StepDeclareAttackers}},
		{"COMBAT_DECLARE_BLOCKERS", []Step{StepDeclareBlockers}},
		{"COMBAT_FIRST_STRIKE_DAMAGE", []Step{StepCombatDamage}},
		{"COMBAT_DAMAGE", []Step{StepCombatDamage}},
		{"COMBAT_END", []Step{StepEndCombat}},
		{"END_OF_TURN", []Step{StepEnd}},
		{"CLEANUP", []Step{StepCleanup}},
		{"combat_begin", []Step{StepBeginCombat}},
	}
	for _, tc := range cases {
		set, unknown := ParsePhases(tc.spec)
		if len(unknown) != 0 {
			t.Errorf("ParsePhases(%q) unknown = %v, want none", tc.spec, unknown)
		}
		if got := set.Steps(); !equalSteps(got, tc.want) {
			t.Errorf("ParsePhases(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}

func TestParsePhasesMainListRangeAll(t *testing.T) {
	cases := []struct {
		spec string
		want []Step
	}{
		{"Main", []Step{StepMain1, StepMain2}},
		{"main", []Step{StepMain1, StepMain2}},
		{"Main1,Main2", []Step{StepMain1, StepMain2}},
		{"Upkeep->", []Step{StepUpkeep, StepDraw, StepMain1, StepBeginCombat,
			StepDeclareAttackers, StepDeclareBlockers, StepCombatDamage,
			StepEndCombat, StepMain2, StepEnd, StepCleanup}},
		{"BeginCombat->EndCombat", []Step{StepBeginCombat, StepDeclareAttackers,
			StepDeclareBlockers, StepCombatDamage, StepEndCombat}},
		{"Untap->Draw", []Step{StepUntap, StepUpkeep, StepDraw}},
		// All names every step; a blank spec is the empty set.
		{"All", allStepSlice()},
		{"", nil},
	}
	for _, tc := range cases {
		set, unknown := ParsePhases(tc.spec)
		if len(unknown) != 0 {
			t.Errorf("ParsePhases(%q) unknown = %v, want none", tc.spec, unknown)
		}
		if got := set.Steps(); !equalSteps(got, tc.want) {
			t.Errorf("ParsePhases(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}

func TestParsePhasesUnknownReported(t *testing.T) {
	cases := []struct {
		spec     string
		wantSet  []Step
		wantUnkn []string
	}{
		// The names the two deleted substring parsers used to accept: bare
		// "End" and "endstep" are NOT Forge script names and must be
		// reported, never matched.
		{"End", nil, []string{"End"}},
		{"endstep", nil, []string{"endstep"}},
		{"Beginning", nil, []string{"Beginning"}},
		// A mixed list reports only the unknown element, in spec order.
		{"Upkeep,Beginning,Draw", []Step{StepUpkeep, StepDraw},
			[]string{"Beginning"}},
		{"Combat", nil, []string{"Combat"}},
		// A range with an unknown end reports the whole element.
		{"Upkeep->Never", nil, []string{"Upkeep->Never"}},
		// An empty element inside a list is reported too (Forge throws).
		{"Upkeep,", []Step{StepUpkeep}, []string{""}},
	}
	for _, tc := range cases {
		set, unknown := ParsePhases(tc.spec)
		if got := set.Steps(); !equalSteps(got, tc.wantSet) {
			t.Errorf("ParsePhases(%q) = %v, want %v", tc.spec, got, tc.wantSet)
		}
		if len(unknown) != len(tc.wantUnkn) {
			t.Fatalf("ParsePhases(%q) unknown = %v, want %v", tc.spec, unknown, tc.wantUnkn)
		}
		for i := range unknown {
			if unknown[i] != tc.wantUnkn[i] {
				t.Errorf("ParsePhases(%q) unknown[%d] = %q, want %q", tc.spec, i, unknown[i], tc.wantUnkn[i])
			}
		}
	}
}

func TestStepSetHasAndEmpty(t *testing.T) {
	var zero StepSet
	if !zero.Empty() {
		t.Error("zero StepSet is not empty")
	}
	if zero.Has(StepUntap) {
		t.Error("zero StepSet contains untap")
	}
	if AllSteps().Empty() {
		t.Error("AllSteps is empty")
	}
	// An invalid step is in no set, including AllSteps.
	var bogus Step = Step(99)
	if AllSteps().Has(bogus) {
		t.Error("AllSteps contains an invalid step")
	}
}

func TestEarliestAfter(t *testing.T) {
	cases := []struct {
		set  []Step
		cur  Step
		want Step
	}{
		// Single-step sets: the registered step, wrapping to next turn when
		// it has already passed (a delayed `End of Turn` registered during
		// the end step fires at the NEXT end step).
		{[]Step{StepEnd}, StepMain2, StepEnd},
		{[]Step{StepEnd}, StepEnd, StepEnd},
		{[]Step{StepUpkeep}, StepUpkeep, StepUpkeep},
		{[]Step{StepUpkeep}, StepMain2, StepUpkeep},
		// Multi-step sets collapse to the first member still to come.
		{[]Step{StepMain1, StepMain2}, StepCombatDamage, StepMain2},
		{[]Step{StepMain1, StepMain2}, StepUntap, StepMain1},
		{[]Step{StepMain1, StepMain2}, StepMain1, StepMain2},
		// A wrapping range registered during cleanup reaches next turn's
		// first member.
		{[]Step{StepUpkeep, StepEnd}, StepCleanup, StepUpkeep},
		{[]Step{StepUntap, StepUpkeep}, StepCleanup, StepUntap},
	}
	for _, tc := range cases {
		got, ok := EarliestAfter(setOf(tc.set...), tc.cur)
		if !ok || got != tc.want {
			t.Errorf("EarliestAfter(%v, %v) = %v,%v want %v,true", tc.set, tc.cur, got, ok, tc.want)
		}
	}
	if _, ok := EarliestAfter(0, StepUntap); ok {
		t.Error("EarliestAfter of the empty set reported a step")
	}
}

func equalSteps(got, want []Step) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func allStepSlice() []Step {
	var out []Step
	for i := 0; i < numSteps; i++ {
		out = append(out, Step(i))
	}
	return out
}
