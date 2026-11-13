package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (gorge, CR 106.6, 601.2g-h, 602.2b):
// a restricted mana source is a payment planner unit for exactly the
// payments its RestrictValid$ admits. Constructed matchups over the
// generated restrictedsource pool:
//
//   - the five BLB Villages (Spell.Creature {W}/{U}/{B}/{G}/{R} beside {C})
//     against creature and noncreature spells of each colour;
//   - Giada (Spell.Angel) with Angels and non-Angels;
//   - Secluded Courtyard (Spell.Creature+ChosenType, Activated.Creature+
//     ChosenType beside {C}) with Goblins that have activated abilities;
//   - Helga (Amount$ X, Spell.Creature+cmcGE4, Spell.Creature+hasXCost)
//     with big creatures and Walking Ballista;
//   - Kolvori's back face, The Ringhart Crest (Spell.Creature+ChosenType,
//     Spell.Creature+Legendary);
//   - Thran Spider's Powerstones (CantCastNonArtifactSpells) with
//     artifacts and artifact activations.
//
// The mechanism is counted from gorge's log: a restricted ManaAdd whose
// batch the same payment spends, classified by its source and by what the
// payment pushed (a spell or an ability).

var restrSourceDecks = [][2][]cardset.Entry{{
	// the review probe: Ringhart Crest face casts beside Courtyard activations
	{
		{Name: "Kolvori, God of Kinship", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Forest", Count: 20}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Secluded Courtyard", Count: 4}, {Name: "Mogg Fanatic", Count: 4}, {Name: "Furnace Whelp", Count: 4},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4},
	},
}, {
	// the Villages against noncreature spells of their colours
	{
		{Name: "Lupinflower Village", Count: 3}, {Name: "Lilypad Village", Count: 3}, {Name: "Mudflat Village", Count: 3},
		{Name: "Oakhollow Village", Count: 3}, {Name: "Rockface Village", Count: 3},
		{Name: "Plains", Count: 2}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 1}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Wind Drake", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Centaur Courser", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Divination", Count: 2},
	},
	{
		{Name: "Kolvori, God of Kinship", Count: 3}, {Name: "Secluded Courtyard", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 3},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Furnace Whelp", Count: 3}, {Name: "Goblin Chieftain", Count: 3},
		{Name: "Mogg Fanatic", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Pacifism", Count: 2},
	},
}, {
	// Giada and Angels against Helga's big and {X} creatures
	{
		{Name: "Giada, Font of Hope", Count: 4}, {Name: "Serra Angel", Count: 3}, {Name: "Vanguard Seraph", Count: 3},
		{Name: "Dazzling Angel", Count: 3}, {Name: "Savannah Lions", Count: 2}, {Name: "Pacifism", Count: 2},
		{Name: "Wind Drake", Count: 2}, {Name: "Plains", Count: 18}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Helga, Skittish Seer", Count: 4}, {Name: "Craw Wurm", Count: 3}, {Name: "Shivan Dragon", Count: 2},
		{Name: "Hill Giant", Count: 3}, {Name: "Walking Ballista", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Elvish Mystic", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 2},
	},
}, {
	// Powerstones (both seats get one per Thran Spider) with artifacts and
	// artifact activations, against Courtyard Goblins
	{
		{Name: "Thran Spider", Count: 4}, {Name: "Mind Stone", Count: 3}, {Name: "Wayfarer's Bauble", Count: 3},
		{Name: "Guardian Idol", Count: 2}, {Name: "Walking Ballista", Count: 3}, {Name: "Ornithopter", Count: 2},
		{Name: "Pestilent Cauldron", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Centaur Courser", Count: 2},
		{Name: "Forest", Count: 16},
	},
	{
		{Name: "Thran Spider", Count: 3}, {Name: "Secluded Courtyard", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Goblin Chieftain", Count: 3}, {Name: "Mogg Fanatic", Count: 3}, {Name: "Mind Stone", Count: 2},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Divination", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 8},
	},
}, {
	// Secluded Courtyard as the main colour source: its restricted colour
	// funds Wizard, Goblin and Dragon casts and Furnace Whelp's {R} pump;
	// Helga against it
	{
		{Name: "Secluded Courtyard", Count: 12}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 2},
		{Name: "Furnace Whelp", Count: 6}, {Name: "Prodigal Sorcerer", Count: 6}, {Name: "Goblin Chieftain", Count: 4},
		{Name: "Mogg Fanatic", Count: 4}, {Name: "Shock", Count: 2},
	},
	{
		{Name: "Helga, Skittish Seer", Count: 6}, {Name: "Craw Wurm", Count: 4}, {Name: "Shivan Dragon", Count: 3},
		{Name: "Walking Ballista", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
	},
}}

// restrictedsource mechanism counters (restrSourceScan).
const (
	restrSourceFixedCast = iota // a fixed spell restriction (a Village, Giada, the Crest) funds a cast
	restrSourceCourtCast        // Secluded Courtyard's restricted colour funds a cast
	restrSourceCourtAct         // ...an activation
	restrSourceStoneAct         // a Powerstone funds an activation
	restrSourceStoneCast        // ...an artifact cast
	restrSourceHelgaCast        // Helga funds a cast
	restrSourceWithheld         // a priority decision where a hand card affordable only with the actor's restricted-only sources is not offered
	restrSourceN
)

var restrSourceNames = [restrSourceN]string{"fixed restriction funds a cast", "Courtyard colour funds a cast", "Courtyard funds an activation",
	"Powerstone funds an activation", "Powerstone funds an artifact cast", "Helga funds a cast", "play refused with a restricted source withheld"}

const restrSourceMinFires = 10

// restrSourceScan walks gorge's log: a restricted positive ManaAdd names its
// source (the " @id" suffix). gorge pushes the spell or ability first and
// pays after (CR 601.2a, 602.2a), so the mana funds the push made since the
// last decision; mana floated before any push (a manual activation) funds
// the next one.
func restrSourceScan(g *GorgeSide, n *[restrSourceN]int) {
	if g == nil {
		return
	}
	var pending []state.ObjID // restricted sources whose mana is waiting for a push
	var cur *events.Event     // the push of the current decision
	credit := func(push *events.Event, src state.ObjID) {
		spell := push.Kind == events.PutOnStack
		art := false
		if ob := g.E.G.Obj(push.Obj); ob != nil && ob.Face() != nil {
			for _, t := range ob.Face().Types {
				art = art || t == "Artifact"
			}
		}
		name := ""
		if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		switch {
		case name == "Secluded Courtyard" && spell:
			n[restrSourceCourtCast]++
		case name == "Secluded Courtyard":
			n[restrSourceCourtAct]++
		case name == "Powerstone Token" && spell && art:
			n[restrSourceStoneCast]++
		case name == "Powerstone Token" && !spell:
			n[restrSourceStoneAct]++
		case name == "Helga, Skittish Seer" && spell:
			n[restrSourceHelgaCast]++
		case spell && name != "":
			n[restrSourceFixedCast]++
		}
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.DecisionMade:
			cur = nil
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
				if cur != nil {
					credit(cur, src)
				} else {
					pending = append(pending, src)
				}
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack, events.AbilityPush:
			cur = ev
			for _, src := range pending {
				credit(ev, src)
			}
			pending = pending[:0]
		}
	}
}

