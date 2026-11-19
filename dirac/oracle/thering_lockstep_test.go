package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// ringCounts counts the Ring's events in a finished gorge log.
type ringCounts struct {
	tempts  int // RingTemptsYou actions
	bearers int // of those, ones that designated a creature
	loots   int // level-2 loot pushes (bearer attacked)
	blocks  int // level-3 block pushes (bearer became blocked)
	drains  int // level-4 drain pushes (bearer dealt combat damage to a player)
}

func countRingEvents(g *GorgeSide) (c ringCounts) {
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.RingTemptsYou:
			c.tempts++
			if e.Obj != 0 {
				c.bearers++
			}
		case events.RingEmblemPush:
			switch e.Amount {
			case 2:
				c.loots++
			case 3:
				c.blocks++
			case 4:
				c.drains++
			}
		}
	}
	return c
}

// ringPodEntries is the thering fixture's four decks: tempt makers (Call of
// the Ring, Bilbo, Birthday Escape, Slip On the Ring, Samwise), the "Whenever
// the Ring tempts you" triggers (Nazgûl, Gollum, Claim the Precious,
// Witch-king), the emblem's level 1 evasion and its level 2/3/4 triggers,
// beside cheap attackers and burn.
func ringPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Birthday Escape", Count: 2}, {Name: "Slip On the Ring", Count: 2}, {Name: "Samwise the Stouthearted", Count: 2},
			{Name: "Bilbo, Retired Burglar", Count: 2}, {Name: "Gandalf, Friend of the Shire", Count: 2}, {Name: "Faramir, Field Commander", Count: 2},
			{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Rohirrim Lancer", Count: 2}, {Name: "Uruk-hai Berserker", Count: 2},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 6},
		},
		{
			{Name: "Call of the Ring", Count: 2}, {Name: "Galadriel of Lothlórien", Count: 2}, {Name: "Gollum's Bite", Count: 2}, {Name: "Gollum, Patient Plotter", Count: 2},
			{Name: "Claim the Precious", Count: 2}, {Name: "Took Reaper", Count: 2}, {Name: "Enraged Huorn", Count: 2},
			{Name: "Uruk-hai Berserker", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Shortcut to Mushrooms", Count: 2},
			{Name: "Rohirrim Lancer", Count: 2}, {Name: "Swamp", Count: 14}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Galadriel of Lothlórien", Count: 2}, {Name: "Shortcut to Mushrooms", Count: 2}, {Name: "Dúnedain Rangers", Count: 2},
			{Name: "Stalwarts of Osgiliath", Count: 2}, {Name: "Enraged Huorn", Count: 2}, {Name: "Rohirrim Lancer", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Uruk-hai Berserker", Count: 2}, {Name: "Slip On the Ring", Count: 2},
			{Name: "Samwise the Stouthearted", Count: 2}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 6},
		},
		{
			{Name: "Sauron, the Dark Lord", Count: 2}, {Name: "Claim the Precious", Count: 2}, {Name: "Gollum, Patient Plotter", Count: 2},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Gollum's Bite", Count: 2}, {Name: "Frodo, Adventurous Hobbit", Count: 2},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Slip On the Ring", Count: 2}, {Name: "Birthday Escape", Count: 2},
			{Name: "Swamp", Count: 10}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
		},
	}
}

// ringPod is the fixture's four-seat matchup.
type ringPod struct {
	reg *cards.Registry
	set cardset.Set
	tbl *kop.Table
}

func newRingPod(t *testing.T) ringPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("thering")
	if err != nil {
		t.Fatal(err)
	}
	decks := ringPodEntries()
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return ringPod{reg: reg, set: set, tbl: tbl}
}

// play plays one seed of the four-seat pod and returns the ring events and
// the kernel's ask counters. The lockstep compares the two sides' option
// lists at every decision, including each ring-bearer ask, so the ask order
// (the deterministic default first, as gorge's KChoose poses it) is checked
// decision by decision.
func (p ringPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, ringCounts, dirac.RingStats) {
	t.Helper()
	decks := ringPodEntries()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(decks[0]), expand(decks[1])},
		MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{expand(decks[2]), expand(decks[3])}, Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	st := &dirac.RingStats{}
	dirac.DebugRingStats = st
	defer func() { dirac.DebugRingStats = nil }()
	var c ringCounts
	rc.Finish = func(g *GorgeSide) { c = countRingEvents(g) }
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, c, *st
}

