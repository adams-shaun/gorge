package rules

import "testing"

// cr-g202: census-only reclassification, no behaviour change. Six keys that
// Forge's own effect for the named API never reads are reported unread on six
// Standard cards. Each is read by OTHER primitives (CounterNum by PutCounter,
// NumCards by Draw, ETB by Tap/PutCounter/Untap, Hidden by ChangeZone), so
// each entry must be API-scoped in ignoredAbilityParams, never key-global in
// ignoredParamKeys. Test-only: the six ignoredAbilityParams entries, the
// regenerated KnownKeys tables and the set TSVs.

// TestInertAPIKeysAreScopedNotGlobal pins the six API-scoped entries and the
// global table's silence on the same keys. The api:PutCounter / api:Draw /
// api:Tap / api:ChangeZone probes are the leak guard: moving any of these keys
// into ignoredParamKeys makes the sibling primitive's genuine read disappear.
func TestInertAPIKeysAreScopedNotGlobal(t *testing.T) {
	t.Parallel()
	scoped := []struct{ prim, key string }{
		{"api:Sacrifice", "OrString"},
		{"api:DealDamage", "CounterNum"},
		{"api:ChangeZone", "NumCards"},
		{"api:ChooseType", "ETB"},
		{"api:DestroyAll", "Hidden"},
		{"api:ChangeZoneAll", "DefinedPlayer"},
	}
	for _, c := range scoped {
		if !ignoredParam(c.prim, c.key) {
			t.Errorf("ignoredParam(%q, %q) = false, want true (API-scoped inert key)", c.prim, c.key)
		}
		// Precondition for the sibling probes below: the entry really is
		// scoped, not a global key that happens to answer this API too.
		if ignoredParamKeys[c.key] != "" {
			t.Errorf("ignoredParamKeys[%q] = %q, want empty: the key is API-scoped only", c.key, ignoredParamKeys[c.key])
		}
	}
	stillRead := []struct{ prim, key string }{
		{"api:PutCounter", "CounterNum"},
		{"api:Draw", "NumCards"},
		{"api:Tap", "ETB"},
		{"api:ChangeZone", "Hidden"},
	}
	for _, c := range stillRead {
		// Each sibling primitive genuinely reads the key, so the census must
		// keep it live here.
		if ignoredParam(c.prim, c.key) {
			t.Errorf("ignoredParam(%q, %q) = true, want false: the entry leaked into the global table or another API's scope", c.prim, c.key)
		}
	}
}
