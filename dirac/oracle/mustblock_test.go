package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The must-be-blocked lockstep: "CARDNAME must be
// blocked if able." printed (Fear of Being Hunted, Vinebred Brawler,
// Raphael) and granted until end of turn (Joraga Invocation's PumpAll,
// Magitek Scythe's Pump, Disturbed Slumber's Animate), beside a lure and a
// menace creature. gorge accepts only a declaration that meets the most CR
// 509.1c requirements (decision.blockRequiredQuota); the kernel must offer
// the same pairs at every pick and Done at the same point.

var kmustblockCarriers = []cardset.Entry{
	{Name: "Fear of Being Hunted", Count: 2}, {Name: "Vinebred Brawler", Count: 2}, {Name: "Raphael, Ninja Destroyer", Count: 2},
	{Name: "Joraga Invocation", Count: 4}, {Name: "Magitek Scythe", Count: 3}, {Name: "Disturbed Slumber", Count: 2},
	{Name: "Prized Unicorn", Count: 2}, {Name: "Boggart Brute", Count: 3}, {Name: "Llanowar Elves", Count: 2},
	{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8},
}

var kmustblockDecks = [][2][]cardset.Entry{{
	kmustblockCarriers,
	{
		{Name: "Grizzly Bears", Count: 7}, {Name: "Hill Giant", Count: 5}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Giant Growth", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Prized Unicorn", Count: 2}, {Name: "Forest", Count: 7}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 2},
	},
}, {kmustblockCarriers, kmustblockCarriers}}

type kmustblockStats struct {
	games, divs, stalls int
	decls               int // block declarations with a must-be-blocked attacker offered
	short               int // two must-be-blocked attackers, fewer able blockers than attackers
	compete             int // a blocker offered a lure block and a must-be-blocked block on another attacker
	expired             int // a block pick against an attacker marked an earlier turn, no longer must-be-blocked
}

// kmustblockPolicy plays casts, attacks and blocks whenever it can (so the
// games reach the declarations), else random. It counts, from gorge's
// options, the declarations the requirement search decides.
func kmustblockPolicy(seed uint64, st *kmustblockStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	marked := map[state.ObjID]int{} // attacker -> turn it was last seen must-be-blocked (granted or printed)
	printed := map[state.ObjID]bool{}
	prevBlock := false
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActBlock:
				buf = append(buf, a)
			}
		}
		a := c.D.Options[0]
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G != nil && c.D.Kind == DBlockers {
			if !prevBlock {
				kmustblockCount(c, st, marked, printed)
			}
			prevBlock = a.Kind() == ActBlock
		} else {
			prevBlock = false
		}
		return a
	})
}

func kmustblockCount(c *PickCtx, st *kmustblockStats, marked map[state.ObjID]int, printed map[state.ObjID]bool) {
	turn := c.G.Turn()
	mbAtk := map[state.ObjID]bool{}
	mbBlk := map[state.ObjID]bool{}
	lureBlk := map[state.ObjID]bool{}
	plainMB := map[state.ObjID]bool{} // blockers offered a must-be-blocked attacker
	seen := map[state.ObjID]bool{}
	for _, x := range c.D.Options {
		if x.Kind() != ActBlock {
			continue
		}
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		seen[o.Attacker] = true
		if o.AttackMust {
			mbAtk[o.Attacker] = true
			mbBlk[o.Obj] = true
			plainMB[o.Obj] = true
			if _, was := marked[o.Attacker]; !was {
				if ob := c.G.E.G.Obj(o.Attacker); ob != nil && ob.Face() != nil && kmustblockPrinted[ob.Face().Name] {
					printed[o.Attacker] = true
				}
			}
			marked[o.Attacker] = turn
		}
		if o.BlockMust && !o.AttackMust {
			lureBlk[o.Obj] = true
		}
	}
	for a := range seen {
		if t, ok := marked[a]; ok && t < turn && !mbAtk[a] && !printed[a] {
			st.expired++
			delete(marked, a)
		}
	}
	if len(mbAtk) > 0 {
		st.decls++
	}
	if len(mbAtk) >= 2 && len(mbBlk) < len(mbAtk) {
		st.short++
	}
	for b := range lureBlk {
		if plainMB[b] {
			st.compete++
			break
		}
	}
}

// kmustblockPrinted names the printed carriers (always must be blocked).
var kmustblockPrinted = map[string]bool{"Fear of Being Hunted": true, "Vinebred Brawler": true, "Raphael, Ninja Destroyer": true}

func runMustblock(t *testing.T, seeds int, greedy bool) kmustblockStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mustblock")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugMustBlockGreedy
	dirac.DebugMustBlockGreedy = greedy
	defer func() { dirac.DebugMustBlockGreedy = old }()
	var st kmustblockStats
	for mi, md := range kmustblockDecks {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmustblock-a", "kmustblock-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kmustblockPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !greedy {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !greedy {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestMustblockLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runMustblock(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls; %d declarations with a must-be-blocked attacker, %d short of blockers, %d lure/must-be-blocked competing, %d expired marks seen", st.games, st.divs, st.stalls, st.decls, st.short, st.compete, st.expired)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	for _, c := range []struct {
		n    int
		what string
	}{
		{st.short, "declarations with two must-be-blocked attackers and fewer able blockers than attackers"},
		{st.compete, "declarations where a lure and a must-be-blocked attacker compete for a blocker"},
		{st.expired, "block picks against an attacker whose granted mark expired"},
	} {
		if c.n < 5 {
			t.Errorf("only %d %s, want >= 5", c.n, c.what)
		}
	}
}

// TestMustblockMutation offers Done once any one requirement is met
// instead of the maximum: the games must diverge.
func TestMustblockMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runMustblock(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left Done at any one requirement in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
