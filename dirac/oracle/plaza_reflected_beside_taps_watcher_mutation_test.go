package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestPlazaReflectedBesideTapsWatcherMutation: with the reflected census unit
// dropped, a Taps-deferred Plaza of Heroes is deferred whole again and the
// recording diverges, proof it exercises dirac.deferredReflMask.
func TestPlazaReflectedBesideTapsWatcherMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "plaza_reflected_beside_taps_watcher.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugNoDeferredRefl = true
	defer func() { dirac.DebugNoDeferredRefl = false }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the recording replayed clean with the reflected census unit dropped: it does not exercise deferredReflMask")
	}
}
