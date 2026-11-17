package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Condition$ AttackedPlayerWithMostLife on Scourge of the Throne's attack
// trigger: gorge fires it only when the attacked player has the most life or
// is tied (trigmatch.PlayerHasMostLife, the Dethrone gate). dirac once read the
// condition as ignored and queued the untap trigger on every first attack, so
// the trigger_order ask offered a third order the gorge side never posed
// (decision 744 of tail-13-c000 pair 4).
func TestMBCScourgeOfTheThroneReprosLockstep(t *testing.T) {
	t.Parallel()
	// The corpus probe comes first and needs no git: a source archive with
	// no .cards (pr-precheck's fresh clone) must skip, and synthReg's
	// repository-root lookup fails there instead.
	if _, err := cardset.CorpusDir(); errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip("no card corpus (.cards): " + err.Error())
	}
	stdReproNamed(t, "attack_most_life_condition")
}