// ringFail reports a seed's divergence with the action window that reached it.
func ringFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

// TestFourSeatRingTemptLockstep plays four-seat pods of the thering fixture:
// the tempt makers, the "Whenever the Ring tempts you" triggers, the
// ring-bearer ask (two or more creatures controlled: the deterministic
// default offered first exactly as gorge's KChoose), the emblem's level-1
// evasion and the level 2/3/4 triggers. No divergence and no stall; the three
// seeds cover a complete game and a 40-turn game, and together they pose the
// ring-bearer ask, loot, block and drain.
func TestFourSeatRingTemptLockstep(t *testing.T) {
	p := newRingPod(t)
	var sum ringCounts
	asks, promises, promiseFires := 0, 0, 0
	for _, seed := range []uint64{1, 2, 14} {
		res, c, st := p.play(t, seed, 40)
		if res.Div != nil {
			ringFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.tempts += c.tempts
		sum.bearers += c.bearers
		sum.loots += c.loots
		sum.blocks += c.blocks
		sum.drains += c.drains
		asks += st.Asks
		promises += st.Promises
		promiseFires += st.PromiseFires
		t.Logf("seed %d: %d decisions, over %v %s; %d tempt(s), %d bearer(s), %d loot, %d block, %d drain, %d ask(s), %d promise(s), %d promise fire(s)", seed, res.Decisions, res.Over, res.Stall,
			c.tempts, c.bearers, c.loots, c.blocks, c.drains, st.Asks, st.Promises, st.PromiseFires)
	}
	t.Logf("totals: %d tempt(s), %d bearer(s), %d loot, %d block, %d drain, %d ask(s), %d promise(s), %d promise fire(s)", sum.tempts, sum.bearers, sum.loots, sum.blocks, sum.drains, asks, promises, promiseFires)
	if sum.tempts == 0 {
		t.Fatal("no game tempted the Ring")
	}
	if asks == 0 {
		t.Fatal("no ring-bearer ask was posed: the default-first option order is untested")
	}
	if sum.loots == 0 {
		t.Fatal("no level-2 loot fired: no Ring-bearer attacked")
	}
	if sum.blocks == 0 {
		t.Fatal("no level-3 promise fired: no Ring-bearer became blocked")
	}
	if promises == 0 || promiseFires == 0 {
		t.Fatalf("level-3 promises registered %d, fired %d: want both above zero", promises, promiseFires)
	}
	if sum.drains == 0 {
		t.Fatal("no level-4 drain fired: no Ring-bearer dealt combat damage")
	}
}

// TestRingTemptLootMutation is the mutation test: the level-2 loot's trigger
// is the only way the attacking Ring-bearer's controller draws, so disabling
// it must diverge.
func TestRingTemptLootMutation(t *testing.T) {
	p := newRingPod(t)
	dirac.DebugRingNoLoot = true
	defer func() { dirac.DebugRingNoLoot = false }()
	res, _, _ := p.play(t, 1, 40)
	if res.Div == nil {
		t.Fatal("disabling the Ring emblem's level-2 loot diverged on no decision")
	}
	t.Logf("mutation diverged at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)
}

// TestRingTemptPinnedRepro pins seed 14 and the ring events it reaches: the
// seeded game's temptation count, its bearer designations, the level-2 loots
// and the level-4 drain, plus the ring-bearer asks the default-first order is
// checked through.
func TestRingTemptPinnedRepro(t *testing.T) {
	p := newRingPod(t)
	res, c, st := p.play(t, 14, 40)
	if res.Div != nil {
		ringFail(t, 14, res)
	}
	want := ringCounts{tempts: 21, bearers: 14, loots: 6, blocks: 1, drains: 5}
	if c != want {
		t.Fatalf("seed 14 ring events = %+v, want %+v", c, want)
	}
	if st.Asks != 1 || st.Promises != 1 || st.PromiseFires != 1 {
		t.Fatalf("seed 14 asks/promises/fires = %d/%d/%d, want 1/1/1", st.Asks, st.Promises, st.PromiseFires)
	}
	t.Logf("seed 14: %d decisions, ring events %+v, asks %d, promises %d/%d", res.Decisions, want, st.Asks, st.Promises, st.PromiseFires)
}
