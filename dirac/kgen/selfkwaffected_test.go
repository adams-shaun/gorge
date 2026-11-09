package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestAffectedKwSelfDependencyRefused: a keyword-granting static whose
// Affected$ tests a keyword other than flying (Manor Gargoyle grants itself
// indestructible while it has defender) refuses by name: the kernel's keyword read
// of the affected permanent would evaluate the same static again without
// end. A static that tests a keyword but writes none (Surge Engine's blue
// while it lacks defender) still lowers.
func TestAffectedKwSelfDependencyRefused(t *testing.T) {
	const why = "keyword static whose Affected$ tests a keyword (layer-6 self-dependency)"
	for _, name := range []string{"Manor Gargoyle"} {
		all, _ := keclBuild(t, []string{name})
		i := keclIndex(t, all, name)
		rs := lowerRefusals(all[i])
		if !slices.ContainsFunc(rs, func(r refusal) bool { return strings.Contains(r.Reason, why) }) {
			t.Errorf("%s: refusals %v, want %q", name, rs, why)
		}
	}
	const clean = "Surge Engine"
	all, _ := keclBuild(t, []string{clean})
	i := keclIndex(t, all, clean)
	if rs := lowerRefusals(all[i]); len(rs) > 0 {
		t.Errorf("%s: refusals %v, want none", clean, rs)
	}
}
