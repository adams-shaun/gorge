package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawRemembers: an OpDraw with RememberDrawn$ remembers the cards that
// actually reached the drawing player's hand. A declined OptionalDecider$
// skip remembers nothing, and a draw replacement that draws nothing remembers
// nothing; AllReplaced remembers every card a replacement body drew, True only
// the pre-draw top card.
func TestDrawRemembers(t *testing.T) {
	const n = 2
	draw := kop.Instr{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(n), C: kop.MvRemember}
	may := kop.Instr{Op: kop.OpMayAsk, C: kop.MayGuardDraw, A: kop.Const(1)}

	t.Run("declined", func(t *testing.T) {
		prog := []kop.Instr{may, draw}
		for _, answer := range []uint8{1, 0} {
			sc := czScen(t, prog)
			s := sc.s
			if s.exec(sc.k, &prog[0]) || s.res.ask != raGenMayAsk {
				t.Fatalf("answer %d: no draw ask, res.ask %d", answer, s.res.ask)
			}
			hand := s.pl[0].handN
			s.applyAskUnread(sc.k, MakeAction(AkChoose, 0, 0, answer))
			wantN, wantHand := uint8(n), hand+uint8(n)
			if answer == 0 {
				wantN, wantHand = 0, hand
			}
			if s.g.nrem != wantN || s.pl[0].handN != wantHand {
				t.Fatalf("answer %d: remembered %d hand %d, want %d and %d",
					answer, s.g.nrem, s.pl[0].handN, wantN, wantHand)
			}
		}
	})

	// An AllReplaced draw whose replacement draws nothing moves no card to
	// hand, so it remembers nothing (gorge recordDrawn: the pre-draw library
	// snapshot met with the hand). Only AllReplaced is pinned here: a lowered
	// True writer never meets such a replacement.
	{
		sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}},
			func(tbl *kop.Table, id map[string]CardID) {
				id["Test DrawNothing"] = CardID(len(tbl.Cards))
				tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test DrawNothing", Types: kop.TEnchantment,
					Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 0}}}})
			})
		sc.perm(id["Test DrawNothing"], 0)
		in := draw
		in.C |= kop.DrawRememberAll
		sc.s.exec(sc.k, &in)
		if sc.s.g.nrem != 0 || sc.s.pl[0].handN != 0 {
			t.Fatalf("draw nothing: remembered %d hand %d, want none", sc.s.g.nrem, sc.s.pl[0].handN)
		}
	}

	// AllReplaced remembers every card a replacement body drew; True only
	// the pre-draw top card.
	for _, all := range []bool{false, true} {
		sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}},
			func(tbl *kop.Table, id map[string]CardID) {
				id["Test DrawDouble"] = CardID(len(tbl.Cards))
				tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test DrawDouble", Types: kop.TEnchantment,
					Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDrawMore, A: 2}}}})
			})
		sc.perm(id["Test DrawDouble"], 0)
		in := draw
		in.A = kop.Const(1)
		if all {
			in.C |= kop.DrawRememberAll
		}
		sc.s.exec(sc.k, &in)
		want := 1
		if all {
			want = 2
		}
		if int(sc.s.g.nrem) != want || sc.s.pl[0].handN != 2 {
			t.Fatalf("all=%v: remembered %d hand %d, want %d remembered and 2 drawn", all, sc.s.g.nrem, sc.s.pl[0].handN, want)
		}
	}
}
