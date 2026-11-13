package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep (CR 700.13): constructed matchups over the
// generated crime pool. Carriers: trig:CommitCrime (Blood Hustler, Vadmir,
// Raven of Fell Omens, Forsaken Miner (from the graveyard), Rattleback
// Apothecary, Overzealous Muscle, Hardbristle Bandit, Freestrider Lookout,
// Magda, Duelist of the Mind, Deepmuck Desperado, Marauding Sphinx,
// Marchesa, Patrolling Peacemaker (the opponent's crimes)), Count$
// CommittedCrimeThisTurn (Nimble Brigand, Slickshot Vault-Buster, Omenport
// Vigilante, Seize the Secrets) and Servant of the Stinger's
// CheckDefinedPlayer$. Enablers: the deserts' ETB, Deadeye Duelist, Blood
// Hustler's ability, Burst Lightning, Slick Sequence, Bite Down, Essence
// Scatter, Soul-Guide Lantern, Soul-Shackled Zombie.
//
// Bandit's Haul is not in the pool: its LOOT counter kind is outside the
// pool's counter set, so kgen refuses it there (a second refusal); Marauding
// Sphinx takes its slot.
//
// Bow of Nylea, Cankerbloom and Goblin Cratermaker are in the B/R/G deck for the
// modal activated abilities: gorge announces no mode at activation, the kernel
// poses modes and mode-bound targets at resolution (modal_activated.go).

var kcrimeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Blood Hustler", Count: 2}, {Name: "Vadmir, New Blood", Count: 1}, {Name: "Raven of Fell Omens", Count: 1},
		{Name: "Forsaken Miner", Count: 2}, {Name: "Rattleback Apothecary", Count: 1}, {Name: "Overzealous Muscle", Count: 1},
		{Name: "Servant of the Stinger", Count: 1}, {Name: "Soul-Shackled Zombie", Count: 1}, {Name: "Freestrider Lookout", Count: 1}, {Name: "Magda, the Hoardmaster", Count: 1}, {Name: "Deadeye Duelist", Count: 2},
		{Name: "Heartfire Hero", Count: 1}, {Name: "Hardbristle Bandit", Count: 3}, {Name: "Burst Lightning", Count: 2},
		{Name: "Bite Down", Count: 2}, {Name: "Bow of Nylea", Count: 2}, {Name: "Cankerbloom", Count: 2}, {Name: "Goblin Cratermaker", Count: 2}, {Name: "Jagged Barrens", Count: 2}, {Name: "Bristling Backwoods", Count: 2},
		{Name: "Festering Gulch", Count: 2}, {Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 1},
	},
	{
		{Name: "Duelist of the Mind", Count: 2}, {Name: "Deepmuck Desperado", Count: 2}, {Name: "Marauding Sphinx", Count: 2},
		{Name: "Nimble Brigand", Count: 2}, {Name: "Slickshot Vault-Buster", Count: 2}, {Name: "Seize the Secrets", Count: 2},
		{Name: "Omenport Vigilante", Count: 2}, {Name: "Patrolling Peacemaker", Count: 2}, {Name: "Marchesa, Dealer of Death", Count: 1},
		{Name: "Essence Scatter", Count: 2}, {Name: "Soul-Guide Lantern", Count: 1}, {Name: "Slick Sequence", Count: 1},
		{Name: "Take Up the Shield", Count: 1}, {Name: "Lonely Arroyo", Count: 2}, {Name: "Soured Springs", Count: 2},
		{Name: "Eroded Canyon", Count: 2}, {Name: "Forlorn Flats", Count: 1}, {Name: "Island", Count: 7},
		{Name: "Plains", Count: 3}, {Name: "Swamp", Count: 1},
	},
}}

// kcrimeMatchups is the two decks in both seats.
func kcrimeMatchups() [][2][]cardset.Entry {
	a, b := kcrimeDecks[0][0], kcrimeDecks[0][1]
	return [][2][]cardset.Entry{{a, b}, {b, a}}
}

