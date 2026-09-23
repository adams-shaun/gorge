package cards

import (
	"reflect"
	"testing"
)

// TestSVarReferencedCorpusPrimitives pins the two corpus APIs reachable only
// through SVar-naming parameters rather than printed SubAbility$ chains. The
// support map deliberately marks every primitive present except the finding's
// API, so Unsupported must expose exactly that hidden primitive. Without the
// face-wide SVar walk, both cards incorrectly appear supported.
func TestSVarReferencedCorpusPrimitives(t *testing.T) {
	r := compiledCorpus(t)
	for _, tc := range []struct {
		name    string
		missing string
	}{
		{name: "Cankerbloom", missing: "api:Proliferate"},
		{name: "Noxious Assault", missing: "api:Poison"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := r.Lookup(tc.name)
			if !ok {
				t.Fatalf("corpus card %q not found", tc.name)
			}
			primitives := c.Primitives()
			supported := make(map[string]bool, len(primitives))
			for _, p := range primitives {
				supported[p] = true
			}
			delete(supported, tc.missing)
			if got := r.Unsupported(c, supported); !reflect.DeepEqual(got, []string{tc.missing}) {
				t.Fatalf("deep missing primitives = %v, want exactly [%s]; card primitives = %v", got, tc.missing, primitives)
			}
		})
	}
}
