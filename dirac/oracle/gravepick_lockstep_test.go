package oracle

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The untargeted graveyard-pick lockstep (gorge
// effHiddenPick over a graveyard): the pool holds the corpus cards the
// hidden pick lowered for. Bloodcurdler's end-step exile names its fetch
// player (DefinedPlayer$ You, ChangeType$ Card.YouCtrl, two picks); the
// other four are game-wide (gorge gameWide: no DefinedPlayer$, every
// player's graveyard in seat order, the picked card entering its owner's
// exile) -- Charnel Troll (Mandatory$ with RememberChanged$ and the
// condition chain behind it), Deathbonnet Sprout's back face (the
// OptionalDecider trigger's may-pick), Entrails Feaster (Optional$ confirm
// before the pick, Chooser$ You) and False Memories (seven mandatory picks
// of its own graveyard's cards, behind a delayed trigger). Beside them the
// creatures and removal that fill the graveyards the pools read, the loot
// and mill that reach Bloodcurdler's threshold and Deathbonnet Sprout's
// three creature cards, and the lands.
//
// The Standard cards that raised the class (Deadly Cover-Up, Kaya,
// Spirits' Justice, Lazav, Familiar Stranger, Azula, Cunning Usurper, Hama,
// the Bloodbender) lower their ChangeZone now but stay refused by kgen for
// the classes their other nodes carry (other tickets'); no fully-lowered
// card carries the DefinedPlayer$ Targeted arm yet, so the targeted
// player's-graveyard composition (pickP a player target, the plain
// graveyard walk) is exercised only through the lowered Azula and Hama
// instructions' shared paths, not by a played game here.
var kgravepickDecks = [2][]cardset.Entry{
	{
		{Name: "Bloodcurdler", Count: 4}, {Name: "Charnel Troll", Count: 4},
		{Name: "Entrails Feaster", Count: 4}, {Name: "False Memories", Count: 3},
		{Name: "Deathbonnet Sprout", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Doom Blade", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Tome Scour", Count: 2}, {Name: "Cathartic Reunion", Count: 2},
		{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Charnel Troll", Count: 3}, {Name: "Entrails Feaster", Count: 3},
		{Name: "Deathbonnet Sprout", Count: 3}, {Name: "False Memories", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Silvercoat Lion", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Terror", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Frantic Search", Count: 2},
		{Name: "Swamp", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 2},
	},
}

// kgravepickTally is what the played games reached. Every graveyard-to-exile
// move in this pool is one of the lowered picks: the pool's removal
// destroys, its loot discards and its mills move to the graveyard, and no
// card exiles from anywhere else.
type kgravepickTally struct {
	games, divs, moves int
	movedNames         map[string]int
	casts              map[string]int
}

// kgravepickPolicy is the cast-first random policy of the czsimple lockstep,
// capturing the gorge side for the event scan.
func kgravepickPolicy(seed uint64, casts map[string]int, gs **GorgeSide) Policy {
	inner := eagerPolicy(seed, casts)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			*gs = c.G
		}
		return inner.Pick(c)
	})
}

// kgravepickScan counts the picks in gorge's event log (gorge leads the
// lockstep, so its log is the reference).
func kgravepickScan(g *GorgeSide, tl *kgravepickTally) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZExile {
			tl.moves++
			name := ""
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
				name = ob.Face().Name
			}
			tl.movedNames[name]++
		}
	}
}

func TestGraveyardPickUntargetedLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gravepick")
	if err != nil {
		// A carrier kgen still refuses leaves the pool: the untargeted
		// graveyard-pick refusal must be gone before this test can run.
		t.Fatalf("gravepick pool: %v", err)
	}
	var gd [2][]*cards.Card
	for s, d := range kgravepickDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kgravepickDecks[0], kgravepickDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tl := &kgravepickTally{movedNames: map[string]int{}, casts: map[string]int{}}
	for seed := uint64(1); seed <= 192; seed++ {
		m, err := NewMatchup(reg, [2]string{"gravepick-a", "gravepick-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, kgravepickPolicy(seed, tl.casts, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		tl.games++
		if res.Div != nil {
			tl.divs++
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions, actionStrings(res.Actions[lo:res.Div.At]))
		}
		kgravepickScan(gs, tl)
	}
	var names []string
	for n := range tl.movedNames {
		names = append(names, n)
	}
	sort.Strings(names)
	var plays []string
	for _, n := range names {
		plays = append(plays, n+"="+strconv.Itoa(tl.movedNames[n]))
	}
	t.Logf("%d games: %d graveyard picks moved %v", tl.games, tl.moves, strings.Join(plays, " "))
	if tl.moves < 20 {
		t.Errorf("%d games moved %d cards from a graveyard, want at least 20", tl.games, tl.moves)
	}
	if len(tl.movedNames) < 3 {
		t.Errorf("the picks moved %d distinct cards, want at least 3 (%v)", len(tl.movedNames), names)
	}
	if tl.casts["False Memories"] < 3 {
		t.Errorf("False Memories cast %d times, want at least 3 (its seven-pick game-wide mandatory pick)", tl.casts["False Memories"])
	}
}