// kcrimePrefer names the cards the carrier-preferring policy plays first.
var kcrimePrefer = map[string]bool{
	"Blood Hustler": true, "Vadmir, New Blood": true, "Raven of Fell Omens": true, "Forsaken Miner": true,
	"Rattleback Apothecary": true, "Overzealous Muscle": true, "Servant of the Stinger": true, "Hardbristle Bandit": true,
	"Freestrider Lookout": true, "Magda, the Hoardmaster": true, "Duelist of the Mind": true, "Deepmuck Desperado": true,
	"Marauding Sphinx": true, "Marchesa, Dealer of Death": true, "Patrolling Peacemaker": true, "Nimble Brigand": true,
	"Slickshot Vault-Buster": true, "Omenport Vigilante": true, "Seize the Secrets": true, "Deadeye Duelist": true,
	"Soul-Shackled Zombie": true, "Soul-Guide Lantern": true, "Bite Down": true, "Burst Lightning": true, "Bow of Nylea": true, "Cankerbloom": true, "Goblin Cratermaker": true,
	"Essence Scatter": true, "Heartfire Hero": true, "Slick Sequence": true, "Take Up the Shield": true,
}

// kcrimeCarriers are the cards each of which must be played kcrimeMinPlays
// times (cast or activated).
var kcrimeCarriers = []string{
	"Blood Hustler", "Forsaken Miner", "Deadeye Duelist", "Burst Lightning", "Bite Down",
	"Duelist of the Mind", "Deepmuck Desperado", "Marauding Sphinx", "Nimble Brigand", "Slickshot Vault-Buster",
	"Seize the Secrets", "Omenport Vigilante", "Patrolling Peacemaker", "Marchesa, Dealer of Death", "Essence Scatter",
	"Soul-Guide Lantern", "Soul-Shackled Zombie", "Heartfire Hero", "Slick Sequence", "Take Up the Shield",
	"Bow of Nylea", "Cankerbloom", "Goblin Cratermaker",
}

const kcrimeMinPlays = 10

// kcrimeGS is the last gorge side the policy saw (a divergence's context).
var kcrimeGS *GorgeSide

// kcrimePolicy plays odd seeds at random and even seeds carrier-first (a
// cast or activation of a listed card when one is offered; the target
// among the legal ones at random; no legal action is ever avoided
// outright). It compares gorge's crime ledger with the kernel's at every
// frame where gorge leads, and counts Slickshot Vault-Buster's +2/+0 in
// gorge's derived power.
func kcrimePolicy(seed uint64, played map[string]int, kb *[2]bool, parity *int) Policy {
	inner := eagerPolicy(seed, played)
	rnd := RandomPolicy(seed ^ 0x6372696d65)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			kcrimeGS = g
			for p := state.PlayerID(0); p < 2; p++ {
				if g.E.CommittedCrimeThisTurn(p) != kb[p] {
					*parity++
				}
			}
			if c.D.Kind == DPriority {
				for i := range g.E.G.Objs {
					ob := &g.E.G.Objs[i]
					if ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Slickshot Vault-Buster" && g.E.Power(state.ObjID(i+1)) >= 3 {
						played["slickshot+2"]++
					}
				}
			}
		}
		if seed%2 == 1 || c.G == nil {
			return inner.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kcrimePrefer[ob.Face().Name] {
						buf = append(buf, a)
					}
				}
			}
		}
		if len(buf) == 0 {
			return inner.Pick(c)
		}
		d := *c.D
		d.Options = buf
		a := rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				played[ob.Face().Name]++
			}
		}
		return a
	})
}

func kcrimeSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	reg, _ := kcastSetup(t)
	set, err := cardset.RawByName("crime")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kcrimeTally is one chunk's (or the run's) counts.
type kcrimeTally struct {
	Games, Parity int
	Played        map[string]int
	Stalls        map[string]int
	St            dirac.CrimeStats
}

// kcrimeChunk plays one chunk of TestCrimeLockstep's games, stopping at its
// first divergence.
func kcrimeChunk(t *testing.T, c seedChunk) kcrimeTally {
	t.Helper()
	reg, set := kcrimeSetup(t)
	tl := kcrimeTally{Played: map[string]int{}, Stalls: map[string]int{}}
	dirac.DebugCrimeStats = &tl.St
	defer func() { dirac.DebugCrimeStats = nil }()
	md := kcrimeMatchups()[c.mi]
	decks := kcastDeck(t, reg, md)
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := c.lo; seed <= c.hi; seed++ {
		res := kcrimeGame(t, reg, set, tbl, decks, seed, tl.Played, &tl.Parity)
		tl.Games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			tl.Stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("matchup %d seed %d last actions %v", c.mi, seed, actionStrings(res.Actions[lo:d.At]))
			kcastDump(t, kcrimeGS)
			t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", c.mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			break
		}
	}
	return tl
}

