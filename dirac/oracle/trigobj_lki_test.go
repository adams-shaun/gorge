package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestTrigObjLKIMutation: read live, the creature that died attacking is a
// card in the graveyard that attacks nothing, so Garna, Bloodfist of Keld
// deals damage where gorge draws, and the repro diverges.
func TestTrigObjLKIMutation(t *testing.T) {
	dirac.DebugTrigLKIOff = true
	defer func() { dirac.DebugTrigLKIOff = false }()
	res := stdReproPlay(t, filepath.Join("testdata", "repro", "dies_trigger_reads_attacking_lki.repro.json"), "std-fdn")
	if res.Div == nil {
		t.Fatal("DebugTrigLKIOff: the Garna repro did not diverge")
	}
}
