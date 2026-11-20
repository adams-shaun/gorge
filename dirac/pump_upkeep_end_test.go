package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// upkeepDropPrep is the hand-built table an UntilYourNextUpkeep override
// record indexes: one row carrying the class flag. The generated prep marks
// both drop gates live (anyNTOv for the cleanup boundary the record must
// survive, useUpkeepOv for the upkeep on which it ends) and disables the echo
// untap hook so the empty fixture takes the ordinary untap path.
func upkeepDropPrep() *prep {
	g := &genPrep{anyNTOv: true}
	g.uses.set(useUpkeepOv)
	g.ix[mechEchoOwed] = arenaNone
	return &prep{
		t:   &kop.Table{Overrides: []kop.Override{{UntilUpkeep: true}}},
		gen: g,
	}
}

// placeUpkeepRecord applies one Duration$ UntilYourNextUpkeep record from
// controller 0 to slot 0, the way ovDurFl builds it at application.
func placeUpkeepRecord(t *testing.T, s *State, k *prep) {
	t.Helper()
	if len(k.t.Overrides) == 0 || !k.t.Overrides[0].UntilUpkeep {
		t.Fatalf("fixture table's row is not UntilUpkeep: %+v", k.t)
	}
	s.res.e.ctrl = 0
	s.g.fx[0] = fxRec{sl: 0, kind: fxOverride, r: ovRec{idx: 1, fl: s.ovDurFl(kop.DurUntilYourNextUpkeep)}}
	s.g.nfx = 1
	if s.g.fx[0].r.fl&ovNT == 0 {
		t.Fatalf("record fl %#x: not an ovNT record (the duration did not stamp it)", s.g.fx[0].r.fl)
	}
	if who := (s.g.fx[0].r.fl & ovNTWho) >> ovNTWhoShift; who != 0 {
		t.Fatalf("record waits for seat %d, want the effect's controller 0", who)
	}
}

func (s *State) hasUpkeepRecord() bool { return s.g.nfx > 0 && s.g.fx[0].r.idx != 0 }

// newUpkeepState is a two-seat vanilla game whose table the hand-built
// upkeep prep replaces only for the drop paths under test; cleanupGen still
// reaches the State's own registered table (s.k()) for the turn-replacement
// clear.
func newUpkeepState(t *testing.T) (*State, *prep) {
	t.Helper()
	var decks [MaxSeats][]CardID
	for p := 0; p < 2; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, 2, 7, Config{Mulligans: 1})
	return s, upkeepDropPrep()
}

// TestPumpUpkeepEndsAtControllerUpkeep: an UntilYourNextUpkeep override
// (Erhnam Djinn's forestwalk grant) survives the cleanup boundaries an
// UntilYourNextTurn record ends at, both seats' untap steps, and ends only as
// its controller's upkeep step begins.
func TestPumpUpkeepEndsAtControllerUpkeep(t *testing.T) {
	s, k := newUpkeepState(t)
	placeUpkeepRecord(t, s, k)
	if !s.hasUpkeepRecord() {
		t.Fatal("precondition: the record is not in s.g.fx")
	}

	// The cleanup before seat 0's own next turn, then the cleanup before
	// seat 1's turn: a next-turn record ends at the second, ours must not.
	s.active, s.g.exBase = 0, 0
	s.cleanupGen(k)
	if !s.hasUpkeepRecord() {
		t.Fatal("record ended at seat 0's cleanup; UntilYourNextUpkeep must outlive the turn")
	}
	s.active, s.g.exBase = 1, 1
	s.cleanupGen(k)
	if !s.hasUpkeepRecord() {
		t.Fatal("record ended at seat 1's cleanup; it ends only when seat 0's upkeep begins")
	}

	// Seat 1's whole turn, whose upkeep and untap are not the end point.
	s.step, s.active = kop.StepUntap, 1
	s.beginStep(k)
	if s.step != kop.StepUpkeep || !s.hasUpkeepRecord() {
		t.Fatalf("after seat 1's untap: step %v record %v, want StepUpkeep and kept", s.step, s.hasUpkeepRecord())
	}
	s.beginStep(k)
	if !s.hasUpkeepRecord() {
		t.Fatal("record ended in seat 1's upkeep; the target's controller is not the effect's")
	}

	// Seat 0's untap step is still part of the duration.
	s.step, s.active = kop.StepUntap, 0
	s.beginStep(k)
	if !s.hasUpkeepRecord() {
		t.Fatal("record ended in seat 0's untap; the duration runs until the upkeep begins")
	}

	// Seat 0's upkeep begins: the grant ends before its triggers fire.
	s.beginStep(k)
	if s.hasUpkeepRecord() {
		t.Fatal("record survived the beginning of seat 0's upkeep")
	}
}

// TestPumpUpkeepFollowsEffectController: the end point is the resolving
// effect's controller, stamped at application (ovDurFl's ovNTWho), not the
// target permanent's controller afterwards.
func TestPumpUpkeepFollowsEffectController(t *testing.T) {
	s, k := newUpkeepState(t)
	placeUpkeepRecord(t, s, k)

	// The target permanent changes controller before the end point.
	s.perms[0].ctrl = 1
	s.res.e.ctrl = 1
	if who := (s.g.fx[0].r.fl & ovNTWho) >> ovNTWhoShift; who != 0 {
		t.Fatalf("record waits for seat %d after the target changed controller, want seat 0", who)
	}

	s.step, s.active = kop.StepUpkeep, 1
	s.beginStep(k)
	if !s.hasUpkeepRecord() {
		t.Fatal("record ended at seat 1's upkeep; the target's new controller is not the end point")
	}
	s.step, s.active = kop.StepUpkeep, 0
	s.beginStep(k)
	if s.hasUpkeepRecord() {
		t.Fatal("record survived seat 0's upkeep; the effect's controller's upkeep is the end point")
	}
}

// TestUpkeepUseOpensTheGate: the table flag standing in for a genPrep bool
// (genneed.go) is set from the lowered program, so a real table takes the drop
// path, and an UntilYourNextTurn program does not set it.
func TestUpkeepUseOpensTheGate(t *testing.T) {
	for _, tc := range []struct {
		dur  int
		want bool
	}{
		{kop.DurUntilYourNextUpkeep, true},
		{kop.DurUntilYourNextTurn, false},
	} {
		tbl := arenaTable(kop.OpOverride)
		tbl.Cards[0].Abilities[0].Prog[0] = kop.Instr{Op: kop.OpOverride, A: kop.Const(tc.dur)}
		k := prepare(tbl)
		if k.gen == nil {
			if tc.want {
				t.Fatal("an UntilYourNextUpkeep program got no genPrep")
			}
			continue
		}
		if got := k.gen.uses.has(useUpkeepOv); got != tc.want {
			t.Errorf("duration %d: useUpkeepOv %v, want %v", tc.dur, got, tc.want)
		}
	}
}
