package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmFraIslandFlags replays std-fra pair 153 (seed
// 13704788432565239360, autopay, mulligans 0), the game whose projection
// first parted on Cards[14].Flags after Void Extrapolator's prepared copy was
// cast: gorge paid the copy's hybrid {U/B} pip with Chandra, Chill of
// Compliance's floating {U} (restricted to noncreature spells), while the
// kernel tapped an Island for the pip and left Chandra's mana floating. The
// pin is the evhunt run's recorded repro, so it reaches the prepared copy and
// the restricted-mana spend that pays it.
func TestPinnedFarmFraIslandFlags(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-fra", rc, 0,
		onBattlefield("Void Extrapolator"), preparedCopySpentRestrictedMana)
}

// preparedCopySpentRestrictedMana is the condition that a prepared copy's cast
// spent mana carrying a spend restriction: a restricted ManaAdd with a negative
// amount in the same cast as the prepared copy's CastInfo (gorge emits the
// spend before the cast commits). It is the interaction the pin exists for.
func preparedCopySpentRestrictedMana(ref *GorgeSide) bool {
	spent := false
	evs := ref.E.L.Events
	for i := range evs {
		e := &evs[i]
		switch e.Kind {
		case events.ManaAdd:
			if e.Amount < 0 && strings.HasPrefix(e.Text, "mana-restriction:") {
				spent = true
			}
		case events.CastInfo:
			if spent && events.FlagsFrom(e.Counter)&state.FlagPreparedCopy != 0 {
				return true
			}
			spent = false
		}
	}
	return false
}
