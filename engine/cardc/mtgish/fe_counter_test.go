package mtgish

import (
	"sort"
	"strings"
	"testing"
)

func TestCounterWords(t *testing.T) {
	pt := func(p, q float64) node { return valNode("CounterType", "PTCounter", []any{p, q}) }
	for _, tc := range []struct {
		in   node
		want string
	}{
		{pt(1, 1), "P1P1"}, {pt(-1, -1), "M1M1"}, {pt(0, 1), "P0P1"}, {pt(2, 0), "P2P0"},
	} {
		got, err := (&Ctx{}).lower(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("PTCounter = %v, %v; want %s", got, err, tc.want)
		}
	}
	if !sort.StringsAreSorted(valCounterNames) {
		t.Error("valCounterNames is not sorted")
	}
	for _, name := range valCounterNames {
		got, err := (&Ctx{}).lower(valNode("CounterType", name, nil))
		want := strings.ToUpper(strings.TrimSuffix(name, "Counter"))
		if err != nil || got != want {
			t.Errorf("%s = %v, %v; want %s", name, got, err, want)
		}
	}
	got, _ := (&Ctx{}).lower(valNode("CounterType", "ChargeCounter", nil))
	if got != "CHARGE" {
		t.Errorf("ChargeCounter = %v", got)
	}
}

// TestValueSliceVariantsRegistered checks the schema against the registry
// (every no-arg *Counter variant is in the list) and the proof slice's use.
func TestValueSliceVariantsRegistered(t *testing.T) {
	s := corpusSchema(t)
	listed := map[string]bool{}
	for _, n := range valCounterNames {
		listed[n] = true
	}
	for _, v := range s.Variants {
		if v.Enum == "CounterType" && v.Args == "" && len(v.Fields) == 0 && strings.HasSuffix(v.Name, "Counter") && !listed[v.Name] {
			t.Errorf("CounterType.%s is in the schema but not in valCounterNames", v.Name)
		}
	}
	checkFamilyCoverage(t, []string{"GameNumber", "Comparison", "CounterType"},
		map[string]string{"Comparison.OneOf": "a set of values"})
}
