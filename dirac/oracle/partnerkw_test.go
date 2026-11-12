package oracle

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The printed-Partner lockstep (CR 702.124a/h/i): kgen lowers K:Partner and
// K:Partner:<text> as deck construction only, so the 40 corpus cards it
// refused for that keyword alone now play. Each duel is a Commander game
// seating a legal partner pair a seat (deck.IsCommanderPair, the check
// gorge's commander config and the dirac adapter apply) and other class cards
// in the library, over the generated partnerkw pool: every one of the 40 is
// cast and gorge and the kernel must agree decision by decision.

// kpkSeat is one seat: its commander pair (deck indices 0 and 1) and its
// 40-card library.
type kpkSeat struct {
	name  string
	cmdrs [2]string
	lib   []cardset.Entry
}

var kpkSeats = [...]kpkSeat{
	{"partner-thrasios-tana", [2]string{"Thrasios, Triton Hero", "Tana, the Bloodsower"}, []cardset.Entry{
		{Name: "Kydele, Chosen of Kruphix", Count: 3}, {Name: "Kraum, Ludevic's Opus", Count: 3},
		{Name: "Slurrk, All-Ingesting", Count: 3}, {Name: "Rograkh, Son of Rohgahh", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 4},
	}},
	{"partner-bruse-ravos", [2]string{"Bruse Tarl, Boorish Herder", "Ravos, Soultender"}, []cardset.Entry{
		{Name: "Akiri, Line-Slinger", Count: 3}, {Name: "Alharu, Solemn Ritualist", Count: 3},
		{Name: "Prava of the Steel Legion", Count: 3}, {Name: "Yoshimaru, Ever Faithful", Count: 3},
		{Name: "Ornithopter", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 4},
	}},
	{"partner-ishai-yoshimaru", [2]string{"Ishai, Ojutai Dragonspeaker", "Yoshimaru, Ever Faithful"}, []cardset.Entry{
		{Name: "April O'Neil, Live on the Scene", Count: 3}, {Name: "Leonardo, the Balance", Count: 3},
		{Name: "Alharu, Solemn Ritualist", Count: 2}, {Name: "Prava of the Steel Legion", Count: 2},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Ornithopter", Count: 2},
		{Name: "Plains", Count: 11}, {Name: "Island", Count: 9},
	}},
	{"partner-reyhan-miara", [2]string{"Reyhan, Last of the Abzan", "Miara, Thorn of the Glade"}, []cardset.Entry{
		{Name: "Slurrk, All-Ingesting", Count: 2}, {Name: "Keskit, the Flesh Sculptor", Count: 3},
		{Name: "Francisco, Fowl Marauder", Count: 3}, {Name: "Splinter, the Mentor", Count: 3},
		{Name: "Michelangelo, the Heart", Count: 3}, {Name: "Llanowar Elves", Count: 4}, {Name: "Typhoid Rats", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	}},
	{"select-leonardo-april", [2]string{"Leonardo, the Balance", "April O'Neil, Live on the Scene"}, []cardset.Entry{
		{Name: "Michelangelo, the Heart", Count: 3}, {Name: "Ishai, Ojutai Dragonspeaker", Count: 2},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 7},
	}},
	{"select-raphael-splinter", [2]string{"Raphael, the Muscle", "Splinter, the Mentor"}, []cardset.Entry{
		{Name: "Rograkh, Son of Rohgahh", Count: 3}, {Name: "Francisco, Fowl Marauder", Count: 3},
		{Name: "Keskit, the Flesh Sculptor", Count: 3}, {Name: "Gray Ogre", Count: 4}, {Name: "Typhoid Rats", Count: 4},
		{Name: "Ornithopter", Count: 3}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	}},
	{"friends-hopper-max", [2]string{"Chief Jim Hopper", "Max, the Daredevil"}, []cardset.Entry{
		{Name: "Mike, the Dungeon Master", Count: 3}, {Name: "Toggo, Goblin Weaponsmith", Count: 3},
		{Name: "Alena, Kessig Trapper", Count: 3}, {Name: "Keleth, Sunmane Familiar", Count: 3},
		{Name: "Anara, Wolvid Familiar", Count: 2}, {Name: "Ornithopter", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 5},
	}},
	{"friends-dustin-lucas", [2]string{"Dustin, Gadget Genius", "Lucas, the Sharpshooter"}, []cardset.Entry{
		{Name: "Esior, Wardwing Familiar", Count: 3}, {Name: "Siani, Eye of the Storm", Count: 3},
		{Name: "Brinelin, the Moon Kraken", Count: 2}, {Name: "Ornithopter", Count: 3}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Bronze Sable", Count: 6}, {Name: "Island", Count: 9}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
	}},
	{"partner-falthis-nadier", [2]string{"Falthis, Shadowcat Familiar", "Nadier, Agent of the Duskenel"}, []cardset.Entry{
		{Name: "Tormod, the Desecrator", Count: 3}, {Name: "Joel, Resolute Survivor", Count: 3},
		{Name: "Ich-Tekik, Salvage Splicer", Count: 3}, {Name: "Kamahl, Heart of Krosa", Count: 2},
		{Name: "Gravedigger", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Ornithopter", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 9},
	}},
	{"partner-esior-toggo", [2]string{"Esior, Wardwing Familiar", "Toggo, Goblin Weaponsmith"}, []cardset.Entry{
		{Name: "Brinelin, the Moon Kraken", Count: 2}, {Name: "Alena, Kessig Trapper", Count: 3},
		{Name: "Anara, Wolvid Familiar", Count: 3}, {Name: "Keleth, Sunmane Familiar", Count: 3},
		{Name: "Kamahl, Heart of Krosa", Count: 2}, {Name: "Coral Merfolk", Count: 4}, {Name: "Gray Ogre", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 5},
	}},
}

// kpkDuels are the seat pairs played: plain partner pairs against each other,
// the partner—Character select pairs, and the partner—Friends forever pairs.
var kpkDuels = [...][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {8, 9}}

// kpkLowered are the corpus cards kgen refused for the printed Partner
// keyword alone: the 22 the Commander precon and cEDH decks carry, then the
// 18 no such deck does.
var kpkLowered = []string{
	"Akiri, Line-Slinger", "Alharu, Solemn Ritualist", "April O'Neil, Live on the Scene",
	"Bruse Tarl, Boorish Herder", "Francisco, Fowl Marauder", "Ishai, Ojutai Dragonspeaker",
	"Keskit, the Flesh Sculptor", "Kraum, Ludevic's Opus", "Kydele, Chosen of Kruphix",
	"Leonardo, the Balance", "Miara, Thorn of the Glade", "Michelangelo, the Heart",
	"Prava of the Steel Legion", "Raphael, the Muscle", "Ravos, Soultender",
	"Reyhan, Last of the Abzan", "Rograkh, Son of Rohgahh", "Slurrk, All-Ingesting",
	"Splinter, the Mentor", "Tana, the Bloodsower", "Thrasios, Triton Hero",
	"Yoshimaru, Ever Faithful",

	"Alena, Kessig Trapper", "Anara, Wolvid Familiar", "Brinelin, the Moon Kraken",
	"Chief Jim Hopper", "Dustin, Gadget Genius", "Esior, Wardwing Familiar",
	"Falthis, Shadowcat Familiar", "Ich-Tekik, Salvage Splicer", "Joel, Resolute Survivor",
	"Kamahl, Heart of Krosa", "Keleth, Sunmane Familiar", "Lucas, the Sharpshooter",
	"Max, the Daredevil", "Mike, the Dungeon Master", "Nadier, Agent of the Duskenel",
	"Siani, Eye of the Storm", "Toggo, Goblin Weaponsmith", "Tormod, the Desecrator",
}

// kpkNoStackAbility are the class cards with no ability that uses the stack:
// statics (Akiri, Anara, Esior, Falthis), keywords (Rograkh) and the {T}
// mana abilities kpkManaAbility names.
var kpkNoStackAbility = map[string]bool{
	"Akiri, Line-Slinger": true, "Rograkh, Son of Rohgahh": true, "Anara, Wolvid Familiar": true,
	"Esior, Wardwing Familiar": true, "Falthis, Shadowcat Familiar": true,
	"Kydele, Chosen of Kruphix": true, "Alena, Kessig Trapper": true, "Dustin, Gadget Genius": true,
}

// kpkManaAbility are the class cards whose ability is a {T} mana ability
// (Kydele's {C} a card drawn, Alena's {R} by entered power, Dustin's
// artifact-only {C}{C}).
var kpkManaAbility = []string{"Kydele, Chosen of Kruphix", "Alena, Kessig Trapper", "Dustin, Gadget Genius"}

// kpkScan counts, in gorge's event log, each class card's casts, its
// abilities put on the stack (triggered or activated) and its mana
// activations: its tap with mana added at once.
func kpkScan(g *GorgeSide, class map[string]bool, casts, abil, mana map[string]int) {
	evs := g.E.L.Events
	for i, ev := range evs {
		ob := g.E.G.Obj(ev.Obj)
		if ob == nil || ob.Face() == nil || !class[ob.Face().Name] {
			continue
		}
		switch ev.Kind {
		case events.PutOnStack:
			if ev.From == state.ZHand || ev.From == state.ZCommand {
				casts[ob.Face().Name]++
			}
		case events.TriggerPush, events.AbilityPush:
			abil[ob.Face().Name]++
		case events.Tap:
			if i+1 < len(evs) && evs[i+1].Kind == events.ManaAdd && evs[i+1].Amount > 0 {
				mana[ob.Face().Name]++
			}
		}
	}
}

func TestPartnerKeywordLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("partnerkw")
	if err != nil {
		t.Fatal(err)
	}
	const seeds = 30 // a duel; Dustin's mana and Tormod's trigger are the rarest reached
	class := map[string]bool{}
	for _, n := range kpkLowered {
		class[n] = true
	}
	casts, abil, mana := map[string]int{}, map[string]int{}, map[string]int{}
	games, stalls := 0, 0
	for _, du := range kpkDuels {
		var decks [2][]*cards.Card
		var ents [2][]cardset.Entry
		var names [2]string
		for s, si := range du {
			seat := kpkSeats[si]
			names[s] = seat.name
			ents[s] = append([]cardset.Entry{{Name: seat.cmdrs[0], Count: 1}, {Name: seat.cmdrs[1], Count: 1}}, seat.lib...)
			for _, e := range ents[s] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 42 {
				t.Fatalf("%s: %d cards, want two commanders and a 40-card library", seat.name, len(decks[s]))
			}
			// the precondition: the seat's pair is one gorge's commander
			// config seats (an illegal pair would seat no commander)
			if !deck.IsCommanderPair(decks[s][0], decks[s][1]) {
				t.Fatalf("%s: %s and %s are not a legal commander pair", seat.name, seat.cmdrs[0], seat.cmdrs[1])
			}
		}
		tbl, _, _, err := set.Matchup(ents[0], ents[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, names, decks, 0)
			if err != nil {
				t.Fatal(err)
			}
			m.Commander = true
			m.Commanders, m.Partners = [2]int{0, 0}, [2]int{1, 1}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			for s := 0; s < 2; s++ {
				pl := &g.E.G.Players[s]
				if len(pl.Commanders) != 2 || g.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand || g.E.G.Obj(pl.Commanders[1]).Zone != state.ZCommand {
					t.Fatalf("%s seed %d: gorge seated %v, want both partners in the command zone", names[s], seed, pl.Commanders)
				}
			}
			res := LockstepLed(m, seed, g, k, kinnerlightPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
			CloseSide(k)
			games++
			kpkScan(g, class, casts, abil, mana)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Logf("%s v %s seed %d: %s", names[0], names[1], seed, res.Stall)
			}
			if d := res.Div; d != nil {
				t.Errorf("%s v %s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", names[0], names[1], seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	var rows []string
	for _, n := range kpkLowered {
		rows = append(rows, n+": "+strconv.Itoa(casts[n])+" casts, "+strconv.Itoa(abil[n])+" abilities, "+strconv.Itoa(mana[n])+" mana")
	}
	sort.Strings(rows)
	t.Logf("%d games, %d kernel stalls\n%s", games, stalls, strings.Join(rows, "\n"))
	if stalls != 0 {
		t.Fatalf("%d kernel stalls", stalls)
	}
	// the precondition the lockstep's agreement depends on: every card this
	// change lowers was cast in some game, and each one's triggered,
	// activated or mana ability was used, so it was played and not merely
	// carried
	for _, n := range kpkLowered {
		switch {
		case casts[n] == 0:
			t.Errorf("%s was never cast", n)
		case !kpkNoStackAbility[n] && abil[n] == 0:
			t.Errorf("%s: no ability of it went on the stack", n)
		}
	}
	for _, n := range kpkManaAbility {
		if mana[n] == 0 {
			t.Errorf("%s never tapped for mana", n)
		}
	}
}
