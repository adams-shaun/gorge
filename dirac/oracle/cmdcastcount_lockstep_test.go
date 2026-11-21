package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// cmdCastPodEntries is the cmdcastcount fixture's four 40-card decks. Each
// opens with its commander (a partner pair in the last deck: Rograkh, then
// Tymna) beside the Count$TotalCommanderCastFromCommandZone readers and cheap
// removal (burn, exile, bounce), so commanders die or leave and are cast again
// at a rising tax.
func cmdCastPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Isamaru, Hound of Konda", Count: 1}, {Name: "Commander's Insignia", Count: 2}, {Name: "Empyrial Storm", Count: 2},
			{Name: "Hatut Zeraze Strike Force", Count: 2}, {Name: "Vanguard of the Restless", Count: 2}, {Name: "Fury Storm", Count: 2},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Journey to Nowhere", Count: 2}, {Name: "Savannah Lions", Count: 4},
			{Name: "Raging Goblin", Count: 3}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Zurgo Bellstriker", Count: 1}, {Name: "Captain Vargus Wrath", Count: 2}, {Name: "Font of Magic", Count: 2},
			{Name: "Echo Storm", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Unsummon", Count: 3}, {Name: "Raging Goblin", Count: 3},
			{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Rofellos, Llanowar Emissary", Count: 1}, {Name: "Jyoti, Moag Ancient", Count: 2}, {Name: "The Swarmlord", Count: 2},
			{Name: "Unsummon", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Giant Growth", Count: 3}, {Name: "Runeclaw Bear", Count: 3}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 4},
		},
		{
			{Name: "Rograkh, Son of Rohgahh", Count: 1}, {Name: "Tymna the Weaver", Count: 1}, {Name: "Jirina Kudro", Count: 2},
			{Name: "Journey to Nowhere", Count: 3}, {Name: "Doom Blade", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Savannah Lions", Count: 4},
			{Name: "Raging Goblin", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Shock", Count: 2},
			{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
		},
	}
}

// cmdCastPod is a Commander pod over the cmdcastcount fixture.
type cmdCastPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][]cardset.Entry
	// partner is true when the last seat's deck opens with a partner pair.
	partner bool
}

// cmdCastResult is one game's reads of the commander cast count.
type cmdCastResult struct {
	res      Result
	stats    dirac.CmdCastStats
	maxCasts int32 // the most casts of one seat's commanders gorge counted at the end
}

func newCmdCastPod(t *testing.T, decks [][]cardset.Entry, partner bool) cmdCastPod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table read
	set, err := cardset.RawByName("cmdcastcount")
	if err != nil {
		t.Fatal(err)
	}
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
	}
	return cmdCastPod{reg: reg, set: set, decks: decks, partner: partner}
}

// repro is the pod as a Repro: every seat's commander is deck index 0, and in
// a partner pod the last seat's partner is index 1.
func (p cmdCastPod) repro(seed uint64) *Repro {
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"c0", "c1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full", Commander: true}
	r.Commanders = make([]int, len(p.decks))
	if p.partner {
		r.Partners = make([]int, len(p.decks))
		for i := range r.Partners {
			r.Partners[i] = -1
		}
		r.Partners[len(p.decks)-1] = 1
	}
	for s := 2; s < len(p.decks); s++ {
		r.MoreDecks = append(r.MoreDecks, "c"+string(rune('0'+s)))
		r.MoreCards = append(r.MoreCards, expand(p.decks[s]))
	}
	return r
}

// play plays one seed in lockstep, gorge leading, and reads back how often
// the kernel evaluated the cast count and what it was.
func (p cmdCastPod) play(t *testing.T, seed uint64, maxTurns int32) cmdCastResult {
	t.Helper()
	r := p.repro(seed)
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := p.set.MatchupN(p.cardLists())
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var out cmdCastResult
	rc.Finish = func(g *GorgeSide) {
		for pl := range g.E.G.Players {
			out.maxCasts = max(out.maxCasts, g.E.CommanderCastsFromCommandZone(state.PlayerID(pl)))
		}
	}
	st := &dirac.CmdCastStats{}
	dirac.DebugCmdCastStats = st
	defer func() { dirac.DebugCmdCastStats = nil }()
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	out.res = LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	out.stats = *st
	if out.res.Div != nil {
		for _, id := range []state.ObjID{40, 61, 62} {
			if o := lead.E.G.Obj(id); o != nil && o.Face() != nil {
				t.Logf("DEBUG #%d = %s zone %v", id, o.Face().Name, o.Zone)
			}
		}
	}
	return out
}

