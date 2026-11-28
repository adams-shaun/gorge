package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestMain runs every oracle test with the manaPCGen memo's shadow check on
// (dirac/mana_pcmemo.go): each memo hit is recomputed and a stale view panics,
// so every lockstep game here also proves the cached path equals the uncached one
// (docs/design/dirac-static-source-cache.md). The quiet-seat proof's shadow
// runs the priority walk the proof would skip and panics when a proved
// window's walk offers more than pass (docs/design/dirac-quiet-seat-proof.md).
func TestMain(m *testing.M) {
	dirac.DebugPCMemoShadow = true
	dirac.DebugSFMemoShadow = true // scriptFilt's memo (dirac/mana_sfmemo.go)
	dirac.DebugQuietShadow = true  // the quiet-seat proof: every posed walk runs and a proved one must be pass alone (dirac/quiet_seat.go)
	os.Exit(m.Run())
}
