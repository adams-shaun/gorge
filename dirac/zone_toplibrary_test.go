package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTopLibraryGuard: ChangeZone Defined$ TopOfLibrary Optional$ True
// (OpMayAsk MayGuardLibraryTop over ExtToGraveyard / OpToLibrary C=1, both on
// RefLibTop; gorge moveDefinedLibraryObjects). An empty library is consumed
// with no ask and no move (unlike a mill's mill_optional); otherwise the
// controller is asked first, nothing moves before the answer, a yes moves the
// top card to the graveyard or the bottom of the library, a no leaves it.
func TestTopLibraryGuard(t *testing.T) {
	may := kop.Instr{Op: kop.OpMayAsk, C: kop.MayGuardLibraryTop, A: kop.Const(1)}
	arms := []struct {
		name string
		move kop.Instr
	}{
		{"graveyard", kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtToGraveyard)), Obj: kop.RefLibTop}},
		{"bottom", kop.Instr{Op: kop.OpToLibrary, Obj: kop.RefLibTop, C: 1}},
	}
	for _, arm := range arms {
		prog := []kop.Instr{may, arm.move}
		t.Run(arm.name+"/empty library", func(t *testing.T) {
			sc := czScen(t, prog)
			s := sc.s
			pl := &s.pl[0]
			pl.libLo = pl.libHi
			if !s.exec(sc.k, &prog[0]) || s.res.ask == raGenMayAsk {
				t.Fatal("an empty library was asked")
			}
			grave := pl.graveN
			s.exec(sc.k, &prog[1])
			if pl.libLen() != 0 || pl.graveN != grave {
				t.Fatalf("the move ran on an empty library: library %d graveyard %d", pl.libLen(), pl.graveN)
			}
		})
		for _, answer := range []uint8{1, 0} {
			t.Run(arm.name+"/answer "+string('0'+rune(answer)), func(t *testing.T) {
				sc := czScen(t, prog)
				s := sc.s
				pl := &s.pl[0]
				top, bottom := pl.lib[int(pl.libLo)], pl.lib[int(pl.libHi)-1]
				libN, graveN := pl.libLen(), pl.graveN
				if s.exec(sc.k, &prog[0]) || s.res.ask != raGenMayAsk || s.res.askP != 0 {
					t.Fatalf("no ask for the controller, res.ask %d", s.res.ask)
				}
				var d Decision
				s.resolveOptionsGen(sc.k, &d)
				if d.Sub != ChGenConfirm || d.N != 2 || d.Options[0].Mode() != 1 || d.Options[1].Mode() != 0 {
					t.Fatalf("ask options: sub %d n %d, want confirm yes then no", d.Sub, d.N)
				}
				if pl.libLen() != libN || pl.graveN != graveN || pl.lib[int(pl.libLo)] != top {
					t.Fatal("moved before the answer")
				}
				s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, answer))
				switch {
				case answer == 0:
					if pl.libLen() != libN || pl.graveN != graveN || pl.lib[int(pl.libLo)] != top {
						t.Fatal("a no moved the top card")
					}
				case arm.name == "graveyard":
					if s.objs[top].zone != kop.ZoneGraveyard || pl.graveN != graveN+1 || pl.libLen() != libN-1 {
						t.Fatalf("a yes: top card zone %d, graveyard %d, library %d", s.objs[top].zone, pl.graveN, pl.libLen())
					}
				default:
					if pl.libLen() != libN || pl.graveN != graveN || pl.lib[int(pl.libHi)-1] != top || pl.lib[int(pl.libLo)] == top {
						t.Fatalf("a yes: the top card is not at the bottom (library %d, new bottom %d, top %d)", pl.libLen(), pl.lib[int(pl.libHi)-1], top)
					}
					if pl.lib[int(pl.libHi)-2] != bottom {
						t.Fatal("the old bottom card is not second from the bottom")
					}
				}
			})
		}
	}
	t.Run("bottom/one-card library", func(t *testing.T) {
		prog := []kop.Instr{may, arms[1].move}
		sc := czScen(t, prog)
		s := sc.s
		pl := &s.pl[0]
		pl.libLo = pl.libHi - 1
		only := pl.lib[int(pl.libLo)]
		if s.exec(sc.k, &prog[0]) || s.res.ask != raGenMayAsk {
			t.Fatal("a one-card library was not asked")
		}
		s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, 1))
		if pl.libLen() != 1 || pl.lib[int(pl.libLo)] != only {
			t.Fatalf("a one-card library changed: %d cards", pl.libLen())
		}
	})
}
