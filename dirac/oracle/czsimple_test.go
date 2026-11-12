package oracle

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: ChangeZone NoShuffle$ (Claim Jumper's
// unshuffled Plains searches), NoReveal$ (Case of the Stashed Skeleton's
// solved tutor), AlreadyRevealed$ (Intimidation Tactics' hand exile) and the
// targeted move's ChangeNum$ 1 (Virtue of Persistence's graveyard return),
// all exact reads. The W/B decks carry removal, artifacts and creatures so
// the graveyards and hands exercise the carriers, and the opponent is
// land-heavy with nonbasic lands so Claim Jumper's "more lands" check flips
// both ways.
var kczsimpleDecks = [2][]cardset.Entry{
	{
		{Name: "Claim Jumper", Count: 4}, {Name: "Case of the Stashed Skeleton", Count: 4},
		{Name: "Intimidation Tactics", Count: 4}, {Name: "Virtue of Persistence", Count: 3},
		{Name: "Elite Vanguard", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Doom Blade", Count: 2}, {Name: "Murder", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Elite Vanguard", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Silvercoat Lion", Count: 3}, {Name: "Ornithopter", Count: 3},
		{Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 2},
		{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 6}, {Name: "Scoured Barrens", Count: 4},
	},
}

// kczsimpleTally is what the played games reached.
type kczsimpleTally struct {
	games, div, stalls         int
	cjSearch, cjFound, cjBoth  int
	tactics, virtue, caseTutor int
	played                     map[string]int
}

// kczsimpleScan counts the four mechanics in gorge's event log (gorge leads
// the lockstep, so its log is the reference).
func kczsimpleScan(g *GorgeSide, tl *kczsimpleTally) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	cjSearches := 0
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush:
			// One Claim Jumper trigger runs its two ChangeZone searches
			// before any other trigger pushes; a second search in the same
			// trigger is the "repeat this process once" arm.
			if name(ev.Obj) == "Claim Jumper" {
				cjSearches = 0
			}
		case events.SearchedLibrary:
			switch name(ev.Obj) {
			case "Claim Jumper":
				tl.cjSearch++
				cjSearches++
				if cjSearches == 2 {
					tl.cjBoth++
				}
			case "Case of the Stashed Skeleton":
				// the solved ability's tutor (Activation$ Solved), the only
				// Case search
				tl.caseTutor++
			}
		case events.MoveZone:
			switch {
			case ev.From == state.ZLibrary && ev.To == state.ZBattlefield && name(ev.Obj) == "Plains":
				// Claim Jumper is the pool's only library-to-battlefield
				// search.
				tl.cjFound++
			case ev.From == state.ZHand && ev.To == state.ZExile:
				// Intimidation Tactics exiles the chosen artifact/creature.
				tl.tactics++
			case ev.From == state.ZGraveyard && ev.To == state.ZBattlefield:
				// Virtue of Persistence's upkeep return; only it moves a
				// graveyard card to the battlefield here.
				tl.virtue++
			}
		}
	}
}

// kczsimpleRun plays seeds 1..seeds of the two decks, mutation applied to the
// kernel table first. It stops at the first divergence; report logs it.
func kczsimpleRun(t *testing.T, seeds uint64, mutate func(*kop.Table), report bool) (*kczsimpleTally, *Divergence) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("czsimple")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kczsimpleDecks {
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
	tbl, _, _, err := set.Matchup(kczsimpleDecks[0], kczsimpleDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	tl := &kczsimpleTally{played: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kczsimple-a", "kczsimple-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, tl.played, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		tl.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			tl.stalls++
		}
		if res.Div != nil {
			tl.div++
			if report {
				lo := max(res.Div.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
			}
			return tl, res.Div
		}
		kczsimpleScan(gs, tl)
	}
	return tl, nil
}

func (tl *kczsimpleTally) log(t *testing.T) {
	t.Helper()
	var names []string
	for n := range tl.played {
		names = append(names, n)
	}
	sort.Strings(names)
	var plays []string
	for _, n := range names {
		plays = append(plays, n+"="+strconv.Itoa(tl.played[n]))
	}
	t.Logf("%d games: searches=%d foundPlains=%d bothSearches=%d tacticsExiles=%d virtueReturns=%d caseTutors=%d stalls=%d",
		tl.games, tl.cjSearch, tl.cjFound, tl.cjBoth, tl.tactics, tl.virtue, tl.caseTutor, tl.stalls)
	t.Logf("carrier plays: %s", strings.Join(plays, " "))
}

func TestCzsimpleLockstep(t *testing.T) {
	t.Parallel()
	tl, div := kczsimpleRun(t, 256, nil, true)
	if div != nil {
		t.Errorf("diverged at decision %d: %s %s %v %s\nref %v\nker %v", div.At, div.What, div.Field, div.Diff, div.Detail, div.RefOptions, div.KerOptions)
		return
	}
	tl.log(t)
	if tl.stalls != 0 {
		t.Errorf("kernel capacity stalls %d, want 0", tl.stalls)
	}
	if tl.cjFound < 20 {
		t.Errorf("Claim Jumper found a Plains %d times, want at least 20", tl.cjFound)
	}
	if tl.cjBoth < 5 {
		t.Errorf("Claim Jumper ran both searches %d times, want at least 5", tl.cjBoth)
	}
	if tl.tactics < 10 {
		t.Errorf("Intimidation Tactics exiled %d cards, want at least 10", tl.tactics)
	}
	if tl.virtue < 10 {
		t.Errorf("Virtue of Persistence returned %d creature cards, want at least 10", tl.virtue)
	}
	if tl.caseTutor < 3 {
		t.Errorf("Case of the Stashed Skeleton tutored %d times, want at least 3", tl.caseTutor)
	}
}

// TestCzsimpleMutation: clearing SrNoShuffle from Claim Jumper's two search
// instructions in the kernel table makes the kernel shuffle while gorge does
// not, so the library order diverges.
func TestCzsimpleMutation(t *testing.T) {
	t.Parallel()
	n := 0
	tl, div := kczsimpleRun(t, 128, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name != "Claim Jumper" {
				continue
			}
			for j := range tbl.Cards[i].Abilities {
				for x := range tbl.Cards[i].Abilities[j].Prog {
					in := &tbl.Cards[i].Abilities[j].Prog[x]
					if in.Op == kop.OpSearchEx && in.B.N&kop.SrNoShuffle != 0 {
						in.B.N &^= kop.SrNoShuffle
						n++
					}
				}
			}
		}
	}, false)
	if n != 2 {
		t.Fatalf("cleared SrNoShuffle from %d Claim Jumper searches, want 2", n)
	}
	if div == nil {
		t.Fatalf("no divergence in %d seeds with SrNoShuffle cleared (searches=%d foundPlains=%d)", tl.games, tl.cjSearch, tl.cjFound)
	}
	t.Logf("diverged on seed %d/%d at decision %d: %s %s", tl.games, 128, div.At, div.What, div.Field)
}