func (p cmdCastPod) cardLists() [][]cardset.Entry { return p.decks }

// check fails on a divergence or a kernel stop.
func (o cmdCastResult) check(t *testing.T, seed uint64) {
	t.Helper()
	res := o.res
	if res.Div != nil {
		ventureFail(t, seed, res)
	}
	if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
	}
}

// cmdCastSeeds are the seeds the lockstep plays.
var cmdCastSeeds = []uint64{1, 2, 3, 4, 5, 6}

func cmdCastSum(t *testing.T, p cmdCastPod, seeds []uint64) dirac.CmdCastStats {
	t.Helper()
	var sum dirac.CmdCastStats
	for _, seed := range seeds {
		o := p.play(t, seed, 40)
		o.check(t, seed)
		sum.Reads += o.stats.Reads
		sum.AtLeast2 += o.stats.AtLeast2
		sum.Max = max(sum.Max, o.stats.Max)
		t.Logf("seed %d: %d decisions, over %v %s; %d read(s), largest %d, gorge's most casts %d", seed, o.res.Decisions, o.res.Over, o.res.Stall, o.stats.Reads, o.stats.Max, o.maxCasts)
	}
	return sum
}

// TestCommanderCastCountFourSeatLockstep plays four-seat Commander pods whose
// decks read Count$TotalCommanderCastFromCommandZone (the last seat's
// commanders a partner pair): no divergence and no stall, and over the seeds
// the kernel reads a count of two or more, so the rising value (not only the
// first cast) matches gorge's log-derived count.
func TestCommanderCastCountFourSeatLockstep(t *testing.T) {
	d := cmdCastPodEntries()
	p := newCmdCastPod(t, d[:], true)
	sum := cmdCastSum(t, p, cmdCastSeeds)
	if sum.Reads == 0 || sum.AtLeast2 == 0 {
		t.Fatalf("reads %d, reads of two or more %d: want both above zero", sum.Reads, sum.AtLeast2)
	}
}

// TestCommanderCastCountTwoSeatLockstep is the commander duel: the same
// decks two at a time, the second duel with the partner pair.
func TestCommanderCastCountTwoSeatLockstep(t *testing.T) {
	d := cmdCastPodEntries()
	var sum dirac.CmdCastStats
	for _, duel := range []struct {
		a, b    int
		partner bool
	}{{0, 1, false}, {2, 3, true}} {
		p := newCmdCastPod(t, [][]cardset.Entry{d[duel.a], d[duel.b]}, duel.partner)
		s := cmdCastSum(t, p, cmdCastSeeds)
		sum.Reads += s.Reads
		sum.AtLeast2 += s.AtLeast2
	}
	if sum.Reads == 0 || sum.AtLeast2 == 0 {
		t.Fatalf("reads %d, reads of two or more %d: want both above zero", sum.Reads, sum.AtLeast2)
	}
}

// TestCommanderCastCountMutation is the mutation test: with only the first
// commander slot read, a partner pair's second commander's casts go
// uncounted, so a partner pod must diverge from gorge's total.
func TestCommanderCastCountMutation(t *testing.T) {
	d := cmdCastPodEntries()
	p := newCmdCastPod(t, d[:], true)
	dirac.DebugCmdCastCountFirstOnly = true
	defer func() { dirac.DebugCmdCastCountFirstOnly = false }()
	for _, seed := range cmdCastSeeds {
		o := p.play(t, seed, 40)
		if o.res.Div != nil {
			t.Logf("seed %d: mutation diverged at %d: %s %s", seed, o.res.Div.At, o.res.Div.What, o.res.Div.Field)
			return
		}
	}
	t.Fatal("reading only the first commander diverged on no seed")
}
