package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Two Remembered reads kgen refused as "written by another ability" whose
// writer is the same resolution's (dirac/kgen lower_remembered_reflex.go,
// lower_remembered_writers.go), in four-seat pods:
//
//   - a reflexive trigger's body reads the parent's set: Calibrated Blast and
//     Yuna's Whistle's ImmediateTrigger (no RememberObjects$) deals damage /
//     puts counters equal to the mana value of the card DigUntil
//     RememberFound$ revealed. gorge hands the instance the parent's set
//     (effImmediateTrigger wholeSet); the kernel captures the value at the
//     parent (OpReflexiveTrigger B, the entry's trigAmt).
//   - Hostage Taker's O-Ring exile with RememberChanged$: the next link's
//     Effect MayPlay reads the exiled card. OpExileUntilLeaves C MvRemember
//     remembers it.

// remPodDeck is a 40-card deck of spell (count n) with a creature curve.
func remPodDeck(spell string, n int, lands ...string) []cardset.Entry {
	d := []cardset.Entry{{Name: spell, Count: n}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 8 - n/2}}
	rest := 40 - n - 8 - (8 - n/2)
	for i, l := range lands {
		c := rest / len(lands)
		if i == 0 {
			c += rest % len(lands)
		}
		d = append(d, cardset.Entry{Name: l, Count: c})
	}
	return d
}

// remPodRun plays seeds four-seat games of each pod in lockstep on a table
// built for the pods' cards (mutate edits it first; nil plays it as built)
// and returns the divergences, the kernel stalls and the watched casts.
func remPodRun(t *testing.T, label string, pods [][4][]cardset.Entry, watch map[string]bool, seeds int, mutate func(*kop.Table)) (divs, stalls int, casts map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var pool []*cards.Card
	for _, pod := range pods {
		for _, d := range pod {
			for _, e := range d {
				if seen[e.Name] {
					continue
				}
				seen[e.Name] = true
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				pool = append(pool, c)
			}
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, label)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	sup := set.Supported()
	for n := range watch {
		if i, ok := set.Lookup(n); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the %s table does not play %s: %s", label, n, set.Reason(i))
		}
	}
	casts = map[string]int{}
	for pi, pod := range pods {
		tbl, ids, err := set.MatchupN(pod[:])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			m, err := KernelMatchupN(reg, tbl, []string{"r0", "r1", "r2", "r3"}, ids, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			rc := RunConfig{MaxDecisions: 3000, MaxTurns: 30}
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("pod %d seed %d: %v", pi, seed, err)
			}
			res := Lockstep(m, seed, k, fixtureWatchPolicy(seed, watch, casts), rc)
			CloseSide(k)
			if d := res.Div; d != nil {
				divs++
				if mutate == nil {
					t.Errorf("pod %d seed %d: diverged at decision %d: %s %s %v %s", pi, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				}
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if mutate == nil {
					t.Errorf("pod %d seed %d: kernel stopped: %s (decision %d, turn %d, casts %v)", pi, seed, res.Stall, res.Decisions, res.Turns, casts)
				}
			}
		}
	}
	return divs, stalls, casts
}

var remReflexWatch = map[string]bool{"Calibrated Blast": true, "Yuna's Whistle": true}

func remReflexPods() [][4][]cardset.Entry {
	blast := remPodDeck("Calibrated Blast", 8, "Mountain")
	whistle := remPodDeck("Yuna's Whistle", 8, "Forest")
	return [][4][]cardset.Entry{{blast, whistle, blast, whistle}, {whistle, blast, whistle, blast}}
}

// TestRemReflexParentSetFourSeatLockstep: the reflexive bodies play in
// lockstep at four seats.
func TestRemReflexParentSetFourSeatLockstep(t *testing.T) {
	_, _, casts := remPodRun(t, "remreflex-oracle", remReflexPods(), remReflexWatch, 12, nil)
	t.Logf("casts %v", casts)
	for n := range remReflexWatch {
		if casts[n] < 5 {
			t.Errorf("%s cast %d times; want 5 or more", n, casts[n])
		}
	}
}

// TestRemReflexParentSetMutation: with the captured amount zeroed the body
// deals no damage and puts no counters, so the kernel diverges from gorge.
func TestRemReflexParentSetMutation(t *testing.T) {
	divs, _, _ := remPodRun(t, "remreflex-oracle", remReflexPods(), remReflexWatch, 12, func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				prog := tb.Cards[i].Abilities[j].Prog
				for k := range prog {
					if prog[k].Op == kop.OpReflexiveTrigger {
						prog[k].B = kop.Const(0)
					}
				}
			}
		}
	})
	if divs == 0 {
		t.Errorf("no game diverged without the captured parent set: the pool does not reach the read")
	}
}

var remExileUntilWatch = map[string]bool{"Hostage Taker": true, "Lightning Bolt": true}

func remExileUntilPods() [][4][]cardset.Entry {
	// four Takers a deck and burn that kills them, so the exiled card
	// returns, is cast from exile or stays: an all-Taker pod chains returns
	// inside one turn until the permission table is full
	taker := remPodDeck("Hostage Taker", 4, "Island", "Swamp")
	bolt := remPodDeck("Lightning Bolt", 6, "Mountain", "Forest")
	return [][4][]cardset.Entry{{taker, bolt, taker, bolt}, {bolt, taker, bolt, taker}}
}

// TestRemExileUntilRememberFourSeatLockstep: Hostage Taker's exile and the
// MayPlay over the remembered card play in lockstep at four seats.
func TestRemExileUntilRememberFourSeatLockstep(t *testing.T) {
	_, _, casts := remPodRun(t, "remexileuntil-oracle", remExileUntilPods(), remExileUntilWatch, 16, nil)
	t.Logf("casts %v", casts)
	if casts["Hostage Taker"] < 10 {
		t.Errorf("Hostage Taker cast %d times; want 10 or more", casts["Hostage Taker"])
	}
}

// TestRemExileUntilRememberMutation: without the remember the MayPlay names
// no card, so the exiled creature is never castable and the kernel diverges.
func TestRemExileUntilRememberMutation(t *testing.T) {
	divs, _, _ := remPodRun(t, "remexileuntil-oracle", remExileUntilPods(), remExileUntilWatch, 16, func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				prog := tb.Cards[i].Abilities[j].Prog
				for k := range prog {
					if prog[k].Op == kop.OpExileUntilLeaves {
						prog[k].C &^= kop.MvRemember
					}
				}
			}
		}
	})
	if divs == 0 {
		t.Errorf("no game diverged without the remembered exile: the pool does not reach the MayPlay")
	}
}
