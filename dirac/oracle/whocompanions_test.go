package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Doctor's companion lockstep (CR 702.124m): the keyword lowered as a
// no-op deck marker must change no decision, and the class's other newly
// lowered carriers -- Ace, Fearless Rebel's Nitro-9 (a Spawner>
// TriggeredDefendingPlayer fight target read as the attack's defending
// player), Adric's and Syr Ginger's Sac<1/NICKNAME> costs (the source itself)
// -- must play identically in gorge and the kernel. Both seats play the same
// deck over the generated whocompanions pool, so every carrier is cast by
// both engines.

var kwcDeck = []cardset.Entry{
	{Name: "Ace, Fearless Rebel", Count: 2}, {Name: "Adric, Mathematical Genius", Count: 2},
	{Name: "ED-E, Lonesome Eyebot", Count: 2}, {Name: "Graham O'Brien", Count: 2},
	{Name: "Jamie McCrimmon", Count: 2}, {Name: "Jirina, Dauntless General", Count: 2},
	{Name: "Junktown", Count: 2}, {Name: "K-9, Mark I", Count: 2},
	{Name: "Kagemaro, First to Suffer", Count: 1}, {Name: "Nardole, Resourceful Cyborg", Count: 1},
	{Name: "Ornithopter", Count: 2}, {Name: "Romana II", Count: 1},
	{Name: "Syr Ginger, the Meal Ender", Count: 2}, {Name: "Sarah Jane Smith", Count: 1},
	{Name: "Tegan Jovanka", Count: 1}, {Name: "Yasmin Khan", Count: 1},
	{Name: "Island", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	{Name: "Mountain", Count: 1}, {Name: "Swamp", Count: 1},
}

// kwcMin is the play floor for the four carriers the brief's lowering classes
// are measured on; the other eleven classmates are exercised when their games
// cast them and their absence would be a divergence, not a coverage gap.
const kwcMin = 4

// kwcScan counts, in gorge's event log, each lowered carrier's cast.
func kwcScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.PutOnStack || ev.From != state.ZHand {
			continue
		}
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
			n[ob.Face().Name]++
		}
	}
}

func runWhoCompanions(t *testing.T, seeds uint64) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("whocompanions")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = kwcDeck
		for _, e := range kwcDeck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwc-a", "kwc-b"}, decks, 1)
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
		kwcScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestWhoCompanionsLockstep(t *testing.T) {
	t.Parallel()

	games, divs, branch := runWhoCompanions(t, 200)
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	t.Logf("%d games, %d divergences; casts %s", games, divs, strings.Join(bs, " "))
	for _, want := range []string{"K-9, Mark I", "Ace, Fearless Rebel", "Adric, Mathematical Genius", "Syr Ginger, the Meal Ender"} {
		if branch[want] < kwcMin {
			t.Errorf("%s cast %d, want at least %d", want, branch[want], kwcMin)
		}
	}
	for _, other := range []string{"ED-E, Lonesome Eyebot", "Graham O'Brien", "Jamie McCrimmon", "Jirina, Dauntless General", "Nardole, Resourceful Cyborg", "Romana II", "Sarah Jane Smith", "Tegan Jovanka", "Yasmin Khan"} {
		if branch[other] == 0 {
			t.Errorf("%s never cast in %d games", other, games)
		}
	}
}
