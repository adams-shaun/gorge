package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Origin$ Library,Graveyard search lockstep: Journey for the Elixir
// searches its caster's library and graveyard for a basic land card, then
// for a card named Jiang Yanggu, puts them into the hand, then shuffles
// (gorge effSearchLibrary over both zones, library first, no zone election;
// dirac's OpSearchEx with SrAltOrigin|SrAltAll). Thought Scour, Mulch and
// Merfolk Looter put basic lands and Jiang Yanggu into the graveyard. The
// expected answer at every decision is gorge's.
var klibgraveDecks = [2][]cardset.Entry{
	{
		{Name: "Journey for the Elixir", Count: 4}, {Name: "Jiang Yanggu", Count: 3},
		{Name: "Thought Scour", Count: 4}, {Name: "Mulch", Count: 3},
		{Name: "Merfolk Looter", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Journey for the Elixir", Count: 4}, {Name: "Jiang Yanggu", Count: 2},
		{Name: "Thought Scour", Count: 4}, {Name: "Mulch", Count: 4},
		{Name: "Merfolk Looter", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Island", Count: 6},
	},
}

// klibgraveTally is what the played games reached, read from gorge's log.
type klibgraveTally struct {
	games, div, stalls int
	journeys           int // Journey for the Elixir resolutions
	landFromGrave      int // a basic land put into a hand from a graveyard
	jiangFromGrave     int // Jiang Yanggu put into a hand from a graveyard
	jiangFromLib       int // Jiang Yanggu put into a hand from a library by Journey's search
}

// klibgraveScan counts the searches' outcomes in gorge's event log. Nothing
// else in the pool moves a card from a graveyard to a hand, and only Journey
// moves Jiang Yanggu from a library to a hand outside a draw.
func klibgraveScan(g *GorgeSide, tl *klibgraveTally) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	resolving := false
	for _, ev := range g.E.L.Events {
		n := name(ev.Obj)
		if ev.Kind == events.PutOnStack && n == "Journey for the Elixir" {
			resolving = true // cast; its searches' moves precede its own move off the stack
		}
		if ev.Kind != events.MoveZone {
			continue
		}
		switch {
		case n == "Journey for the Elixir" && ev.From == state.ZStack:
			tl.journeys++
			resolving = false
		case ev.To != state.ZHand:
		case ev.From == state.ZGraveyard && (n == "Forest" || n == "Island"):
			tl.landFromGrave++
		case ev.From == state.ZGraveyard && n == "Jiang Yanggu":
			tl.jiangFromGrave++
		case ev.From == state.ZLibrary && n == "Jiang Yanggu" && resolving:
			tl.jiangFromLib++
		}
	}
}

// klibgraveRun plays seeds 1..seeds with both seatings; it stops at the
// first divergence.
func klibgraveRun(t *testing.T, seeds uint64) (*klibgraveTally, *Divergence) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("libgrave")
	if err != nil {
		t.Fatal(err)
	}
	tl := &klibgraveTally{}
	for swap := 0; swap < 2; swap++ {
		md := klibgraveDecks
		if swap == 1 {
			md = [2][]cardset.Entry{klibgraveDecks[1], klibgraveDecks[0]}
		}
		var gd [2][]*cards.Card
		for s, d := range md {
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
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			m, err := NewMatchup(reg, [2]string{"klibgrave-a", "klibgrave-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
			CloseSide(k)
			tl.games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				tl.stalls++
			}
			klibgraveScan(g, tl)
			if d := res.Div; d != nil {
				tl.div++
				lo := max(d.At-12, 0)
				t.Logf("swap %d seed %d diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
				return tl, d
			}
		}
	}
	return tl, nil
}

func TestLibGraveSearchLockstep(t *testing.T) {
	t.Parallel()
	tl, div := klibgraveRun(t, 48)
	t.Logf("%d games, %d divergences, %d stalls; Journey resolutions %d: basic lands from a graveyard %d, Jiang Yanggu from a graveyard %d, from a library %d",
		tl.games, tl.div, tl.stalls, tl.journeys, tl.landFromGrave, tl.jiangFromGrave, tl.jiangFromLib)
	if div != nil {
		t.Fatalf("diverged at decision %d: %s %s %v %s", div.At, div.What, div.Field, div.Diff, div.Detail)
	}
	if tl.stalls != 0 {
		t.Errorf("kernel capacity stalls %d, want 0", tl.stalls)
	}
	for name, n := range map[string]int{"Journey resolutions": tl.journeys, "basic lands from a graveyard": tl.landFromGrave,
		"Jiang Yanggu from a graveyard": tl.jiangFromGrave, "Jiang Yanggu from a library": tl.jiangFromLib} {
		if n < 3 {
			t.Errorf("%s: %d, want at least 3", name, n)
		}
	}
}
