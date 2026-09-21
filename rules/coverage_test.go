package rules

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/effects"
)

// TestNonAPIPrimitivesAreRegistered pins Ruling W1's premise: that the
// keyword, static and trigger families this build implements
// (rules/combat.go, rules/statics.go, rules/trigger.go, each an init()
// calling effects.RegisterNonAPI) actually register themselves. This test
// runs inside package rules's own test binary, where those three files are
// necessarily compiled in and their init()s have necessarily already run
// before main (or TestMain) ever starts -- so on its own it proves only
// that the registrations exist, not that every consumer of
// effects.Supported() sees them.
//
// That gap is exactly Ruling W1's bug: cmd/forgec's report command imports
// cards and effects but, before this task, never imported rules, so none of
// these three init()s ran in that separate binary and its
// effects.Supported() returned "api:" primitives only -- understating
// coverage by thousands of cards (trig:ChangesZone alone gated 7008 of
// them). Proving the forgec binary itself is fixed needs a check that does
// not get to piggy-back on package rules already being loaded, which is
// TestForgecBinaryImportsRules below -- a static check of forgec's own
// import graph, not a second copy of this runtime assertion.
func TestNonAPIPrimitivesAreRegistered(t *testing.T) {
	supported := effects.Supported()
	var haveKw, haveStat, haveTrig bool
	for k := range supported {
		switch {
		case strings.HasPrefix(k, "kw:"):
			haveKw = true
		case strings.HasPrefix(k, "stat:"):
			haveStat = true
		case strings.HasPrefix(k, "trig:"):
			haveTrig = true
		}
	}
	if !haveKw {
		t.Error(`effects.Supported() has no "kw:" entry -- rules/combat.go's init no longer registers a keyword?`)
	}
	if !haveStat {
		t.Error(`effects.Supported() has no "stat:" entry -- rules/statics.go's init no longer registers a static?`)
	}
	if !haveTrig {
		t.Error(`effects.Supported() has no "trig:" entry -- rules/trigger.go's init no longer registers a trigger?`)
	}
}

// TestNumLoyaltyActPrimitiveIsRegistered pins the exact support declaration
// used by cards.Registry.Unsupported and deck validation. The rules engine
// implements this static in loyaltyAbilityLimit; omitting it here would still
// reject Oath of Teferi as missing stat:NumLoyaltyAct.
func TestNumLoyaltyActPrimitiveIsRegistered(t *testing.T) {
	if !effects.Supported()["stat:NumLoyaltyAct"] {
		t.Fatal(`effects.Supported() is missing "stat:NumLoyaltyAct"`)
	}
}

// TestTokenReplacementPrimitivesAreRegistered pins the support declaration
// for the token-creation replacement class (Divine Visitation, Doubling
// Season, Academy Manufactor, Xorn, ...): cards' census derives
// repl:CreateToken from the R: line and api:ReplaceToken from the
// ReplaceWith$ body, and rules/replacement.go's init registers both now
// that replacementMatchesRemembered's CreateToken case and
// continueCreateTokenReplacements implement the class. Omitting either
// would leave all 35 corpus carrier cards unplayable.
func TestTokenReplacementPrimitivesAreRegistered(t *testing.T) {
	supported := effects.Supported()
	if !supported["repl:CreateToken"] {
		t.Fatal(`effects.Supported() is missing "repl:CreateToken"`)
	}
	if !supported["api:ReplaceToken"] {
		t.Fatal(`effects.Supported() is missing "api:ReplaceToken"`)
	}
}

// TestAddCounterReplacementPrimitivesAreRegistered pins the support
// declaration for the counter-placement replacement class (Hardened Scales,
// Branching Evolution, Doubling Season, Vorinclex, ...): cards' census
// derives repl:AddCounter from the R: line and api:ReplaceCounter from the
// ReplaceWith$ body, and rules/replacement.go's init registers both now that
// replacementMatchesRemembered's AddCounter case and
// applyAddCounterReplacements implement the class. Omitting either would
// leave all 30 corpus carrier cards unplayable.
func TestAddCounterReplacementPrimitivesAreRegistered(t *testing.T) {
	supported := effects.Supported()
	if !supported["repl:AddCounter"] {
		t.Fatal(`effects.Supported() is missing "repl:AddCounter"`)
	}
	if !supported["api:ReplaceCounter"] {
		t.Fatal(`effects.Supported() is missing "api:ReplaceCounter"`)
	}
}

// TestForgecBinaryImportsRules is the second assertion path: a static check,
// independent of anything already loaded into this test binary, that
// cmd/forgec's own dependency graph includes package rules. `go list -deps`
// reports the fully-resolved import set exactly as the go tool would build
// it, so a pass here means the forgec binary -- not just this test's
// process -- links in rules/combat.go, rules/statics.go and
// rules/trigger.go and therefore runs their init()s before its own report
// command ever calls effects.Supported(). This is what the blank import
// `_ "github.com/adams-shaun/gorge/rules"` in cmd/forgec/main.go (Ruling
// W1) actually fixes; skip only if the go tool itself is unavailable to run
// this check with, not if the corpus is absent -- this test needs neither
// .cards/ nor a build, only the module's own source tree.
func TestForgecBinaryImportsRules(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/adams-shaun/gorge/cmd/forgec").Output()
	if err != nil {
		if _, lookErr := exec.LookPath("go"); lookErr != nil {
			t.Skip("go tool not available to run `go list -deps`")
		}
		t.Fatalf("go list -deps github.com/adams-shaun/gorge/cmd/forgec: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "github.com/adams-shaun/gorge/rules" {
			return
		}
	}
	t.Fatal("cmd/forgec does not import github.com/adams-shaun/gorge/rules -- " +
		"its effects.Supported() call in report() sees API primitives only " +
		`(Ruling W1); add a blank import "_ \"github.com/adams-shaun/gorge/rules\"" to cmd/forgec/main.go`)
}
