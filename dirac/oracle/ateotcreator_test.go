package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestAteotCreatorSourceMutation puts an AtEOT$ rider's delayed trigger on
// the stack with the affected token as its source (the earlier shape):
// gorge sources it on the creating permanent (CR 603.7d-e), so the
// projected Stack[].Source must diverge.
func TestAteotCreatorSourceMutation(t *testing.T) {
	old := dirac.DebugAteotSourceAffected
	dirac.DebugAteotSourceAffected = true
	defer func() { dirac.DebugAteotSourceAffected = old }()
	games, divs, tl := runTokenateot(t, 128, false)
	if tl.srcDivs == 0 {
		t.Fatalf("mutation (delayed trigger sourced on the affected object) ran %d games, %d divergences, none on Stack[].Source", games, divs)
	}
	t.Logf("mutation diverged in %d of %d games, %d on Stack[].Source", divs, games, tl.srcDivs)
}
