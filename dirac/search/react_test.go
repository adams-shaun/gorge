package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func reactMatchup(t *testing.T) (*kop.Table, []dirac.CardID, []dirac.CardID) {
	t.Helper()
	a := []cardset.Entry{
		{Name: "Burst Lightning", Count: 8},
		{Name: "Giant Growth", Count: 8},
		{Name: "Stab", Count: 4},
		{Name: "Llanowar Elves", Count: 8},
		{Name: "Fanatical Firebrand", Count: 8},
		{Name: "Mountain", Count: 12},
		{Name: "Forest", Count: 12},
	}
	b := []cardset.Entry{
		{Name: "Llanowar Elves", Count: 8},
		{Name: "Fanatical Firebrand", Count: 8},
		{Name: "Felling Blow", Count: 8},
		{Name: "Mountain", Count: 18},
		{Name: "Forest", Count: 18},
	}
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	return tbl, da, db
}

// reactSticky is the sticky events recomputed independently of observe.
func reactSticky(s *dirac.State, seat uint8) bool {
	for i := 0; i < s.StackLen(); i++ {
		if _, _, _, ctrl := s.StackEntry(i); ctrl != seat {
			return true
		}
	}
	for i := 0; i < s.NumObjs(); i++ {
		o := dirac.Obj(i)
		if _, atk := s.HBotAttackTarget(o); atk || s.HBotBlocking(o) != dirac.NoObj {
			return true
		}
	}
	return false
}

// TestReactTrackTurnScoped: the tracker is reset by a new turn (an event on
// the previous turn never fires it), fires on an opponent event this turn,
// and does not fire on a change to the seat's own permanents or graveyard.
func TestReactTrackTurnScoped(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	fires, resets, ownOnly := 0, 0, 0
	for seed := uint64(1); seed <= 6; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x55)
		var tr reactTrack
		var d dirac.Decision
		turn, wasFired := -1, false
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			tr.observe(s, 0)
			if s.Turn() != turn {
				if wasFired && !tr.fired && !reactSticky(s, 0) {
					resets++
				}
				if tr.fired && !reactSticky(s, 0) {
					t.Fatalf("seed %d turn %d: fired at the turn's first decision with no sticky event", seed, s.Turn())
				}
				turn = s.Turn()
			}
			before := tr.fired
			f := tr.firedNow(s, 0)
			cur := reactCount(s, 0)
			rose := cur.perm > tr.base.perm || cur.ctr > tr.base.ctr || cur.gy > tr.base.gy || cur.ex > tr.base.ex
			if f && !before && !rose && !reactSticky(s, 0) {
				t.Fatalf("seed %d: fired with no opponent event", seed)
			}
			if !f && (rose || reactSticky(s, 0)) {
				t.Fatalf("seed %d: opponent event this turn did not fire", seed)
			}
			if f && !before {
				fires++
			}
			// the seat's own footprint moving alone never fires
			if !f && reactCount(s, 1) != (reactSig{}) {
				ownOnly++
			}
			wasFired = tr.fired
			s.Apply(pol.Pick(&d))
		}
	}
	if fires == 0 || resets == 0 || ownOnly == 0 {
		t.Fatalf("vacuous: fires %d, resets %d, own-only %d", fires, resets, ownOnly)
	}
}

// TestReactRuleWindows: the first decision matching each window case of the
// react rule, over seeded random games.
func TestReactRuleWindows(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	o := DefaultOptions()
	o.InstWin = InstWinReact
	sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
	var held, afterEvent, onStack, oppEnd int
	for seed := uint64(1); seed <= 40; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x99)
		var tr reactTrack
		var d dirac.Decision
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			tr.observe(s, 0)
			if d.Kind == dirac.DkPriority && d.Player == 0 {
				sr.seat, sr.curReact = 0, &tr
				for j := 0; j < int(d.N); j++ {
					a := d.Options[j]
					if a.Kind() != dirac.AkCast {
						continue
					}
					def := &s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))]
					if def.Types&kop.TLand != 0 || (def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0) {
						continue
					}
					blocked := sr.instBlocked(s, a)
					st := s.StepNow()
					switch {
					case s.StackLen() > 0:
						onStack++
						if blocked {
							t.Fatalf("seed %d: cast held with a non-empty stack", seed)
						}
					case s.ActivePlayer() != 0 && st >= kop.StepEnd:
						oppEnd++
						if blocked {
							t.Fatalf("seed %d: cast held at the opponent's end step", seed)
						}
					case tr.fired:
						afterEvent++
						if blocked {
							t.Fatalf("seed %d: cast held after an opponent event", seed)
						}
					default:
						if !blocked {
							t.Fatalf("seed %d turn %d step %d: cast allowed with no opponent event", seed, s.Turn(), st)
						}
						held++
					}
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	t.Logf("held %d, after event %d, on stack %d, opp end %d", held, afterEvent, onStack, oppEnd)
	if held == 0 || afterEvent == 0 || onStack == 0 || oppEnd == 0 {
		t.Fatal("a window case was never reached")
	}
}

