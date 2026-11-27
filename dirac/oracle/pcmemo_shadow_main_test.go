package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestMain runs every oracle test with the manaPCGen memo's shadow check on
// (dirac/mana_pcmemo.go): each memo hit is recomputed and a stale view panics,
// so every lockstep game here also proves the cached path equals the uncached one
// (docs/design/dirac-static-source-cache.md).
func TestMain(m *testing.M) {
	dirac.DebugPCMemoShadow = true
	dirac.DebugSFMemoShadow = true // scriptFilt's memo (dirac/mana_sfmemo.go)
	os.Exit(m.Run())
}
