package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The DigUntil ImprintFound$/RememberRevealed$ lockstep over the generated
// digimprint pool, whose carriers are the three shapes gorge's effDigUntil
// digs apart from the Jodah trio (dirac/kgen/lower_digimprint.go):
//
//   - Dream Harvest: RememberRevealed$ alone with MinTotalCMC$ 5, each
//     opponent digs until the revealed matches' mana values sum to 5, and
//     the controller may free-cast the remembered (all revealed) cards;
//   - Improvisation Capstone: the same RememberRevealed$+MinTotalCMC$ shape
//     over its own library, a DB$ Play tail (CastFree) instead of a static;
//   - Black Widow, Super Spy: ImprintFound$ alone — the damaged player digs
//     until a nonland card, then "You may put a +1/+1 counter on NICKNAME.
//     If you don't, you may cast the exiled nonland card" (the optional
//     put's RememberPut$ feeds the Effect's ConditionDefined$ Remembered
//     EQ0, and the seek-found imprint is the kernel's chosen set).
//
// Deck A digs with the sorceries, deck B attacks with Black Widow; lands
// dominate both libraries so the scans walk.

var kimDecks = [2][]cardset.Entry{
	{
		{Name: "Dream Harvest", Count: 4}, {Name: "Improvisation Capstone", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Black Widow, Super Spy", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

const kimMin = 2

// kimScan counts the mechanics in gorge's event log: each carrier's
// library-to-exile dig window (the resolving ability is the latest pushed
// trigger or cast spell), Black Widow's accepted +1/+1 counter, and every
// cast out of exile (the free casts the digs grant).
func kimScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	face := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	last := "" // the latest cast spell or pushed trigger: the dig's resolution
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack, events.TriggerPush, events.AbilityPush:
			if nm := face(ev.Obj); nm != "" {
				last = nm
			}
			if ev.Kind == events.PutOnStack && ev.From == state.ZExile {
				// a cast out of exile: the digs' free casts
				n["freecast"]++
			}
		case events.MoveZone:
			if ev.From == state.ZLibrary && ev.To == state.ZExile {
				switch last {
				case "Dream Harvest":
					n["dream"]++
				case "Improvisation Capstone":
					n["cap"]++
				case "Black Widow, Super Spy":
					n["widow"]++
				}
			}
		case events.CounterChange:
			if face(ev.Obj) == "Black Widow, Super Spy" {
				n["widowput"]++
			}
		}
	}
}

// runDigimprint plays the seeded games; mutate clears the DigUntil
// ImprintFound$/RememberRevealed$/MinTotalCMC$ flags in the generated table
// and stops at the first divergence.
func runDigimprint(t *testing.T, seeds uint64, mutate bool) (games, divs int, played, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("digimprint")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kimDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kimDecks[0], kimDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		cleared := 0
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				for pi := range tbl.Cards[ci].Abilities[ai].Prog {
					in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
					if in.Op != kop.OpDigUntil {
						continue
					}
					for _, bit := range []int{kop.DuRememberRevealed, kop.DuMinTotalCMC, kop.DuImprintFound} {
						in.C &^= int16(bit) << 3
					}
					cleared++
				}
			}
		}
		if cleared == 0 {
			t.Fatal("mutation found no DigUntil in the pool")
		}
	}
	played, branch = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kim-a", "kim-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		kimScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			kczDump(t, gs)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestDigunttilImprintLockstep(t *testing.T) {
	t.Parallel()

	games, divs, _, branch := runDigimprint(t, 256, false)
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	t.Logf("%d games, %d divergences; resolutions %s", games, divs, strings.Join(bs, " "))
	for _, want := range []string{"dream", "cap", "widow", "widowput", "freecast"} {
		if branch[want] < kimMin {
			t.Errorf("%s resolutions %d, want at least %d", want, branch[want], kimMin)
		}
	}
}

// TestDigimprintMutation proves the pool digs: without the
// RememberRevealed$/MinTotalCMC$/ImprintFound$ flags some seed diverges.
func TestDigimprintMutation(t *testing.T) {
	t.Parallel()

	games, divs, _, _ := runDigimprint(t, 200, true)
	if divs == 0 {
		t.Fatalf("flag mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
