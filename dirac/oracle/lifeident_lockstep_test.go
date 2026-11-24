package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lifeIdentPodEntries is the lifeident fixture's four 40-card decks. Each
// opens with its commander (a partner pair in the last two) and holds four
// War Rooms beside cheap creatures and removal and 24 basic lands, so the
// {3}, {T} draw is affordable from the middle turns on. The seats' commander
// colour identities count 0 (Karn, Silver Golem), 1 (Isamaru, Hound of
// Konda), 2 (Tymna the Weaver and Ravos, Soultender: both white-black, so a
// sum over the pair would be 4) and 3 (Rograkh, Son of Rohgahh and Tymna).
func lifeIdentPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Karn, Silver Golem", Count: 1}, {Name: "War Room", Count: 4}, {Name: "Savannah Lions", Count: 4},
			{Name: "Raging Goblin", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Plains", Count: 12}, {Name: "Mountain", Count: 12},
		},
		{
			{Name: "Isamaru, Hound of Konda", Count: 1}, {Name: "War Room", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Lightning Bolt", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
		},
		{
			{Name: "Tymna the Weaver", Count: 1}, {Name: "Ravos, Soultender", Count: 1}, {Name: "War Room", Count: 4},
			{Name: "Walking Corpse", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Doom Blade", Count: 2},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 12},
		},
		{
			{Name: "Rograkh, Son of Rohgahh", Count: 1}, {Name: "Tymna the Weaver", Count: 1}, {Name: "War Room", Count: 4},
			{Name: "Raging Goblin", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Shock", Count: 2},
			{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
		},
	}
}

// lifeIdentPod is a Commander game over the lifeident fixture. partners[i]
// says whether seat i's deck opens with a partner pair.
type lifeIdentPod struct {
	reg      *cards.Registry
	set      cardset.Set
	decks    [][]cardset.Entry
	partners []bool
}

func newLifeIdentPod(t *testing.T, decks [][]cardset.Entry, partners []bool) lifeIdentPod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table read
	set, err := cardset.RawByName("lifeident")
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
	return lifeIdentPod{reg: reg, set: set, decks: decks, partners: partners}
}

// repro is the pod as a Repro: every seat's commander is deck index 0 and a
// partner is index 1.
func (p lifeIdentPod) repro(seed uint64) *Repro {
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
	r.Partners = make([]int, len(p.decks))
	for i := range r.Partners {
		r.Partners[i] = -1
		if p.partners[i] {
			r.Partners[i] = 1
		}
	}
	for s := 2; s < len(p.decks); s++ {
		r.MoreDecks = append(r.MoreDecks, "c"+string(rune('0'+s)))
		r.MoreCards = append(r.MoreCards, expand(p.decks[s]))
	}
	return r
}

// play plays one seed in lockstep, gorge leading, and reads back the life
// cost payments the kernel made.
func (p lifeIdentPod) play(t *testing.T, seed uint64) (Result, dirac.LifeIdentStats) {
	t.Helper()
	r := p.repro(seed)
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, 40
	st := &dirac.LifeIdentStats{}
	dirac.DebugLifeIdentStats = st
	defer func() { dirac.DebugLifeIdentStats = nil }()
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), *st
}

// lifeIdentSeeds are the seeds the lockstep plays.
var lifeIdentSeeds = []uint64{1, 2, 3, 4, 5, 6}

// lifeIdentSum plays every seed, fails on a divergence or a kernel stop, and
// sums the payments.
func lifeIdentSum(t *testing.T, p lifeIdentPod) dirac.LifeIdentStats {
	t.Helper()
	var sum dirac.LifeIdentStats
	for _, seed := range lifeIdentSeeds {
		res, st := p.play(t, seed)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.Pays += st.Pays
		for n, c := range st.ByCount {
			sum.ByCount[n] += c
		}
		t.Logf("seed %d: %d decisions, over %v %s; payments by life charged %v", seed, res.Decisions, res.Over, res.Stall, st.ByCount)
	}
	return sum
}

// TestLifeIdentFourSeatLockstep plays four-seat Commander pods in which every
// seat holds War Rooms and the seats' commander colour identities count 0, 1,
// 2 and 3: no divergence and no stall, and over the seeds the kernel charges
// each of those amounts at least once, so gorge's per-payer count (the union
// over a partner pair) is matched at every value the fixture can produce.
func TestLifeIdentFourSeatLockstep(t *testing.T) {
	d := lifeIdentPodEntries()
	p := newLifeIdentPod(t, d[:], []bool{false, false, true, true})
	sum := lifeIdentSum(t, p)
	for n := 0; n <= 3; n++ {
		if sum.ByCount[n] == 0 {
			t.Errorf("no payment of %d life in %d payments %v", n, sum.Pays, sum.ByCount)
		}
	}
	if sum.ByCount[4]+sum.ByCount[5] != 0 {
		t.Errorf("payments above 3 life: %v", sum.ByCount)
	}
}

// TestLifeIdentTwoSeatLockstep is the commander duel: the same decks two at a
// time, the second duel between the two partner pairs.
func TestLifeIdentTwoSeatLockstep(t *testing.T) {
	d := lifeIdentPodEntries()
	var sum dirac.LifeIdentStats
	for _, duel := range []struct {
		a, b     int
		partners []bool
	}{{0, 1, []bool{false, false}}, {2, 3, []bool{true, true}}} {
		p := newLifeIdentPod(t, [][]cardset.Entry{d[duel.a], d[duel.b]}, duel.partners)
		s := lifeIdentSum(t, p)
		sum.Pays += s.Pays
		for n, c := range s.ByCount {
			sum.ByCount[n] += c
		}
	}
	if sum.Pays == 0 || sum.ByCount[2]+sum.ByCount[3] == 0 {
		t.Fatalf("payments %d by life charged %v: want some, and some of two or three life", sum.Pays, sum.ByCount)
	}
}

// TestLifeIdentLockstepMutation is the mutation test: with the life part
// charging nothing, a seat with a coloured commander keeps life gorge takes,
// so some seed must diverge.
func TestLifeIdentLockstepMutation(t *testing.T) {
	d := lifeIdentPodEntries()
	p := newLifeIdentPod(t, d[:], []bool{false, false, true, true})
	dirac.DebugLifeIdentFree = true
	defer func() { dirac.DebugLifeIdentFree = false }()
	for _, seed := range lifeIdentSeeds {
		res, _ := p.play(t, seed)
		if res.Div != nil {
			t.Logf("seed %d: mutation diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("a free life part diverged on no seed")
}
