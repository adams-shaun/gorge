package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDigUntilOptionalDeclineSkipsShuffle is the decline path of an
// OpMayAsk(MayGuardDigUntil) guard over [DigUntil, Shuffle] — the lowered
// shape of an Optional$ DigUntil with Shuffle$ True (Thicket Elemental's
// kicked ETB). gorge's effDigUntil declines with continue before the walk's
// shuffle tail (engine/effects/diguntil.go:208): answering "no" — and the
// empty library's unasked guard — must skip the Shuffle behind the dig too,
// so neither side reorders the library nor draws RNG; a "yes" runs the dig
// and the shuffle. The guard's A operand is the skip count on both the
// declined ask (dispatch applyAskUnread) and the unasked guard (act_mayask
// drawAsks), so the skip=1 form the lowering used to emit leaves the Shuffle
// running after a decline (the pinned skip=1 subtest) and the skip=2 form the
// lowering now emits covers both instructions.
func TestDigUntilOptionalDeclineSkipsShuffle(t *testing.T) {
	const (
		foundGraveRestLib = int16(kop.ZoneGraveyard) | int16(kop.ZoneLibrary)<<3
	)
	scen := func(t *testing.T, skip int, bear bool) (*scen, Obj) {
		t.Helper()
		var f kop.FilterID
		var bearID CardID
		sc, _ := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
			f = kop.FilterID(len(tbl.Filters) - 1)
			bearID = CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature})
			return []kop.Instr{
				{Op: kop.OpMayAsk, C: kop.MayGuardDigUntil, Obj: kop.RefController, A: kop.Const(skip)},
				{Op: kop.OpDigUntil, Obj: kop.RefController, A: kop.Const(1), C: foundGraveRestLib, F: f},
				{Op: kop.OpShuffle, Obj: kop.RefController},
			}
		})
		var o Obj
		if bear {
			o = sc.obj(bearID, 0)
			sc.s.objs[o].zone = kop.ZoneLibrary
			sc.s.pl[0].libTop(o)
		}
		return sc, o
	}
	libCopy := func(s *State) []Obj {
		pl := &s.pl[0]
		return append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
	}

	// The guard asks over a card-bearing library, and a decline with the
	// covering skip=2 leaves the library and the RNG untouched.
	{
		sc, bear := scen(t, 2, true)
		s := sc.s
		before := libCopy(s)
		draws := s.rng.Draws
		s.runProg(sc.k)
		if s.res.ask != raGenMayAsk {
			t.Fatalf("skip=2: no dig may-ask, res.ask %d", s.res.ask)
		}
		s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, 0)) // no
		if s.rng.Draws != draws {
			t.Fatalf("skip=2 decline: RNG draws %d -> %d, want none: the shuffle ran behind the decline", draws, s.rng.Draws)
		}
		if after := libCopy(s); len(after) != len(before) || after[0] != before[0] || after[len(after)-1] != before[len(before)-1] {
			t.Fatalf("skip=2 decline: the library moved (%d -> %d cards, ends %v/%v)", len(before), len(after), before[0], after[0])
		}
		if z := s.objs[bear].zone; z != kop.ZoneLibrary {
			t.Fatalf("skip=2 decline: bear zone %d, want the library", z)
		}
	}

	// The pinned skip=1 form runs the Shuffle after the decline: the same
	// program with the old skip draws and reorders, which is exactly the
	// divergence the covering skip removes.
	{
		sc, _ := scen(t, 1, true)
		s := sc.s
		draws := s.rng.Draws
		s.runProg(sc.k)
		if s.res.ask != raGenMayAsk {
			t.Fatalf("skip=1: no dig may-ask, res.ask %d", s.res.ask)
		}
		s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, 0)) // no
		if s.rng.Draws == draws {
			t.Fatal("skip=1 decline: no RNG draws, want the shuffle the short skip leaves running")
		}
	}

	// An accept runs the dig (the bear to the graveyard) and then the shuffle.
	{
		sc, bear := scen(t, 2, true)
		s := sc.s
		draws := s.rng.Draws
		s.runProg(sc.k)
		if s.res.ask != raGenMayAsk {
			t.Fatalf("accept: no dig may-ask, res.ask %d", s.res.ask)
		}
		s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, 1)) // yes
		if z := s.objs[bear].zone; z != kop.ZoneGraveyard {
			t.Fatalf("accept: bear zone %d, want the graveyard (the dig ran)", z)
		}
		if s.rng.Draws == draws {
			t.Fatal("accept: no RNG draws, want the shuffle tail")
		}
	}

	// The unasked guard (an empty library) advances past both instructions:
	// with the covering skip nothing runs and the RNG is untouched.
	{
		sc, _ := scen(t, 2, false)
		s := sc.s
		pl := &s.pl[0]
		pl.libLo = pl.libHi // the empty-library decline: gorge continues before the walk's shuffle tail
		draws := s.rng.Draws
		s.runProg(sc.k)
		if s.res.ask == raGenMayAsk {
			t.Fatal("empty library: the guard asked")
		}
		if s.rng.Draws != draws {
			t.Fatalf("empty library: RNG draws %d -> %d, want none", draws, s.rng.Draws)
		}
		if s.res.pc != 3 {
			t.Fatalf("empty library: pc %d, want 3 (past the dig and the shuffle)", s.res.pc)
		}
	}
}
