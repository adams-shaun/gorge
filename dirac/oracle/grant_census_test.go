package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// tail-08's, on the same terms: the recording whose kernel dropped The
// World Tree's activation beside the lands its 6-land static granted
// "{T}: Add one mana of any color" -- the grant cap's search exhausted
// gorge's node budget over a cost the join census prices (the ability
// arm's, granted mana abilities included), so the offer fell one
// activation short of gorge's.
func TestTail08ReprosLockstep(t *testing.T) {
	t.Parallel()
	dirac.DebugGrantCensusPays = true
	defer func() { dirac.DebugGrantCensusPays = false }()
	stdReprosGlob(t, "tail-08")
}
