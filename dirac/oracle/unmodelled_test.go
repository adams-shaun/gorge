package oracle

import (
	"fmt"
	"strings"
	"testing"
)

// unmodelledKnown names the fixture tests that still reach an unmodelled stall, by
// test and stall. This list only shrinks: maxUnmodelledKnown holds the count down.
// Anything else that reaches an unmodelled stall in lockstepRun or sonRunStalls
// fails.
var unmodelledKnown = map[string]bool{
	"TestAdversaryLockstep|kernel numtimes": true,
	"TestFiredmgLockstep|kernel replplus":   true,
	"TestFiredmgMutation|kernel replplus":   true,
	"TestOjerLockstep|kernel replplus":      true,
}

// maxUnmodelledKnown is the list's size. Lower it with the list; never raise it.
const maxUnmodelledKnown = 4

// unmodelledStallError is the failure of test (its top-level name) reaching res's
// stall, nil when the stall is a capacity, or there is none, or the list names it.
func unmodelledStallError(test string, res *Result) error {
	if !res.StallUnmodelled {
		return nil
	}
	top, _, _ := strings.Cut(test, "/")
	if unmodelledKnown[top+"|"+res.Stall] {
		return nil
	}
	return fmt.Errorf("%s is an unmodelled stall, not a declared capacity: fix it, refuse the card, or (only to remove soon) name %q in unmodelledKnown", res.Stall, top+"|"+res.Stall)
}

// TestUnmodelledListOnlyShrinks: raising the bound takes a visible edit here.
func TestUnmodelledListOnlyShrinks(t *testing.T) {
	if len(unmodelledKnown) > maxUnmodelledKnown {
		t.Errorf("%d entries, at most %d: this list only shrinks", len(unmodelledKnown), maxUnmodelledKnown)
	}
	if len(unmodelledKnown) < maxUnmodelledKnown {
		t.Errorf("%d entries: lower maxUnmodelledKnown to match", len(unmodelledKnown))
	}
}

// TestUnmodelledStallFailsLockstepRun is the rule lockstepRun and sonRunStalls
// apply: an unmodelled stall fails, a capacity one does not, and a list entry only
// excuses its own test and its own reason.
func TestUnmodelledStallFailsLockstepRun(t *testing.T) {
	gap := &Result{Stall: KernelStallPrefix + "replleave", StallUnmodelled: true}
	if err := unmodelledStallError("TestAnything", gap); err == nil || !strings.Contains(err.Error(), "replleave") {
		t.Errorf("an unmodelled stall: %v", err)
	}
	limit := &Result{Stall: KernelStallPrefix + "board"}
	if err := unmodelledStallError("TestAnything", limit); err != nil {
		t.Errorf("a capacity stall: %v", err)
	}
	if err := unmodelledStallError("TestAnything", &Result{Stall: "turns"}); err != nil {
		t.Errorf("a turn cap: %v", err)
	}
	listed := &Result{Stall: KernelStallPrefix + "replplus", StallUnmodelled: true}
	if err := unmodelledStallError("TestOjerLockstep/sub", listed); err != nil {
		t.Errorf("a listed test and reason: %v", err)
	}
	if err := unmodelledStallError("TestOjerLockstep", gap); err == nil {
		t.Error("a listed test excused a reason it does not name")
	}
	if err := unmodelledStallError("TestSomethingElse", listed); err == nil {
		t.Error("a listed reason excused a test it does not name")
	}
}

// unmodelledAfter is stallAfter with a reason of the unmodelled class.
type unmodelledAfter struct {
	stallAfter
	unmodelled bool
}

func (s *unmodelledAfter) CapacityStall() string {
	if s.n <= 0 {
		return "replleave"
	}
	return ""
}

func (s *unmodelledAfter) StallIsUnmodelled() bool { return s.unmodelled }

// A stall reaches Result with its class, and Clean follows the class: a capacity
// stall is still evidence, an unmodelled one is not.
func TestLockstepCarriesTheStallClass(t *testing.T) {
	t.Parallel()
	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 20261017)[0]
	for _, unmodelled := range []bool{false, true} {
		m, err := c.Matchup(p.A, p.B, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := GorgeKernel(Config{})(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, &unmodelledAfter{stallAfter: stallAfter{Side: k, n: 20}, unmodelled: unmodelled}, RandomPolicy(p.Seed), RunConfig{})
		if res.Div != nil {
			t.Fatalf("a stall diverged: %v", res.Div)
		}
		if res.Stall != KernelStallPrefix+"replleave" || res.StallUnmodelled != unmodelled {
			t.Errorf("unmodelled=%v: stall %q, flag %v", unmodelled, res.Stall, res.StallUnmodelled)
		}
		if res.Clean() == unmodelled {
			t.Errorf("unmodelled=%v: Clean() = %v", unmodelled, res.Clean())
		}
	}
}
