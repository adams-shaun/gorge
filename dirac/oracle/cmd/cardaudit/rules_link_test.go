package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
)

// TestSupportedIncludesNonAPIPrimitives pins main.go's blank engine/rules
// import. That package's init() functions register the keyword, trigger,
// static and replacement primitives; without the import this binary sees the
// "api:" primitives only, and g_claim calls every other card missing.
func TestSupportedIncludesNonAPIPrimitives(t *testing.T) {
	for _, key := range []string{"kw:Flying", "trig:ChangesZone", "stat:Continuous", "repl:PayLife"} {
		if !effects.Supported()[key] {
			t.Errorf("effects.Supported()[%q] = false; cardaudit's main.go is missing the engine/rules blank import", key)
		}
	}
}
