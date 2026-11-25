package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestMBCDragonScarredBearReprosLockstepMutation: with the reflected source
// classed by the Taps watchers again and the reflected census unit dropped, the
// recording diverges, proof it exercises dirac.reflTapsUnwatched and
// deferredReflMask together.
func TestMBCDragonScarredBearReprosLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "taps_deferred_reflected_source.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugNoReflTapsUnwatched = true
	dirac.DebugNoDeferredRefl = true
	defer func() {
		dirac.DebugNoReflTapsUnwatched = false
		dirac.DebugNoDeferredRefl = false
	}()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the recording replayed clean with the reflected source deferred: it does not exercise reflTapsUnwatched and deferredReflMask")
	}
}
