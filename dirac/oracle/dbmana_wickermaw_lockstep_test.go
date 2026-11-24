package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Foraging Wickermaw's "{1}: Add one mana of any color. This creature becomes
// that color until end of turn. Activate only once each turn." is a CR 605.1a
// mana ability gorge resolves off the stack. This fixture is a no-divergence
// guard over the std-ecl pool, not a rider test: gorge's cast/unless offer
// gate does not price the planner-deferred chosen-colour source for a play,
// so none of these seeds reaches Wickermaw's {1} activation (a mutation that
// drops the Animate rider still leaves them clean). The activation's kernel
// shape is pinned by dirac's TestWickermawChosenAnimateRider, whose table
// mirrors the kgen-lowered card that TestDbManaWickermawLowers pins.
var wickermawDecks = [2][]cardset.Entry{
	{{Name: "Foraging Wickermaw", Count: 4}, {Name: "Glamer Gifter", Count: 4}, {Name: "Forest", Count: 32}},
	{{Name: "Foraging Wickermaw", Count: 4}, {Name: "Glamer Gifter", Count: 4}, {Name: "Forest", Count: 32}},
}

// TestDbManaWickermawLockstep: four seeds on the std-ecl synthetic pool,
// alternating the gorge autopay surface (fixtureLockstep's seed parity), 0
// divergent. It guards the pool against a lowering regression; it does not
// exercise the rider (see the note above).
func TestDbManaWickermawLockstep(t *testing.T) {
	t.Parallel()
	fixtureLockstep(t, "std-ecl", wickermawDecks, 4)
}
