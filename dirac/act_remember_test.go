package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiscardRemember: an OpDiscard with B.N&1 (RememberDiscarded$) puts
// each card it discards into the remembered set (gorge
// discardAndRememberEvent); without the bit nothing joins.
func TestDiscardRemember(t *testing.T) {
	for _, rem := range []bool{true, false} {
		sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
		fc := id["Valley Floodcaller"]
		for i := 0; i < 3; i++ {
			sc.hand(fc, 0)
		}
		in := kop.Instr{Op: kop.OpDiscard, Obj: kop.RefController, A: kop.Const(2), C: kop.ChooseRandom}
		if rem {
			in.B = kop.Const(1)
		}
		cp := *sc.k.t
		cp.Cards = append([]kop.CardDef(nil), cp.Cards...)
		cp.Cards[fc].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}
		sc.k.t = &cp
		sc.k.gen.anyRemDisc = tableRemembersDiscards(&cp)
		sc.s.res = resCtx{e: stackEntry{card: fc, ctrl: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
		sc.s.g.nrem = 0
		sc.s.exec(sc.k, &cp.Cards[fc].Abilities[0].Prog[0])
		want := 0
		if rem {
			want = 2
		}
		if int(sc.s.g.nrem) != want || sc.s.pl[0].handN != 1 {
			t.Fatalf("remember=%v: remembered %d (want %d), hand %d", rem, sc.s.g.nrem, want, sc.s.pl[0].handN)
		}
	}
}

// TestSacrificeRemember: an OpSacrifice with B.N&1 (RememberSacrificed$)
// puts the permanent it sacrifices into the remembered set (gorge
// effSacrifice's rememberLKICapture); without the bit nothing joins.
func TestSacrificeRemember(t *testing.T) {
	for _, rem := range []bool{true, false} {
		sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
		fc := id["Valley Floodcaller"]
		a := sc.perm(fc, 0)
		in := kop.Instr{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.Const(1), F: firstInstr(t, sc, fc, kop.OpUntapAll).F}
		if rem {
			in.B = kop.Const(1)
		}
		cp := *sc.k.t
		cp.Cards = append([]kop.CardDef(nil), cp.Cards...)
		cp.Cards[fc].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}
		sc.k.t = &cp
		sc.k.gen.anyRemSac = tableRemembersOp(&cp, kop.OpSacrifice)
		sc.s.res = resCtx{e: stackEntry{card: fc, ctrl: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
		sc.s.g.nrem = 0
		sc.s.exec(sc.k, &cp.Cards[fc].Abilities[0].Prog[0])
		want := 0
		if rem {
			want = 1
		}
		if sc.onBF(a) || int(sc.s.g.nrem) != want || rem && sc.s.g.rem[0] != a {
			t.Fatalf("remember=%v: on battlefield %v, remembered %d (want %d)", rem, sc.onBF(a), sc.s.g.nrem, want)
		}
	}
}

// TestDigUntilRemember: RememberFound$ replaces the remembered set with the
// found cards, accumulating across the walk's players.
func TestDigUntilRemember(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b, c := sc.hand(fc, 0), sc.hand(fc, 0), sc.hand(fc, 1)
	sc.s.g.nrem = 0
	sc.s.remember(a)
	sc.s.digUntilRemember(true, []Obj{b})
	sc.s.digUntilRemember(false, []Obj{c})
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != b || sc.s.g.rem[1] != c {
		t.Fatalf("remembered %v (n=%d)", sc.s.g.rem[:sc.s.g.nrem], sc.s.g.nrem)
	}
}
