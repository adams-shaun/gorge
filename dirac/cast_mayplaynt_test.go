package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mayNTScen builds a scenario whose table grants a play permission with
// Duration$ UntilYourNextTurn, so prepMayPlayNT lays coldMayNTQ out and
// mayNTSweep reads its records; withGrant=false leaves the table plain.
func mayNTScen(t *testing.T, withGrant bool) *scen {
	t.Helper()
	sc, _ := newSetScenTweak(t, "blb", []cardset.Entry{{Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		if !withGrant {
			return
		}
		cd := &tbl.Cards[id["Plains"]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbActivated, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, C: int16(kop.ZoneExile), A: kop.Const(kop.DurUntilYourNextTurn)},
		}})
	})
	return sc
}

// ntRec appends an UntilYourNextTurn record of ctrl to the pool and returns
// its index.
func (sc *scen) ntRec(ctrl uint8) int {
	i := int(sc.s.g.nmay)
	*sc.s.mayAt(i) = mayPlay{obj: Obj(i + 1), ctrl: ctrl, until: kop.DurUntilYourNextTurn, kind: kop.MayPlayFree, turn: mayWord(sc.s.turn)}
	sc.s.g.nmay++
	return i
}

// TestMayPlayUntilNextTurnEndsWhenTurnIsNext: an OpMayPlay record with
// Duration$ UntilYourNextTurn ends at the cleanup before its controller's
// next turn begins, the boundary gorge's rescheduleNextTurnBoundaries
// re-derives from the live rotation and the pending extra-turn queue every
// cleanup -- not at the first cleanup where its controller is not the active
// player (the keep arm this replaces): an extra turn granted its controller
// makes the boundary the current turn's cleanup, and the record never
// outlives the turn before it. Precondition: no extra turn is pending where
// a case's assertion keys on the ordinary rotation (exNextTurn).
func TestMayPlayUntilNextTurnEndsWhenTurnIsNext(t *testing.T) {
	t.Run("scan and sweep", func(t *testing.T) {
		plain := mayNTScen(t, false)
		if n := coldMayNTQ.count(plain.s.cold.layout); n != 0 {
			t.Fatalf("a table with no UntilYourNextTurn grant: cold counters %d; want 0", n)
		}
		if ends := plain.s.mayNTSweep(); ends != 0 {
			t.Fatalf("a table with no UntilYourNextTurn grant swept %v; want 0", ends)
		}
		sc := mayNTScen(t, true)
		if n := coldMayNTQ.count(sc.s.cold.layout); n != MaxSeats {
			t.Fatalf("cold counters %d; want %d", n, MaxSeats)
		}
	})

	t.Run("extra turn for the controller ends it now", func(t *testing.T) {
		// A grant registered on its controller's turn; a take-an-extra-turn
		// resolves the same turn, so the controller's turn is the next one
		// and gorge ends the grant at this cleanup (UntilTurn = extra
		// turn - 1). The replaced arm kept it (s.active == r.ctrl) and would
		// offer the cast through the extra turn.
		sc := mayNTScen(t, true)
		s := sc.s
		if s.g.exN != 0 {
			t.Fatalf("%d extra turns pending; nextActive would not follow the queue", s.g.exN)
		}
		s.active = 0
		s.g.exBase = s.active // the seeded fixture's rotation base is the live active seat
		sc.ntRec(0)
		s.g.exQ[s.g.exN] = 0
		s.g.exN++
		if nt := s.nextActive(); nt != 0 {
			t.Fatalf("nextActive %d, want 0 (the granted extra turn)", nt)
		}
		if ends := s.mayNTSweep(); ends&(1<<0) == 0 {
			t.Fatal("the controller's extra turn is next; the record must end at this cleanup")
		}
	})

	t.Run("the controller's own cleanup keeps it", func(t *testing.T) {
		// Registered on the controller's turn, nothing pending: the record
		// survives the controller's own cleanup and ends at the opponent's.
		sc := mayNTScen(t, true)
		s := sc.s
		if s.g.exN != 0 {
			t.Fatalf("%d extra turns pending; nextActive would not follow the rotation", s.g.exN)
		}
		s.active = 0
		s.g.exBase = s.active // the seeded fixture's rotation base is the live active seat
		sc.ntRec(0)
		if ends := s.mayNTSweep(); ends&(1<<0) != 0 {
			t.Fatalf("nextActive %d: the controller's own cleanup must keep the record", s.nextActive())
		}
		s.active = 1
		s.g.exBase = s.active // mid seat 1's turn, the last normal turn's holder
		if ends := s.mayNTSweep(); ends&(1<<0) == 0 {
			t.Fatalf("nextActive %d: the record must end at the cleanup before the controller's turn", s.nextActive())
		}
	})

	t.Run("only UntilYourNextTurn records sweep", func(t *testing.T) {
		// A DurYourNextTurn record (until the end of the controller's next
		// turn) is the other arm's (turn_gen.go's turn-word comparison); the
		// sweep must not read it.
		sc := mayNTScen(t, true)
		s := sc.s
		s.active = 0
		s.g.exBase = s.active
		i := int(s.g.nmay)
		*s.mayAt(i) = mayPlay{obj: Obj(i + 1), ctrl: 0, until: kop.DurYourNextTurn, kind: kop.MayPlayFree, turn: mayWord(s.turn)}
		s.g.nmay++
		if ends := s.mayNTSweep(); ends != 0 {
			t.Fatalf("a DurYourNextTurn record ended (%v); the sweep is UntilYourNextTurn only", ends)
		}
	})
}
