package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Gideon's animate ability and its "prevent all damage that would be dealt to
// him this turn" Effect (Duration$ UntilHostLeavesPlayOrEOT over ValidTarget$
// Card.EffectSource), played decision by decision against gorge: the kernel's
// unbounded PreventNext shield on the source, ended by the turn or the source
// leaving the battlefield.
var kgideonShieldDecks = [2][]cardset.Entry{
	{
		{Name: "Gideon, Ally of Zendikar", Count: 4}, {Name: "Gideon, Martial Paragon", Count: 3},
		{Name: "Gideon, the Oathsworn", Count: 3}, {Name: "Gideon, Champion of Justice", Count: 3},
		{Name: "Savannah Lions", Count: 7}, {Name: "Plains", Count: 20},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 5},
		{Name: "Lightning Bolt", Count: 5}, {Name: "Mountain", Count: 18},
	},
}

func TestGideonShieldLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "gideonshield", kgideonShieldDecks, 200, "KGIDEONSHIELD_SEEDS")
}

// TestGideonShieldMutation: with the unbounded shield dropped, some seed
// diverges, so the animated Gideon's damage is prevented.
func TestGideonShieldMutation(t *testing.T) {
	t.Parallel()
	if lockstepRun(t, "gideonshield", kgideonShieldDecks, 200, false, dropExt(kop.ExtPreventNext, kop.PrevNextAll)) == 0 {
		t.Fatal("no seed diverged with Gideon's damage shield dropped")
	}
}
