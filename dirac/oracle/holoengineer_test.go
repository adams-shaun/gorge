package oracle

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CreateToken rewriter lockstep: Holo-Engineer (the tokens its controller
// would create are 3/3 Illusion artifact creature tokens instead, the same
// number; its own enters token included) beside creature and non-creature token
// makers, with the opponent making tokens under Exalted Sunborn / Ojer Taq so
// the rewriter must leave the opponent's tokens alone. Holo-Engineer is a
// singleton in the decks of the plain lockstep; a second matching replacement
// poses gorge's CR 616.1 order ask, which the kernel poses too
// (repl_tokorder.go); TestHoloEngineerOrderAsk plays those matchups.

const holoIllusion = "c_3_3_a_illusion"

var holoDecks = [2][]cardset.Entry{
	// A: Holo-Engineer once, token makers (creature, Treasure, Clue) and cheap creatures
	{
		{Name: "Holo-Engineer", Count: 1}, {Name: "Dragon Fodder", Count: 4}, {Name: "Raise the Alarm", Count: 4},
		{Name: "Thraben Inspector", Count: 3}, {Name: "Strike It Rich", Count: 3}, {Name: "Goblin Instigator", Count: 3},
		{Name: "Raging Goblin", Count: 2}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
	},
	// B: the opponent's token multipliers and makers, no Holo-Engineer
	{
		{Name: "Exalted Sunborn", Count: 3}, {Name: "Ojer Taq, Deepest Foundation", Count: 2},
		{Name: "Dragon Fodder", Count: 3}, {Name: "Raise the Alarm", Count: 3}, {Name: "Thraben Inspector", Count: 3},
		{Name: "Strike It Rich", Count: 2}, {Name: "Goblin Instigator", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
}

// the order-ask matchup: three Holo-Engineers, and the multipliers beside them
var holoOrderDecks = [2][]cardset.Entry{
	{
		{Name: "Holo-Engineer", Count: 4}, {Name: "Doubling Season", Count: 3}, {Name: "Exalted Sunborn", Count: 3},
		{Name: "Dragon Fodder", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Thraben Inspector", Count: 3},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 6},
	},
	holoDecks[1],
}

// holoTally counts the coverage from gorge's log, per player: the tokens made
// by the player who controls a Holo-Engineer (read when the event is read, so
// the counts are approximate) and the ones made by its opponent.
type holoTally struct {
	ref          *GorgeSide
	evAt         int
	illusions    int // Illusion tokens made under a Holo-Engineer
	holoEnters   int // Holo-Engineer entries (each makes its own token)
	oppOwn       int // tokens made for the Holo-Engineer's opponent that kept their own script
	holoOther    int // a non-Illusion token made under a Holo-Engineer (must be 0)
	rewrittenNon int // Illusions beyond the Holo-Engineer entries: tokens another source made
	asks         int // CR 616.1 token order asks gorge posed (KReplacement decisions during a token creation)
}

func (p *holoTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if d := ref.E.Pending(); d != nil && d.Kind == decision.KReplacement && strings.Contains(d.Prompt, "token creation") {
		p.asks++
	}
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	holoCtl := func(pl state.PlayerID) bool {
		for _, id := range g.Zone(state.ZBattlefield, pl) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Holo-Engineer" {
				return true
			}
		}
		return false
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.MoveZone:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Holo-Engineer" && e.To == state.ZBattlefield {
				p.holoEnters++
			}
		case events.TokenCreate:
			switch {
			case holoCtl(e.Player) && e.Text == holoIllusion:
				p.illusions++
			case holoCtl(e.Player):
				p.holoOther++
			case holoCtl(1-e.Player) && e.Text != holoIllusion:
				p.oppOwn++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

func holoMatchup(t *testing.T, md [2][]cardset.Entry) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("holoengineer")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range md {
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
			t.Fatalf("deck %d (%v): %d cards", s, d[0].Name, len(decks[s]))
		}
	}
	return reg, set, decks
}

// runHolo plays the matchup and returns the games, the divergences, the stalls
// (by the kernel's reason) and the tally. mutate makes the kernel ignore the
// rewriters.
func runHolo(t *testing.T, md [2][]cardset.Entry, seeds int, mutate bool) (games, divs int, stalls map[string]int, tl *holoTally) {
	t.Helper()
	reg, set, decks := holoMatchup(t, md)
	old := dirac.DebugTokRewriteOff
	dirac.DebugTokRewriteOff = mutate
	defer func() { dirac.DebugTokRewriteOff = old }()
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	tl = &holoTally{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		if v := os.Getenv("HOLO_ONLY"); v != "" && v != fmt.Sprint(seed) {
			continue
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"holo-a", "holo-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kreplmultPolicy(seed, &kreplmultTally{}),
			RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tl.check})
		CloseSide(k)
		games++
		if r, ok := strings.CutPrefix(res.Stall, KernelStallPrefix); ok {
			stalls[r]++
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, tl
}

func TestHoloEngineerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runHolo(t, holoDecks, 192, false)
	t.Logf("%d games, %d divergences, stalls %v, %d Illusions under Holo-Engineer (%d entries), %d own-script tokens for its opponent, %d other tokens under it",
		games, divs, stalls, tl.illusions, tl.holoEnters, tl.oppOwn, tl.holoOther)
	if divs != 0 || len(stalls) != 0 {
		t.Fatalf("%d divergences, stalls %v", divs, stalls)
	}
	if tl.holoOther != 0 {
		t.Errorf("gorge made %d tokens under a Holo-Engineer that were not Illusions", tl.holoOther)
	}
	// the Holo-Engineer entries' own Illusions are one each; every one past that
	// is another source's token rewritten
	if rw := tl.illusions - tl.holoEnters; rw < 40 {
		t.Errorf("%d tokens of other sources rewritten, want at least 40", rw)
	}
	if tl.oppOwn < 40 {
		t.Errorf("%d tokens made for the opponent kept their script, want at least 40", tl.oppOwn)
	}
}

// TestHoloEngineerMutation ignores the rewriters (every token keeps its own
// script): the games must diverge.
func TestHoloEngineerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runHolo(t, holoDecks, 48, true)
	if divs == 0 {
		t.Fatalf("mutation ignored the rewriters in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}

// holoAskPairs are the order-ask matchups: Holo-Engineer beside a token
// multiplier it contests (Doubling Season, Exalted Sunborn), beside a second
// Holo-Engineer, and the plain order matchup (three kinds of competitor).
var holoAskPairs = map[string][2][]cardset.Entry{
	"order": holoOrderDecks,
	"doubling": {
		{
			{Name: "Holo-Engineer", Count: 3}, {Name: "Doubling Season", Count: 4},
			{Name: "Dragon Fodder", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Thraben Inspector", Count: 3},
			{Name: "Strike It Rich", Count: 2}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
		},
		holoDecks[1],
	},
	"twoholo": {
		{
			{Name: "Holo-Engineer", Count: 8},
			{Name: "Dragon Fodder", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Thraben Inspector", Count: 3},
			{Name: "Strike It Rich", Count: 3}, {Name: "Goblin Instigator", Count: 2}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 5},
		},
		holoDecks[1],
	},
	"sunborn": {
		{
			{Name: "Holo-Engineer", Count: 3}, {Name: "Exalted Sunborn", Count: 4},
			{Name: "Dragon Fodder", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Thraben Inspector", Count: 3},
			{Name: "Strike It Rich", Count: 2}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6},
		},
		holoDecks[1],
	},
}

// TestHoloEngineerOrderAsk: a Holo-Engineer beside a token multiplier (or a
// second Holo-Engineer) is gorge's CR 616.1 order ask, which the kernel poses
// too and plays on from: no game diverges, none stalls on the order, and the
// games reach the contested creation (the asks are answered in lockstep, the
// ones counted here from gorge's replacement decisions).
func TestHoloEngineerOrderAsk(t *testing.T) {
	if isolate(t) {
		return
	}
	names := make([]string, 0, len(holoAskPairs))
	for n := range holoAskPairs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		games, divs, stalls, tl := runHolo(t, holoAskPairs[name], 96, false)
		t.Logf("%s: %d games, %d divergences, stalls %v, %d order asks answered", name, games, divs, stalls, tl.asks)
		if divs != 0 {
			t.Errorf("%s: %d divergences", name, divs)
		}
		for r, n := range stalls {
			// five or more candidates are past the ask's four (the declared StallReplCands)
			if r != "replcands" && r != "board" {
				t.Errorf("%s: %d games stalled on %q", name, n, r)
			}
		}
		if tl.asks < 10 {
			t.Errorf("%s: %d token order asks reached in %d games, want at least 10", name, tl.asks, games)
		}
	}
}