func reactSearcher() *Searcher {
	o := DefaultOptions()
	o.Sims = 24
	o.OpponentNodes = true
	o.PriorTopK = 6
	o.LazyPriors = true
	o.InstWin = InstWinReact
	return NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
}

// TestReactHeldRoot: the root's held list is the pruned casts (the options
// the rule blocks, but for the bot's own answer), and a root left with only
// the bot's pass is the held singleton.
func TestReactHeldRoot(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	sr := reactSearcher()
	var withHeld, singles int
	for seed := uint64(1); seed <= 8; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x21)
		var st SeatState
		var d dirac.Decision
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			st.ObserveReact(s, 0)
			if d.Player != 0 || d.Kind != dirac.DkPriority {
				s.Apply(pol.Pick(&d))
				continue
			}
			res := sr.SeatSearch(s, &d, seed, &st)
			var blocked []uint8
			sr.curReact = &sr.rootReact // the walk left it on its last world's
			for j := 0; j < int(d.N); j++ {
				if sr.instBlocked(s, d.Options[j]) {
					blocked = append(blocked, uint8(j))
				}
			}
			held := sr.RootHeld()
			if res.Stats.Searched == 0 && !res.HeldSingle && res.Acts[0].Kind() == dirac.AkPlayLand {
				res.Play(s) // a land play is no point: nothing is pruned
				continue
			}
			if len(held) > len(blocked) || len(blocked)-len(held) > 1 {
				t.Fatalf("seed %d: held %v, blocked %v; N %d kind %v acts %v searched %v sims %d opts %v", seed, held, blocked, d.N, d.Kind, res.Acts[:res.NActs], res.Stats.Searched, sr.Opts.Sims, d.Options[:d.N])
			}
			for _, h := range held {
				ok := false
				for _, b := range blocked {
					ok = ok || b == h
				}
				if !ok || d.Options[h].Kind() != dirac.AkCast {
					t.Fatalf("seed %d: held option %d is not a pruned cast (blocked %v)", seed, h, blocked)
				}
			}
			if len(held) > 0 {
				withHeld++
				if res.Stats.ReactHeld != 1 {
					t.Fatalf("seed %d: ReactHeld %d", seed, res.Stats.ReactHeld)
				}
			}
			if res.HeldSingle {
				singles++
				if res.N != 1 || res.Visits[0] != 1 || res.Prior[0] != 1 || res.Q[0] != 0.5 || res.RootValue != 0.5 ||
					res.Sims != 0 || res.Searched || !res.Record || len(held) == 0 || res.Stats.ReactSingle != 1 ||
					res.NActs != 1 || res.Acts[0].Kind() != dirac.AkPass {
					t.Fatalf("seed %d: held singleton %+v", seed, res)
				}
			} else if res.Stats.ReactSingle != 0 {
				t.Fatalf("seed %d: ReactSingle without HeldSingle", seed)
			}
			res.Play(s)
		}
	}
	t.Logf("%d roots held a cast, %d singletons", withHeld, singles)
	if withHeld == 0 || singles == 0 {
		t.Fatal("no held root / held singleton reached")
	}
}

// TestReactLazyExact: with react on, the bot is asked at every point either
// way (lazyPoint refuses the seat's own priority points), so noLazyBot true
// and false give identical Results and held lists.
func TestReactLazyExact(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	for seed := uint64(1); seed <= 3; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x33)
		eager, lazy := reactSearcher(), reactSearcher()
		eager.noLazyBot = true
		var se, sl SeatState
		var d dirac.Decision
		n := 0
		for i := 0; i < 1500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			se.ObserveReact(s, 0)
			sl.ObserveReact(s, 0)
			if d.Player != 0 {
				s.Apply(pol.Pick(&d))
				continue
			}
			re := *eager.SeatSearch(s, &d, seed, &se)
			he := append([]uint8(nil), eager.RootHeld()...)
			rl := *lazy.SeatSearch(s, &d, seed, &sl)
			if re != rl || string(he) != string(lazy.RootHeld()) {
				t.Fatalf("seed %d decision %d: lazy and eager Results differ", seed, i)
			}
			n++
			re.Play(s)
		}
		if n == 0 {
			t.Fatal("no seat decision")
		}
	}
}
