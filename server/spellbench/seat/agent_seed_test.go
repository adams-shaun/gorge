package seat

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

func TestAgentSeedFitsStrictJSONInteger(t *testing.T) {
	over := 0
	for i := 0; i < 256; i++ {
		s := agentSeed([]byte("secret"), fmt.Sprintf("g-%d", i), "p0")
		if s > uint64(wire.MaxInt) {
			over++
		}
	}
	if over != 0 {
		t.Fatalf("%d of 256 agent seeds exceed wire.MaxInt (2^53-1)", over)
	}
}