func restrSourceCutPersistent(s string) string {
	for _, suf := range []string{" pmc", " pm"} {
		s = strings.TrimSuffix(s, suf)
	}
	return s
}

// restrSourcePolicy is eagerPolicy steering type choices (restrSourcePickType), counting
// the priority decisions where a nonland hand card is not offered although
// the actor's untapped mana sources would cover its mana value only with
// the restricted-only ones (Giada, Helga, the Crest, a Powerstone): the
// withheld side. The counter is a coverage floor, not an oracle: the
// lockstep compares both engines' option lists at every decision.
func restrSourcePolicy(seed uint64, played map[string]int, gs **GorgeSide, n *[restrSourceN]int) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		if c.G != nil && c.D.Kind == DPriority {
			restrSourceCountWithheld(c, n)
		}
		if a, ok := restrSourcePickType(seed, c); ok {
			return a
		}
		return inner.Pick(c)
	})
}

// restrSourceTypes are the creature types the decks carry: three seeds in four
// steer a Courtyard's or a Crest's choice to one of them (rotated by seed),
// the fourth leaves it to the random policy (usually a type nothing has).
var restrSourceTypes = []string{"Goblin", "Wizard", "Dragon", "Bear", "Dragon", "Cat", "Angel", "Human"}

func restrSourcePickType(seed uint64, c *PickCtx) (Canon, bool) {
	if c.G == nil || seed%4 == 3 {
		return 0, false
	}
	ct, _ := chooseCode("creature_type")
	ty, _ := chooseCode("type")
	for i := range restrSourceTypes {
		want := restrSourceTypes[(int(seed)+i)%len(restrSourceTypes)]
		for _, a := range c.D.Options {
			if k := a.Kind(); k != ActChoose+CanonKind(ct) && k != ActChoose+CanonKind(ty) {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && o.Label == want {
				return a, true
			}
		}
	}
	return 0, false
}

func restrSourceCountWithheld(c *PickCtx, n *[restrSourceN]int) {
	g := c.G.E.G
	p := c.D.Player
	restr, plain := 0, 0 // untapped restricted-only and other mana sources of the actor
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Controller != state.PlayerID(p) || ob.Tapped || ob.Face() == nil {
			continue
		}
		switch ob.Face().Name {
		case "Giada, Font of Hope", "Helga, Skittish Seer", "The Ringhart Crest", "Powerstone Token":
			restr++
		default:
			if ob.Face().IsLand() || ob.Face().Name == "Mind Stone" || ob.Face().Name == "Llanowar Elves" || ob.Face().Name == "Elvish Mystic" {
				plain++
			}
		}
	}
	if restr == 0 {
		return
	}
	offered := map[state.ObjID]bool{}
	for _, a := range c.D.Options {
		if a.Kind() == ActCast {
			if o, ok := c.G.GorgeOption(a); ok {
				offered[o.Obj] = true
			}
		}
	}
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone != state.ZHand || ob.Owner != state.PlayerID(p) || ob.Face() == nil || offered[state.ObjID(i+1)] {
			continue
		}
		// unaffordable from the plain sources, affordable by count with the
		// restricted ones: the restricted source decided the offer
		if mv := int(ob.Face().ManaValue()); !ob.Face().IsLand() && mv > plain && mv <= plain+restr {
			n[restrSourceWithheld]++
			return
		}
	}
}

func restrSourceSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("restrictedsource")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

func TestRestrictedSourceLockstep(t *testing.T) {
	t.Parallel()
	reg, set := restrSourceSetup(t)
	seeds := uint64(48)
	if s := os.Getenv("KF24_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	var fires [restrSourceN]int
	played := map[string]int{}
	games, divs := 0, 0
	for mi, md := range restrSourceDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			if o := os.Getenv("KF24_ONLY"); o != "" && o != fmt.Sprintf("%d:%d", mi, seed) {
				continue
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kf24-a", "kf24-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			res := Lockstep(m, seed, k, restrSourcePolicy(seed, played, &gs, &fires), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			restrSourceScan(gs, &fires)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if res.Div != nil {
				divs++
				if divs == 1 && os.Getenv("KF24_DUMP") != "" {
					for i := max(0, len(res.Actions)-8); i < len(res.Actions); i++ {
						t.Logf("  action %d %v", i, res.Actions[i])
					}
					kcastDump(t, gs)
				}
				if divs <= 6 {
					t.Errorf("matchup %d seed %d: diverged at %d: %s %s %v", mi, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
			}
		}
	}
	t.Logf("%d games, %d divergent; mechanism fires %v", games, divs, fires)
	for i, c := range fires {
		if c < restrSourceMinFires {
			t.Errorf("%s: %d times, want at least %d", restrSourceNames[i], c, restrSourceMinFires)
		}
	}
}

// restrSourceMutation runs the matchups with dirac.DebugRestrictedSource = mut and reports
// whether any seed diverges.
func restrSourceMutation(t *testing.T, mut uint8) {
	reg, set := restrSourceSetup(t)
	old := dirac.DebugRestrictedSource
	dirac.DebugRestrictedSource = mut
	defer func() { dirac.DebugRestrictedSource = old }()
	for mi, md := range restrSourceDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kf24-a", "kf24-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			if res.Div != nil {
				t.Logf("mutation %d diverged: matchup %d seed %d at %d: %s %s", mut, mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Fatalf("mutation %d did not diverge", mut)
}

// TestRestrictedSourceMutation: each dirac.DebugRestrictedSource mutation (1 every restricted
// source admitted for every payment, 2 none, 3 Secluded Courtyard moved to
// the choice-shaped pass) must diverge from gorge.
func TestRestrictedSourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mut := range []uint8{1, 2, 3} {
		restrSourceMutation(t, mut)
	}
}
