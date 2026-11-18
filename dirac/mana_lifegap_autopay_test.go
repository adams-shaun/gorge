package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func TestLifeGapAutopayPlainCastPriced(t *testing.T) {
	deck0 := []cardset.Entry{{Name: "Angel of Vitality", Count: 1}, {Name: "Angel of the Ruins", Count: 1}, {Name: "Curse of Opulence", Count: 1}, {Name: "Dauntless Bodyguard", Count: 1}, {Name: "Elite Scaleguard", Count: 1}, {Name: "Fellwar Stone", Count: 1}, {Name: "Ganax, Astral Hunter", Count: 1}, {Name: "Gideon's Lawkeeper", Count: 1}, {Name: "Goblin Recruiter", Count: 1}, {Name: "Lunar Frenzy", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Pacifism", Count: 1}, {Name: "Patriar's Seal", Count: 1}, {Name: "Peat Bog", Count: 1}, {Name: "Pilgrim of the Ages", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Power Matrix", Count: 1}, {Name: "Reya Dawnbringer", Count: 1}, {Name: "Soul Warden", Count: 1}, {Name: "Spiteful Prankster", Count: 1}, {Name: "Touch the Spirit Realm", Count: 2}, {Name: "Treasonous Ogre", Count: 2}, {Name: "Wispweaver Angel", Count: 1}}
	deck1 := []cardset.Entry{{Name: "Abundant Growth", Count: 2}, {Name: "Act on Impulse", Count: 1}, {Name: "Brood Monitor", Count: 1}, {Name: "Crimson Fleet Commodore", Count: 1}, {Name: "Crop Rotation", Count: 1}, {Name: "Falter", Count: 1}, {Name: "Fellwar Stone", Count: 1}, {Name: "Fog", Count: 1}, {Name: "Footfall Crater", Count: 1}, {Name: "Forest", Count: 8}, {Name: "Ganax, Astral Hunter", Count: 1}, {Name: "Goblin Recruiter", Count: 1}, {Name: "Grab the Reins", Count: 1}, {Name: "Lunar Frenzy", Count: 1}, {Name: "Manabond", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Noose Constrictor", Count: 2}, {Name: "Patriar's Seal", Count: 1}, {Name: "Peat Bog", Count: 1}, {Name: "Quakefoot Cyclops", Count: 1}, {Name: "Strangleroot Geist", Count: 1}, {Name: "Sudden Spinnerets", Count: 1}, {Name: "Treasonous Ogre", Count: 2}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck0, deck1)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true})
	var ogre Obj
	for _, n := range []string{"Plains", "Mountain", "Goblin Recruiter", "Plains", "Plains", "Treasonous Ogre"} {
		ogre = sc.perm(id[n], 0)
	}
	g := sc.hand(id["Ganax, Astral Hunter"], 0)
	x := sc.hand(id["Lunar Frenzy"], 0)
	offered := func(o Obj) bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) {
				return true
			}
		}
		return false
	}
	if !lifeOnlyPain(sc.s.manaPCAt(sc.k, sc.slot(ogre))) {
		t.Fatal("precondition: the Ogre is not the life-only pain census unit")
	}
	if !offered(g) {
		t.Fatal("the {4}{R} cast four lands and the Ogre's life pay is not offered: gorge's planner prices it with the Ogre's last-resort unit")
	}
	if offered(x) {
		t.Fatal("the {X}{R} cast is offered: gorge's planner never prices an {X} cost, so the autopay surface drops it")
	}
	sc.s.pl[0].life = 3
	if offered(g) {
		t.Fatal("the cast is offered at 3 life: the planner refuses a unit whose pain is at least the payer's life")
	}
}
