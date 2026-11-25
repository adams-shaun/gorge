package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Master Biomancer: "Each other creature you control enters with a number of
// additional +1/+1 counters on it equal to Master Biomancer's power" (CR
// 614.12). kgen writes Count$CardPower as VSelfPower, a direct self read the
// others-enter source override did not cover (Arwen's toughness rides VExpr):
// Chief Engineer (power 1) entered under a 2-power Biomancer with 1 counter in
// the kernel and 2 in gorge. Recorded as tail-14 pair 2 (surface autopay).
func TestOthersEnterCountersSourcePowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "others_enter_counters_source_power")
}

// TestOthersEnterCountersSourcePowerMutation makes the kernel read the
// entering creature's power again: the recorded game diverges.
func TestOthersEnterCountersSourcePowerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "others_enter_counters_source_power.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugOthersEnterEntrantValue = true
	defer func() { dirac.DebugOthersEnterEntrantValue = false }()
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("the mutation (entrant's power) replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}