// kcrimeGame plays one seeded game of matchup md under the policy.
func kcrimeGame(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, decks [2][]*cards.Card, seed uint64, played map[string]int, parity *int) Result {
	t.Helper()
	cfg := Config{GorgeAutopaySurface: seed%4 < 2}
	m, err := NewMatchup(reg, [2]string{"kcrime-a", "kcrime-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	var kb [2]bool
	OccupancyHook = func(s *dirac.State) { kb = [2]bool{s.CrimeLedger(0), s.CrimeLedger(1)} }
	defer func() { OccupancyHook = nil }()
	return Lockstep(m, seed, k, kcrimePolicy(seed, played, &kb, parity), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
}

func TestCrimeLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip here, before the chunks (each would
	// skip alone and leave this test's totals empty).
	corpusDirOrSkip(t)
	t.Parallel()
	// Each chunk plays alone in a child process (it sets DebugCrimeStats and
	// OccupancyHook, which every game reads) and logs its tally; the parent
	// sums them and checks the floors over the whole run.
	var tot kcrimeTally
	matchupChunks(t, len(kcrimeMatchups()), 1, 1024, 256, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		var part kcrimeTally
		if isolateTally(t, &part) {
			mu.Lock()
			addTally(&tot, &part)
			mu.Unlock()
			return
		}
		part = kcrimeChunk(t, c)
		logTally(t, &part)
	})
	if isolatedChild() || t.Failed() {
		return
	}
	st, games, played, stalls, parity := &tot.St, tot.Games, tot.Played, tot.Stalls, tot.Parity
	st.Parity = parity
	t.Logf("%d games, stalls %v, plays %v", games, stalls, played)
	t.Logf("crime stats: %+v", *st)
	for _, n := range kcrimeCarriers {
		if played[n] < kcrimeMinPlays {
			t.Errorf("carrier %s played %d times, want at least %d", n, played[n], kcrimeMinPlays)
		}
	}
	for name, v := range map[string]int{
		"BySpell": st.BySpell, "ByAbility": st.ByAbility, "ByTrigger": st.ByTrigger, "NonCrime": st.NonCrime, "Fires": st.Fires,
		"MultiCrime": st.MultiCrime, "ModalAbility": st.ModalAbility, "CountTrue": st.CountTrue, "CountFalse": st.CountFalse, "slickshot+2": played["slickshot+2"],
	} {
		if v < 10 {
			t.Errorf("%s = %d, want at least 10", name, v)
		}
	}
	for name, v := range map[string]int{
		"GraveCrime": st.GraveCrime, "StackCrime": st.StackCrime, "OppListener": st.OppListener, "GraveListener": st.GraveListener,
		"ValiantCrime": st.ValiantCrime,
	} {
		if v < 3 {
			t.Errorf("%s = %d, want at least 3", name, v)
		}
	}
	if st.ReversedQueued != 0 {
		t.Errorf("%d casts reversed with a crime trigger queued", st.ReversedQueued)
	}
	if parity != 0 {
		t.Errorf("%d ledger parity mismatches against gorge", parity)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestCrimeMutation: each debug switch must diverge on some seed.
func TestCrimeMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set := kcrimeSetup(t)
	for _, sw := range []struct {
		name string
		flag *bool
	}{
		{"AnyTarget", &dirac.DebugCrimeAnyTarget},
		{"EachTarget", &dirac.DebugCrimeEachTarget},
		{"NoReset", &dirac.DebugCrimeNoReset},
		{"ModalTwice", &dirac.DebugCrimeModalTwice},
	} {
		t.Run(sw.name, func(t *testing.T) {
			*sw.flag = true
			defer func() { *sw.flag = false }()
			for mi, md := range kcrimeMatchups() {
				decks := kcastDeck(t, reg, md)
				tbl, _, _, err := set.Matchup(md[0], md[1])
				if err != nil {
					t.Fatal(err)
				}
				for seed := uint64(1); seed <= 96; seed++ {
					parity := 0
					res := kcrimeGame(t, reg, set, tbl, decks, seed, map[string]int{}, &parity)
					if res.Div != nil || parity != 0 {
						t.Logf("matchup %d seed %d: diverged (%v), parity mismatches %d", mi, seed, res.Div != nil, parity)
						return
					}
				}
			}
			t.Fatal("debug switch did not diverge")
		})
	}
}
