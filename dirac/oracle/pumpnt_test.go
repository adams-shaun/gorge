package oracle

import (
	"math/bits"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Pump/PumpAll Duration$ UntilYourNextTurn lockstep (CR 611.2b): a
// keyword-only grant lasts until the cleanup before its controller's next
// turn, the boundary the kernel's ovNT override record already models. The
// three carriers are For the Common Good (PumpAll Indestructible on your
// tokens), Rootwise Survivor (Pump Haste on the land its Survival trigger
// animates) and Elspeth, Storm Slayer (PumpAll Flying on your creatures).

var kpumpntDecks = [2][]cardset.Entry{
	{ // A: white/green -- Elspeth, For the Common Good, Raise the Alarm
		{Name: "Elspeth, Storm Slayer", Count: 1},
		{Name: "For the Common Good", Count: 2},
		{Name: "Raise the Alarm", Count: 2},
		{Name: "Serra Angel", Count: 2},
		{Name: "Day of Judgment", Count: 4},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 13},
		{Name: "Forest", Count: 14},
	},
	{ // B: green -- Rootwise Survivor, sizes and a wipe to answer Elspeth
		{Name: "Rootwise Survivor", Count: 1},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Hill Giant", Count: 3},
		{Name: "Serra Angel", Count: 2},
		{Name: "Day of Judgment", Count: 4},
		{Name: "Forest", Count: 14},
		{Name: "Plains", Count: 13},
	},
}

type kpumpntStats struct {
	grants  int // granted-keyword occurrences created
	liveOpp int // distinct grants observed during an opponent's turn
}

// kpumpntProbe biases priority toward casts and activations (a random
// policy otherwise mostly passes) and reads the grants through the
// projection at every canonical decision.
type kpumpntProbe struct {
	st   *kpumpntStats
	base Policy
	r    *rand.Rand
	tbl  *kop.Table
	pj   Projection      // the projection observe just read
	prev [MaxInst]uint32 // granted keyword bits seen last decision, per instance
	live [MaxInst]uint32 // granted bits already counted live during an opponent's turn
}

// kpumpntGuard is the largest mass grant this test asks the kernel to hold:
// the override pool is 16 records (maxFx), and a PumpAll records one per
// permanent, so a wider grant is a capacity stall, not a rules difference.
const kpumpntGuard = 4

func (p *kpumpntProbe) pick(c *PickCtx) Canon {
	p.observe(c)
	var casts, acts []Canon
	for _, a := range c.D.Options {
		if p.guarded(c, a) {
			continue
		}
		switch a.Kind() {
		case ActCast:
			casts = append(casts, a)
		case ActActivate:
			acts = append(acts, a)
		}
	}
	if len(casts) > 0 && p.r.IntN(4) != 0 {
		return casts[p.r.IntN(len(casts))]
	}
	if len(acts) > 0 && p.r.IntN(2) == 0 {
		return acts[p.r.IntN(len(acts))]
	}
	for i := 0; i < 8; i++ {
		a := p.base.Pick(c)
		if !p.guarded(c, a) {
			return a
		}
	}
	for _, a := range c.D.Options {
		if !p.guarded(c, a) {
			return a
		}
	}
	return c.D.Options[0]
}

// guarded reports whether a is a mass grant wider than kpumpntGuard: an
// Elspeth, Storm Slayer activation or a For the Common Good cast on the
// probe's own board. Such a pick would fill the kernel's fixed override
// pool (StallTables) instead of testing the duration.
func (p *kpumpntProbe) guarded(c *PickCtx, a Canon) bool {
	if a.Kind() != ActActivate && a.Kind() != ActCast {
		return false
	}
	src := int(a.Src())
	if src >= int(p.pj.NInst) || src >= MaxInst {
		return false
	}
	h := p.pj.Cards[src].NameHash
	if h != nameHash("Elspeth, Storm Slayer") && h != nameHash("For the Common Good") {
		return false
	}
	ctrl := p.pj.Cards[src].Controller
	n := 0
	for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
		o := &p.pj.Cards[i]
		if o.Zone == ZoneBattlefield && o.Controller == ctrl && uint16(o.Chars.Types)&uint16(kop.TCreature) != 0 {
			n++
		}
	}
	return n > kpumpntGuard
}

// observe counts each newly granted keyword and each grant that is still
// live while the opponent (not the grant's controller) is the active
// player. A token or copy has no table card, so its printed mask is zero;
// this pool's only tokens are keywordless Soldier tokens.
func (p *kpumpntProbe) observe(c *PickCtx) {
	if c.G == nil {
		return
	}
	c.G.Project(&p.pj)
	for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
		o := &p.pj.Cards[i]
		if o.Zone != ZoneBattlefield || uint16(o.Chars.Types)&uint16(kop.TCreature) == 0 {
			p.prev[i] = 0
			continue
		}
		printed := kop.KwMask(0)
		if o.Card != NoCardIndex && int(o.Card) < len(p.tbl.Cards) {
			printed = p.tbl.Cards[o.Card].Keywords
		}
		granted := uint32(kop.KwMask(o.Chars.Keywords) &^ printed)
		p.st.grants += bits.OnesCount32(granted &^ p.prev[i])
		p.prev[i] = granted
		if o.Controller != p.pj.Active {
			newLive := granted &^ p.live[i]
			p.st.liveOpp += bits.OnesCount32(newLive)
			p.live[i] |= newLive
		}
	}
}

// kpumpntSetup builds the matchup's corpus decks and table.
func kpumpntSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pumpnt")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpumpntDecks {
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
	tbl, _, _, err := set.Matchup(kpumpntDecks[0], kpumpntDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// kpumpntRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity-stall count and the probe's tallies (the harmonize
// shape: MaxTurns bounds each game, and a turn/decision cap is not a kernel
// stall). report logs the first divergence as a test error (the mutation
// run expects one and stays quiet).
func kpumpntRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st kpumpntStats) {
	t.Helper()
	reg, set, tbl, decks := kpumpntSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kpumpnt-a", "kpumpnt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kpumpntProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x70756d70)), tbl: tbl}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			if stalls < 5 {
				t.Logf("seed %d stalled %q at turn %d decision %d", seed, res.Stall, res.Turns, res.Decisions)
			}
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestPumpntLockstep(t *testing.T) {
	div, stalls, st := kpumpntRun(t, 256, true)
	t.Logf("256 games, %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if div {
		t.Fatalf("diverged")
	}
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.grants < 30 {
		t.Errorf("next-turn grants: %d, want at least 30", st.grants)
	}
	if st.liveOpp < 10 {
		t.Errorf("grants live during an opponent's turn: %d, want at least 10", st.liveOpp)
	}
}

// TestPumpntMutation: a next-turn override that ends at end of turn instead
// must make the lockstep diverge.
func TestPumpntMutation(t *testing.T) {
	dirac.DebugPumpNTEndsEOT = true
	defer func() { dirac.DebugPumpNTEndsEOT = false }()
	if diverged, _, _ := kpumpntRun(t, 64, false); !diverged {
		t.Fatal("DebugPumpNTEndsEOT: no divergence in 64 seeds")
	}
}
