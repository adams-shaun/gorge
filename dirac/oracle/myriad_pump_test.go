package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Pump-granted Myriad lockstep (CR 702.116a): "target creature gains
// myriad until end of turn" registers an end-of-turn keyword grant whose
// trigger gorge's granted walk synthesizes (a __kwMyriadGranted push) and the
// kernel lists as an EOT override's latent GrantedMyriad ability. Duke Ulder
// Ravengard and Mass of Mysteries grant it at the beginning of combat;
// Corporeal Projection is a sorcery (with Overload). Ironwill Forger's
// Lieutenant condition needs a commander, so in this commander-less match it
// only plays (its trigger never fires) and is not counted.
//
// A Pump's grant is registered with the pumped creature as its source, so a
// push names the holder, not the Pump. Each deck carries one granting Pump
// and nothing that changes control, so a push is counted for the Pump of the
// pushing seat's deck.

var (
	kmyriadPumpDuke = []cardset.Entry{
		{Name: "Duke Ulder Ravengard", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Raging Goblin", Count: 3},
		{Name: "Glory Seeker", Count: 3}, {Name: "Goblin Piker", Count: 3},
		{Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	}
	kmyriadPumpMass = []cardset.Entry{
		{Name: "Mass of Mysteries", Count: 4},
		{Name: "Fire Elemental", Count: 3}, {Name: "Earth Elemental", Count: 3},
		{Name: "Air Elemental", Count: 2}, {Name: "Water Elemental", Count: 2},
		{Name: "Birds of Paradise", Count: 3}, {Name: "Manalith", Count: 3},
		{Name: "City of Brass", Count: 4},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4},
		{Name: "Swamp", Count: 3}, {Name: "Forest", Count: 2},
	}
	kmyriadPumpProjection = []cardset.Entry{
		{Name: "Corporeal Projection", Count: 4}, {Name: "Ironwill Forger", Count: 2},
		{Name: "Coral Merfolk", Count: 3}, {Name: "Wind Drake", Count: 2},
		{Name: "Goblin Piker", Count: 3}, {Name: "Gray Ogre", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Glory Seeker", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 5},
	}
)

func TestMyriadPumpLockstep(t *testing.T) {
	t.Parallel()

	byGrantor := map[string]int{}
	games, stalls := 0, 0
	for _, m := range []struct {
		decks    [2][]cardset.Entry
		grantors [2]string
	}{
		{[2][]cardset.Entry{kmyriadPumpDuke, kmyriadPumpProjection}, [2]string{"Duke Ulder Ravengard", "Corporeal Projection"}},
		{[2][]cardset.Entry{kmyriadPumpMass, kmyriadPumpProjection}, [2]string{"Mass of Mysteries", "Corporeal Projection"}},
	} {
		g, s := runMyriadPool(t, "myriadpump", m.decks, 100, func(g *GorgeSide) {
			if g == nil {
				return
			}
			for _, ev := range g.E.L.Events {
				if ev.Kind == events.GrantTriggerPush && ev.Counter == "__kwMyriadGranted" && int(ev.Player) < len(m.grantors) {
					byGrantor[m.grantors[ev.Player]]++
				}
			}
		})
		if t.Failed() {
			return
		}
		games, stalls = games+g, stalls+s
	}
	t.Logf("%d games, %d stalls; Pump-granted Myriad instances by grantor: %v", games, stalls, byGrantor)
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, n := range []string{"Duke Ulder Ravengard", "Mass of Mysteries", "Corporeal Projection"} {
		if byGrantor[n] < kmyriadMin {
			t.Errorf("%s granted Myriad instances %d, want at least %d", n, byGrantor[n], kmyriadMin)
		}
	}
}
