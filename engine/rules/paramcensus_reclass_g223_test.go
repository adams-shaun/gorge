package rules

import (
	"strings"
	"testing"
)

// cr-g223: census-only reclassification, no rules change. Eight keys Forge's
// own effect for the named API never reads (or reads only for presentation)
// are reported unread, and six of them are then loud-degraded by the API's
// runtime unread Note. Each is either read by OTHER primitives or is a real
// presentation/setup key, so each entry must be API-scoped in
// ignoredAbilityParams, never key-global in ignoredParamKeys.

// TestG223InertKeysAreScopedNotGlobal pins the eight API-scoped entries and
// the sibling primitives' silence on the same keys. A key moved into the
// key-global ignoredParamKeys makes the sibling primitive's genuine read
// disappear (M1: ETB on api:SetState), so the still-read probes fail.
func TestG223InertKeysAreScopedNotGlobal(t *testing.T) {
	t.Parallel()
	scoped := []struct{ prim, key string }{
		{"api:ChangeZone", "AlreadyRevealed"},
		{"api:ChangeZone", "ETB"},
		{"api:Play", "ShowCardToActivator"},
		{"api:Pump", "Origin"},
		{"api:ReplaceEffect", "VarType"},
		{"api:Effect", "ExileOnLost"},
		{"api:ImmediateTrigger", "AfterReplacement"},
		{"api:PutCounter", "SpecifyCounter"},
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
		{"api:ChangeZone", "Origin"},
		{"api:Effect", "Abilities"},
		{"api:ImmediateTrigger", "AnnounceMax"},
		{"api:SetState", "ETB"},
	}
	for _, c := range stillRead {
		// Each sibling primitive genuinely reads the key, so the census must
		// keep it live here. M1 (moving ETB into ignoredParamKeys) fails the
		// api:SetState probe because the global table answers every bucket.
		if ignoredParam(c.prim, c.key) {
			t.Errorf("ignoredParam(%q, %q) = true, want false: the entry leaked into the global table or another API's scope", c.prim, c.key)
		}
	}
	// The runtime-table half is a subset of the scoped entries: a key named
	// there without its census entry would silence the runtime Note while the
	// census kept labelling the card.
	for _, k := range runtimeIgnoredAbilityParams {
		api, key, ok := strings.Cut(k, ".")
		if !ok {
			t.Fatalf("runtimeIgnoredAbilityParams entry %q is not <API>.<Key>", k)
		}
		if ignoredAbilityParams[k] == "" {
			t.Errorf("runtimeIgnoredAbilityParams names %q but ignoredAbilityParams has no entry for it", k)
		}
		if !ignoredParam("api:"+api, key) {
			t.Errorf("ignoredParam(%q, %q) = false, want true (runtime-table key must also be census-scoped)", "api:"+api, key)
		}
	}
	if ignoredParam("api:ImmediateTrigger", "AnnounceMax") {
		t.Error("ImmediateTrigger.AnnounceMax is a real read and must not be ignored")
	}
}
