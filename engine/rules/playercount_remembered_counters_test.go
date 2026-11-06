package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPhyresisOutbreakCountHeadIsRead is the real-corpus pin for
// PlayerCountRemembered$Counters: the compiled face must carry the head and
// the registry must stop reporting it unsupported.
func TestPhyresisOutbreakCountHeadIsRead(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	head := cards.ValueHeadPrefix + "PlayerCountRemembered$Counters"

	var carrier *cards.Card
	n := 0
	for _, c := range reg.AllCards() {
		if len(c.Faces) == 0 || c.Faces[0].Name != "Phyresis Outbreak" {
			continue
		}
		carrier = c
		n++
	}
	if n != 1 || carrier == nil {
		t.Fatalf("corpus has %d cards named Phyresis Outbreak, want exactly 1", n)
	}
	if !slices.Contains(carrier.ValueHeads(), head) {
		t.Fatalf("test precondition: Phyresis Outbreak ValueHeads() = %v, want it to contain %q",
			carrier.ValueHeads(), head)
	}

	miss := reg.Unsupported(carrier, effects.Supported())
	if slices.Contains(miss, head) {
		t.Fatalf("Phyresis Outbreak still reports %q unsupported (missing %v)", head, miss)
	}
}
