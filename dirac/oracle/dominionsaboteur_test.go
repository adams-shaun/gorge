package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Dominion Saboteur: "You may have this creature enter as a copy of any
// artifact or creature on the battlefield, except it isn't legendary. If you
// do, it enters with additional counters on it equal to the same number and
// kinds of counters the copied permanent has on it." The ETB clone's
// RememberCloneOrigin$ origin is read by the chained PutCounter's
// CounterType$ EachFromSource | EachFromSource$ Remembered; the kernel folds
// the chain into OpBecomeCopy's BecomeCopyCtr flag, so the counters are on
// the entering copy before its enters trigger matches (CR 707.9b, 614.12).
//
// The deck enters with a +1/+1 counter (Kalonian Hydra), with charge counters
// (Surge Node), as a legendary creature (Isamaru, Hound of Konda), and as a
// plain creature (Grizzly Bears) so the election's decline is exercised too.
var kdominionSaboteurDecks = [2][]cardset.Entry{
	{
		{Name: "Dominion Saboteur", Count: 4},
		{Name: "Kalonian Hydra", Count: 4},
		{Name: "Surge Node", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 14},
		{Name: "Forest", Count: 13},
		{Name: "Plains", Count: 13},
	},
	{
		{Name: "Plains", Count: 20}, {Name: "Island", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// kdominionSaboteurCounts is gorge's evidence that a scenario ran: copies of
// the counter carriers, a non-legendary copy of the legendary, and declines.
type kdominionSaboteurCounts struct {
	p1p1, charge, legendary, decline int
}

// kdominionSaboteurRun plays seeds 1..n against the fixture's table, with
// mutate applied to a copy of it when non-nil. It returns the number of
// diverging games and gorge's scenario counts.
func kdominionSaboteurRun(t *testing.T, n uint64, mutate func(*kop.Table), report bool) (int, kdominionSaboteurCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dominionsaboteur")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kdominionSaboteurDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kdominionSaboteurDecks[0], kdominionSaboteurDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mut := *tbl
		mutate(&mut)
		tbl = &mut
	}
	var div int
	var cnt kdominionSaboteurCounts
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"ds-a", "ds-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				switch ev.Kind {
				case events.Choose:
					if ev.Counter == "clone" && len(ev.IDs) == 0 {
						cnt.decline++
					}
				case events.ClonePermanent:
					if len(ev.IDs) == 0 {
						continue
					}
					src := ref.E.G.Obj(ev.IDs[0])
					dst := ref.E.G.Obj(ev.Obj)
					if src == nil || dst == nil || src.Face() == nil {
						continue
					}
					switch src.Face().Name {
					case "Kalonian Hydra", "Surge Node":
						// CR 707.5: the template's printed enters-with
						// counters (four +1/+1, six charge) are on the copy
						// before the clone event; CR 707.9b: the remembered
						// origin's counters follow as the rider. The origin
						// may have spent its own since, so the rider is only
						// required to be positive (the lockstep projection is
						// the exact comparison).
						kind, printed := "P1P1", 4
						if src.Face().Name == "Surge Node" {
							kind, printed = "CHARGE", 6
						}
						// before: the counters placed on the copy ahead of the
						// clone event; after: the first placement behind it.
						before, after := 0, 0
						for x := range evs[:seen] {
							if e := &evs[x]; e.Kind == events.CounterChange && e.Obj == dst.ID && e.Counter == kind {
								before += int(e.Amount)
							}
						}
						for x := seen + 1; x < len(evs); x++ {
							if e := &evs[x]; e.Kind == events.CounterChange && e.Obj == dst.ID && e.Counter == kind {
								after = int(e.Amount)
								break
							}
						}
						if before != printed && report {
							t.Errorf("seed %d: %s copy entered with %d printed %s counters, want %d", seed, src.Face().Name, before, kind, printed)
						}
						if before == printed && after > 0 {
							if kind == "P1P1" {
								cnt.p1p1++
							} else {
								cnt.charge++
							}
						}
					case "Isamaru, Hound of Konda":
						if !slices.Contains(ref.E.Derived(dst.ID).Types, "Legendary") {
							cnt.legendary++
						} else if report {
							t.Errorf("seed %d: Isamaru copy is still legendary", seed)
						}
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, cnt
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, cnt
}

// TestDominionSaboteurLockstep: 200 games, 0 divergences, and every scenario
// (a counter creature, a charge artifact, a legendary template, a decline)
// actually occurred.
func TestDominionSaboteurLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, cnt := kdominionSaboteurRun(t, 200, nil, true)
	t.Logf("200 games: %d divergences; copies: +1/+1 %d, charge %d, nonlegendary Isamaru %d; declines %d",
		div, cnt.p1p1, cnt.charge, cnt.legendary, cnt.decline)
	if div != 0 {
		t.Errorf("%d diverging games; want 0", div)
	}
	if cnt.p1p1 < 1 || cnt.charge < 1 || cnt.legendary < 1 || cnt.decline < 1 {
		t.Errorf("scenarios %+v; want at least one of each", cnt)
	}
}

// TestDominionSaboteurMutation: with every OpBecomeCopy's BecomeCopyCtr flag
// cleared (the origin's counters not copied), some seed diverges.
func TestDominionSaboteurMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	div, _ := kdominionSaboteurRun(t, 100, func(tbl *kop.Table) {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		n := 0
		for i := range cs {
			abs := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpBecomeCopy && p[x].B.N&kop.BecomeCopyCtr != 0 {
						p[x].B.N &^= kop.BecomeCopyCtr
						n++
					}
				}
				abs[j].Prog = p
			}
			cs[i].Abilities = abs
		}
		if n == 0 {
			t.Fatal("no OpBecomeCopy carries the BecomeCopyCtr flag")
		}
		tbl.Cards = cs
	}, false)
	if div == 0 {
		t.Fatal("no seed diverged with the copy origin's counters dropped")
	}
	t.Logf("%d seeds diverged with the copy origin's counters dropped", div)
}
