package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestInstWinLazyExact: with an instwin mode (or endfire) on, lazyPoint
// must not stand in for the seat's own priority points, whose candidate
// set the filter cuts by the state. The same seeded games searched with
// the bot asked at every point (noLazyBot) and with the shortcut must
// reach the same Result and Stats at every seat decision, and the filter
// must have blocked a cast and opened the endfire window somewhere (the
// test is not vacuous). Pass is option 0, the stand-in answer, so under
// instwin the stand-in set rarely differs from the real one in practice;
// the eager searcher's probe bot therefore also checks that lazyPoint
// refuses every own priority point the walk reaches (under endfire the
// stand-in set would keep the pass the filter drops).
func TestInstWinLazyExact(t *testing.T) {
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
	type arm struct {
		mode    uint8
		endFire bool
	}
	fired := 0 // endfire windows over the endfire arms
	arms := []arm{{InstWinPostBlock, false}, {InstWinPostBlockResp, false}, {InstWinCombatResp, false}, {InstWinHold, false}, {InstWinOff, true}, {InstWinHold, true}}
	for _, ar := range arms {
		o := DefaultOptions()
		o.Sims = 48
		o.OpponentNodes = true
		o.PriorTopK = 6
		o.LazyPriors = true
		o.InstWin, o.EndFire = ar.mode, ar.endFire
		esr := NewSearcher(o, &fakeNet{val: -1}, nil)
		probe := &instLazyProbe{sr: esr}
		esr.Bot = probe
		lsr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
		esr.noLazyBot = true
		blocked, ends, decisions, skips := 0, 0, 0, 0
		for seed := uint64(1); seed <= 3; seed++ {
			s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
			pol := dirac.NewRandomPolicy(seed ^ 0x77)
			var d dirac.Decision
			for i := 0; i < 1500; i++ {
				s.StepInto(&d)
				if d.Kind == dirac.DkNone {
					break
				}
				if d.Player != 0 {
					s.Apply(pol.Pick(&d))
					continue
				}
				if d.Kind == dirac.DkPriority {
					esr.seat = 0
					for j := 0; j < int(d.N); j++ {
						if esr.instBlocked(s, d.Options[j]) {
							blocked++
							break
						}
					}
					if esr.endFireNow(s, &d) {
						ends++
					}
				}
				ra := esr.Search(s, seed*0x51+uint64(i))
				rb := lsr.Search(s, seed*0x51+uint64(i))
				if !sameSearch(ra, rb) || ra.Stats != rb.Stats || ra.Searched != rb.Searched || ra.Sims != rb.Sims {
					t.Fatalf("instwin=%d endfire=%v seed %d step %d: lazy search differs:\neager %+v\nlazy  %+v",
						ar.mode, ar.endFire, seed, i, ra.Stats, rb.Stats)
				}
				decisions++
				skips += rb.Stats.PriorSkips
				// the driver holds removal outside the endfire window
				// (else the bot fires it at once and the window never opens)
				if ar.endFire && d.Kind == dirac.DkPriority && ra.NActs == 1 && removalCast(s, ra.Acts[0]) && !esr.endFireNow(s, &d) {
					for j := 0; j < int(d.N); j++ {
						if d.Options[j].Kind() == dirac.AkPass {
							s.Apply(d.Options[j])
							break
						}
					}
					continue
				}
				ra.Play(s)
			}
		}
		t.Logf("instwin=%d endfire=%v: %d decisions, %d with a blocked cast, %d endfire windows, %d lazy skips, %d own priority points probed",
			ar.mode, ar.endFire, decisions, blocked, ends, skips, probe.own)
		if probe.accepted > 0 {
			t.Fatalf("instwin=%d endfire=%v: lazyPoint took the stand-in set at %d of the seat's own priority points",
				ar.mode, ar.endFire, probe.accepted)
		}
		if probe.own == 0 {
			t.Fatalf("instwin=%d endfire=%v: no own priority point reached the probe", ar.mode, ar.endFire)
		}
		if ar.mode != InstWinOff && blocked == 0 {
			t.Fatalf("instwin=%d endfire=%v: the window never blocked a cast", ar.mode, ar.endFire)
		}
		fired += ends
	}
	if fired == 0 {
		t.Fatalf("the endfire window never opened")
	}
}

// instLazyProbe is Heuristic, and at each of advance's own points that is
// the seat's priority decision it asks lazyPoint (with no side effect but
// the scratch point) and counts an acceptance.
type instLazyProbe struct {
	sr            *Searcher
	own, accepted int
}

func (b *instLazyProbe) PurePick() {}

func (b *instLazyProbe) Pick(s *dirac.State, d *dirac.Decision) int {
	sr := b.sr
	if sr.hint >= 0 && s == &sr.w.s && d == &sr.w.d && d.Kind == dirac.DkPriority && d.Player == sr.seat {
		b.own++
		st0 := sr.St
		var pt point
		if sr.lazyPoint(s, d, &pt) {
			b.accepted++
		}
		sr.St = st0
	}
	return Heuristic{}.Pick(s, d)
}
