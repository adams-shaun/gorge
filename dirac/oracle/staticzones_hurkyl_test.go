package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Hurkyl's Final Meditation mirror: the card's own "as long as it's not your
// turn, this spell costs {3} more" (an Affected$ Card.Self RaiseCost with
// Condition$ NotPlayerTurn, EffectZone$ All). Seven Islands pay it on its
// controller's turn only, so a kernel that taxed it differently from gorge
// offers a cast gorge does not. Both hands hold Hurkyls, so a gorge that read
// the raise as every spell's (the script has no ValidCard$) would tax a
// Hurkyl cast on its controller's turn by the opponent's hand copy.
var kstaticzonesHurkylDecks = [2][]cardset.Entry{
	{{Name: "Hurkyl's Final Meditation", Count: 4}, {Name: "Island", Count: 36}},
	{{Name: "Hurkyl's Final Meditation", Count: 4}, {Name: "Island", Count: 36}},
}

func TestStaticZonesHurkylLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kstaticzonesHurkylDecks, 80, "KSTATICZONES_SEEDS")
}
