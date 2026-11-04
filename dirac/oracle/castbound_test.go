package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Nyota Uhura, Communications chooses a number from 0 to 10 as it enters
// (CR 614.12) and draws when an opponent casts a spell of that mana value: the
// opponent's deck casts spells of mana value 1, 2 and 3 and an X spell, and
// carries its own Nyota, so each side's trigger reads its own source's number.
func TestEnterNumberSpellCastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{{Name: "Nyota Uhura, Communications", Count: 10}, {Name: "Open Communications", Count: 4}, {Name: "Island", Count: 26}}
	b := []cardset.Entry{
		{Name: "Planetary Bombardment", Count: 4}, {Name: "Boarding Action", Count: 4}, {Name: "Plasma Cascade", Count: 4},
		{Name: "Khaaaaaaaaaaaannn!", Count: 4}, {Name: "Nyota Uhura, Communications", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Island", Count: 6},
	}
	fixtureLockstep(t, "std-trk", [2][]cardset.Entry{a, b}, 300)
}

// The castbound fixture: Chalice of the Void counters a spell whose mana value
// equals its charge counters, Obscura Ascendancy triggers on its quest
// counters, Nyota on its chosen number, Talion on its chosen number against a
// spell's mana value, power or toughness (a noncreature spell has neither, CR
// 208.3). gorge reads each bound with the
// trigger's source as the source; so does the kernel (trig_castbound.go).
var castBoundDecks = []fixtureDecks{
	{
		{{Name: "Chalice of the Void", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 20}},
		{{Name: "Nyota Uhura, Communications", Count: 6}, {Name: "Ornithopter", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Divination", Count: 6}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 8}},
	},
	{
		{{Name: "Obscura Ascendancy", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8}},
		{{Name: "Chalice of the Void", Count: 6}, {Name: "Nyota Uhura, Communications", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 12}},
	},
	{
		{{Name: "Talion, the Kindly Lord", Count: 8}, {Name: "Divination", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Island", Count: 12}, {Name: "Swamp", Count: 12}},
		{{Name: "Grizzly Bears", Count: 2}, {Name: "Earth Elemental", Count: 4}, {Name: "Wall of Stone", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Ornithopter", Count: 2}, {Name: "Forest", Count: 2}, {Name: "Mountain", Count: 18}},
	},
}

var castBoundWatch = map[string]bool{"Chalice of the Void": true, "Obscura Ascendancy": true, "Nyota Uhura, Communications": true, "Talion, the Kindly Lord": true}

func TestCastBoundFixtureLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "castbound", castBoundDecks, castBoundWatch, 300, true, nil)
	t.Logf("%d games, %d divergent, %d stalls; casts %v", run.games, run.divs, run.stalls, run.casts)
	for n := range castBoundWatch {
		if run.casts[n] == 0 {
			t.Errorf("%s never cast", n)
		}
	}
}
