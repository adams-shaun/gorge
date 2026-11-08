package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// killClockPower is group K's clock P from the projection: the seat's
// non-defender creatures' positive power, tapped and sick included.
func killClockPower(p *dirac.Projection, seat uint8) int32 {
	var sum int32
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone == kop.ZoneBattlefield && o.Ctrl == seat && p.Chars[i].Types&kop.TCreature != 0 &&
			p.Chars[i].Kw&kop.KwDefender == 0 && o.Power > 0 {
			sum += int32(o.Power)
		}
	}
	return sum
}

// untappedCreatures counts the seat's untapped creatures.
func untappedCreatures(p *dirac.Projection, seat uint8) int {
	n := 0
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone == kop.ZoneBattlefield && o.Ctrl == seat && p.Chars[i].Types&kop.TCreature != 0 && o.Flags&dirac.ProjTapped == 0 {
			n++
		}
	}
	return n
}

// TestExtKillCrossingsFire scans FDN random games, computing 492, 493 and
// 489 from the projection and the sparse bag (never from group K), and
// requires each to hold at least once, so a missing write fails.
func TestExtKillCrossingsFire(t *testing.T) {
	c, pool := testCorpus(t)
	rowCastI := featRowID("mz|hand|castable|instant-speed")
	var n492, n493, n489 int
	seen := 0
	for _, p := range Pairings(pool, 1500, 11) {
		if n492 >= 2 && n493 >= 2 && n489 >= 2 {
			break
		}
		s, fp, ok := killPair(t, c, p)
		if !ok {
			continue
		}
		enc := feat.NewEncoder(fp)
		enc.SetExt(feat.ExtAll)
		pol := dirac.NewRandomPolicy(p.Seed)
		var d dirac.Decision
		for n := 0; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			var proj dirac.Projection
			s.Project(&proj)
			s.ProjectChars(&proj)
			x := enc.EncodeState(s, &d)
			seat := d.Player
			seen++

			oppSpell, mineOnStack := false, false
			for i := 0; i < int(proj.NStack); i++ {
				if e := &proj.Stack[i]; e.Kind == 0 {
					if e.Ctrl != seat {
						oppSpell = true
					} else {
						mineOnStack = true
					}
				}
			}
			want492 := canAttackPower(&proj, seat) >= int32(proj.Life[1-seat]) && oppSpell
			opLethal := canAttackPower(&proj, 1-seat) >= int32(proj.Life[seat])
			if _, got := extSlot(x.Ext, 492); got != want492 {
				t.Fatalf("decision %d: 492 set=%v, want %v", seen, got, want492)
			}
			// 493 one way (the castable-instant row is hashed: presence can
			// only over-approximate); it must also fire at least once
			if _, got := extSlot(x.Ext, 493); got {
				if !opLethal || !extHasRow(x.Sparse, rowCastI) {
					t.Fatalf("decision %d: 493 set without opponent lethal power (%v) and a castable instant", seen, opLethal)
				}
				n493++
			}
			// 489 needs my spell on the stack, the opponent's single untapped
			// creature, and my clock lethal; it is only checked one way here
			// (the stack target is the finer half of the rule).
			if _, got := extSlot(x.Ext, 489); got {
				if !mineOnStack || untappedCreatures(&proj, 1-seat) != 1 || killClockPower(&proj, seat) < int32(proj.Life[1-seat]) {
					t.Fatalf("decision %d: 489 set without its preconditions (mine on stack %v, opp untapped %d, my P %d, opp life %d)",
						seen, mineOnStack, untappedCreatures(&proj, 1-seat), killClockPower(&proj, seat), proj.Life[1-seat])
				}
				n489++
			}
			if want492 {
				n492++
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if n492 == 0 || n493 == 0 || n489 == 0 {
		t.Fatalf("after %d decisions: 492 held %d, 493 held %d, 489 set %d times; each must occur", seen, n492, n493, n489)
	}
	t.Logf("%d decisions: 492 x%d, 493 x%d, 489 x%d", seen, n492, n493, n489)
}

// TestExtKillCounterSides: in counter-prevents-lethal, with my counter on
// the opponent's lethal burn, the burned seat sees 490 and not 491; the
// burner, looking at its own burn under the opponent's counter, sees 491 and
// not 490.
func TestExtKillCounterSides(t *testing.T) {
	sc, ok := stackScenarios(t)["counter-prevents-lethal"]
	if !ok {
		t.Fatal("counter-prevents-lethal scenario missing")
	}
	st, d := castCounterOnTop(t, sc)
	var q dirac.Projection
	st.Project(&q)
	if q.NStack < 2 {
		t.Fatalf("stack %d, want the counter over the burn", q.NStack)
	}
	burner := 1 - sc.Seat
	if q.Stack[q.NStack-1].Ctrl != sc.Seat || q.Stack[0].Ctrl != burner {
		t.Fatalf("stack controllers %d (top) %d (bottom), want counter %d over burn %d",
			q.Stack[q.NStack-1].Ctrl, q.Stack[0].Ctrl, sc.Seat, burner)
	}
	me := encStack(sc.Pool, feat.ExtAll, st, d, sc.Seat)
	if _, ok := extSlot(me, 490); !ok {
		t.Fatalf("burned seat: 490 not set: %v", me)
	}
	if _, ok := extSlot(me, 491); ok {
		t.Fatalf("burned seat: 491 set on the opponent's burn: %v", me)
	}
	if _, ok := extSlot(me, 487); ok {
		t.Fatalf("burned seat: 487 set although it has no damage on the stack: %v", me)
	}
	// the burner's view (the decision's player is the burned seat, but the
	// encoder reads the seat it is given)
	you := encStack(sc.Pool, feat.ExtAll, st, d, burner)
	if _, ok := extSlot(you, 487); !ok {
		t.Fatalf("burner: 487 not set with its lethal burn on the stack: %v", you)
	}
	if _, ok := extSlot(you, 491); !ok {
		t.Fatalf("burner: 491 not set with the opponent's counter on its burn: %v", you)
	}
	if _, ok := extSlot(you, 490); ok {
		t.Fatalf("burner: 490 set on a counter that is not its own: %v", you)
	}
}
