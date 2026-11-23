package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Benefactor's Draught and Battle Cry lockstep (dirac/kgen/lower_zone.go emblem
// over an end-of-turn Mode$ Blocks line; kernel trig_emblemblocks.go
// fireEmblemsBlocks): "whenever a creature an opponent controls blocks, draw
// a card" and "whenever a creature blocks this turn, it gets +0/+1". The deck
// is vanilla 2/2 blockers beside the two instants, so the emblems are live
// when blockers are declared.
func blocksEmblemDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Benefactor's Draught", Count: 6}, {Name: "Battle Cry", Count: 6}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
	}
	return [2][]cardset.Entry{a, append([]cardset.Entry(nil), a...)}
}

func TestBlocksEmblemLockstep(t *testing.T) {
	st := &dirac.EffEOTTrigStats{}
	dirac.DebugEffEOTTrigStats = st
	defer func() { dirac.DebugEffEOTTrigStats = nil }()
	lockstepRun(t, "blocksemblem", blocksEmblemDecks(), 300, true, nil)
	if st.BlocksFires == 0 {
		t.Fatal("no blocks emblem trigger was queued in the games; the lockstep proves nothing")
	}
	t.Logf("blocks emblem fires %d", st.BlocksFires)
}

// TestBlocksEmblemLockstepMutation: with the blocks emblem hook off, some seed
// diverges (gorge draws and pumps; the kernel would not).
func TestBlocksEmblemLockstepMutation(t *testing.T) {
	dirac.DebugIgnoreBlocksEmblem = true
	defer func() { dirac.DebugIgnoreBlocksEmblem = false }()
	if lockstepRun(t, "blocksemblem", blocksEmblemDecks(), 300, false, nil) == 0 {
		t.Fatal("ignoring the blocks emblem left the lockstep clean")
	}
}
