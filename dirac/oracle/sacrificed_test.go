package oracle

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The sacrificed-this-turn lockstep: Sawblade Skinripper
// (end-step damage = permanents sacrificed), Obsessive Pursuit (attack counters),
// Detective's Satchel (a Thopter only after an artifact sacrifice), Furtive
// Courier (unblockable after an artifact sacrifice) and Suspicious Detonation
// (3 cheaper after an artifact sacrifice) over Clue and Food makers, creature
// sacrifice outlets (Sawblade, Hungry Ghoul) and a steal (Control Magic: the
// sacrificer is the controller now, not the owner). Gorge's answer at every
// decision is the expected one.
var ksacrificedDecks = [2][]cardset.Entry{
	{ // A: B/R/U
		{Name: "Obsessive Pursuit", Count: 4}, {Name: "Sawblade Skinripper", Count: 4}, {Name: "Detective's Satchel", Count: 3},
		{Name: "Furtive Courier", Count: 3}, {Name: "Suspicious Detonation", Count: 3}, {Name: "Hungry Ghoul", Count: 2},
		{Name: "Bake into a Pie", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 4},
	},
	{ // B: B/R/U with a steal, the opponent that sacrifices on its own turns
		{Name: "Obsessive Pursuit", Count: 3}, {Name: "Sawblade Skinripper", Count: 3}, {Name: "Detective's Satchel", Count: 2},
		{Name: "Furtive Courier", Count: 2}, {Name: "Suspicious Detonation", Count: 2}, {Name: "Hungry Ghoul", Count: 3},
		{Name: "Control Magic", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Llanowar Elves", Count: 1},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Island", Count: 4},
	},
}

type ksacrificedStats struct {
	games, artifactSacs, sacs int
	nonzeroTurns, resetTurns  int // turns whose active player has sacrificed >= 1 / has sacrificed 0 after a nonzero turn of its own
	maxAll, maxArtifact       int
	carrierLive               map[string]int // turns with a sacrifice this turn and the carrier on the active player's battlefield
}

func ksacrificedSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sacrificed")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ksacrificedDecks {
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
	return reg, set, decks
}

// ksacrificedProbe measures how often the pool reaches the mechanism, from
// gorge's own Entered list (Sacrificed && Sacrificer == the active player):
// per game, the active player's tally at the last sample of each of its turns.
type ksacrificedProbe struct {
	stats   *ksacrificedStats
	turn    int
	active  state.PlayerID
	all     int
	art     int
	live    map[string]bool
	nonzero [2]bool // the player has had a nonzero turn this game
}

func (p *ksacrificedProbe) flush() {
	if p.turn == 0 {
		return
	}
	st := p.stats
	switch {
	case p.all > 0:
		st.nonzeroTurns++
		p.nonzero[p.active&1] = true
		for n := range p.live {
			st.carrierLive[n]++
		}
	case p.nonzero[p.active&1]:
		st.resetTurns++
	}
	st.sacs += p.all
	st.artifactSacs += p.art
}

func (p *ksacrificedProbe) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if t := int(g.Turn); t != p.turn {
		p.flush()
		p.turn, p.active, p.all, p.art, p.live = t, g.Active, 0, 0, map[string]bool{}
	}
	all, art := 0, 0
	for _, en := range g.Entered {
		if !en.Sacrificed || en.Controller != g.Active {
			continue
		}
		all++
		if o := g.Obj(en.Obj); o != nil {
			if f := o.Face(); f != nil && f.IsArtifact() {
				art++
			}
		}
	}
	p.all, p.art = all, art
	st := p.stats
	st.maxAll, st.maxArtifact = max(st.maxAll, all), max(st.maxArtifact, art)
	if all > 0 {
		for _, id := range g.Zone(state.ZBattlefield, g.Active) {
			if o := g.Obj(id); o != nil && o.Face() != nil {
				p.live[o.Face().Name] = true
			}
		}
	}
	return nil
}

// ksacrificedRun plays seeds 1..seeds with the probe, reporting divergences
// as test errors when report is set (the mutation run expects one and stays
// quiet).
func ksacrificedRun(t *testing.T, seeds uint64, report bool) (divs, stalls int, st *ksacrificedStats) {
	t.Helper()
	reg, set, decks := ksacrificedSetup(t)
	tbl, _, _, err := set.Matchup(ksacrificedDecks[0], ksacrificedDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &ksacrificedStats{carrierLive: map[string]int{}}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ksacrificed-a", "ksacrificed-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pr := &ksacrificedProbe{stats: st}
		pol := castActivatePolicy(seed)
		if seed%4 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24, Check: pr.check})
		pr.flush()
		CloseSide(k)
		st.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Logf("seed %d stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !report {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestSacrificedLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(256)
	divs, stalls, st := ksacrificedRun(t, seeds, true)
	var live []string
	for n, c := range st.carrierLive {
		live = append(live, n+"="+strconv.Itoa(c))
	}
	sort.Strings(live)
	t.Logf("%d games, %d divergences, %d stalls; %d sacrifices (%d artifacts), max %d per turn (%d artifacts); %d turns with a nonzero count, %d later turns reading 0; turns with a sacrifice and the carrier in play: %v",
		st.games, divs, stalls, st.sacs, st.artifactSacs, st.maxAll, st.maxArtifact, st.nonzeroTurns, st.resetTurns, live)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.artifactSacs < 1 {
		t.Errorf("no artifact sacrifice: the pool does not reach ExSacrificedThisTurn N=1")
	}
	if st.nonzeroTurns < 1 {
		t.Errorf("no turn with a nonzero sacrificed count")
	}
	if st.resetTurns < 1 {
		t.Errorf("no later turn reads 0: the turn reset is unexercised")
	}
}

// TestSacrificedMutation: a tally that survives the turn boundary must make
// the lockstep diverge.
func TestSacrificedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSacrificedNoTurnReset = true
	defer func() { dirac.DebugSacrificedNoTurnReset = false }()
	if divs, _, _ := ksacrificedRun(t, 64, false); divs == 0 {
		t.Fatal("DebugSacrificedNoTurnReset: no divergence in 64 seeds")
	}
}
