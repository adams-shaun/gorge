package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// On BLB's table.

// activateAb applies o's activation of ability ab when it is offered at
// the next decision, reporting whether it was.
func (sc *scen) activateAb(o Obj, ab uint8) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) && a.Mode() == ab {
			sc.s.Apply(a)
			return true
		}
	}
	return false
}

// TestCardKitsaCopiesTargetSpell (CR 707.10, 602.5b; gorge effCopySpellAbility's
// targeted arm, activation restriction IsPresent$ Card.powerGE3+Self):
// Kitsa's "{2},{T}: copy target instant or sorcery spell you control" is
// offered only while Kitsa's power is 3 or greater, and the copy resolves
// too: Pearl of Wisdom draws 2 + 2.
func TestCardKitsaCopiesTargetSpell(t *testing.T) {
	for _, big := range []bool{false, true} {
		sc, id := giftScen(t, "Kitsa, Otterball Elite", "Pearl of Wisdom", "Island")
		s := sc.s
		kitsa := sc.perm(id["Kitsa, Otterball Elite"], 0)
		if big {
			s.perms[sc.slot(kitsa)].setCtrN(s, int(kop.CtrP1P1), 1) // 2/4; prowess makes it 3
		}
		s.pl[0].pool[kop.U] = 6
		pearl := sc.hand(id["Pearl of Wisdom"], 0)
		sc.castCard(t, pearl)
		// the prowess trigger (+1/+1) resolves first, the Pearl stays
		for i := 0; i < 20; i++ {
			d := s.Step()
			if d.Kind == DkPriority && s.sp == 1 && s.npend == 0 {
				break
			}
			if d.Kind == DkPriority {
				s.Apply(MakeAction(AkPass, 0, 0, 0))
			} else {
				s.Apply(d.Options[0])
			}
		}
		if s.sp != 1 {
			t.Fatalf("big=%v: stack %d, want the Pearl alone", big, s.sp)
		}
		h0 := s.pl[0].handN
		got := sc.activateAb(kitsa, 1)
		if got != big {
			t.Fatalf("big=%v: copy ability offered %v", big, got)
		}
		if got {
			sc.do(t, AkTarget, 0xff, uint8(pearl))
		}
		sc.resolveAll(t)
		want := 2
		if big {
			want = 4
		}
		if n := int(s.pl[0].handN) - int(h0); n != want {
			t.Fatalf("big=%v: drew %d, want %d", big, n, want)
		}
	}
}

// TestStormsplitterCopyExiledAtEndStep (CR 707.2, 603.7; gorge
// CopyPermanent AtEOT$ Exile -> scheduleAtEOT): each instant or sorcery
// cast makes a token copy of Stormsplitter, exiled at the beginning of the
// next end step; the original stays.
func TestStormsplitterCopyExiledAtEndStep(t *testing.T) {
	sc, id := giftScen(t, "Stormsplitter", "Pearl of Wisdom", "Island")
	s := sc.s
	ss := sc.perm(id["Stormsplitter"], 0)
	s.pl[0].pool[kop.U] = 3
	sc.castCard(t, sc.hand(id["Pearl of Wisdom"], 0))
	sc.resolveAll(t)
	cs := sc.copiesOf(id["Stormsplitter"])
	if len(cs) != 2 {
		t.Fatalf("%d Stormsplitters after the cast, want 2", len(cs))
	}
	var tok Obj = NoObj
	for _, sl := range cs {
		if o := s.perms[sl].obj; s.objs[o].of&ofToken != 0 {
			tok = o
		}
	}
	if tok == NoObj {
		t.Fatal("no token copy")
	}
	if s.kw(sc.k, s.objs[tok].slot)&kop.KwHaste == 0 {
		t.Fatal("the copy lacks Stormsplitter's haste")
	}
	turn := s.turn
	for i := 0; i < 80 && s.turn == turn && s.step != kop.StepCleanup; i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			s.Apply(d.Options[0])
		}
	}
	if sc.onBF(tok) {
		t.Fatal("the token copy was not exiled at the end step")
	}
	if !sc.onBF(ss) {
		t.Fatal("the original left")
	}
	if n := len(sc.copiesOf(id["Stormsplitter"])); n != 1 {
		t.Fatalf("%d Stormsplitters after the end step, want 1", n)
	}
}

// TestMockingbirdEntersAsACopy (CR 707.2, 707.9b, 614.1c; gorge's ETB
// clone, cast_etbchoice.go kind "copy" and effClone): cast for X=1 ({1}{U}:
// 2 mana spent) Mockingbird is offered the creatures with mana value 2 or
// less (Jolly Gerbils, not Stormsplitter) and "enter as itself"; picking
// the Gerbils makes it a 2/3 Jolly Gerbils that is also a Bird with
// flying; declining leaves a 1/1 Mockingbird.
func TestMockingbirdEntersAsACopy(t *testing.T) {
	for _, pick := range []bool{true, false} {
		sc, id := giftScen(t, "Mockingbird", "Jolly Gerbils", "Stormsplitter", "Island")
		s := sc.s
		gerbils := sc.perm(id["Jolly Gerbils"], 1)
		storm := sc.perm(id["Stormsplitter"], 0)
		s.pl[0].pool[kop.U] = 2
		mb := sc.hand(id["Mockingbird"], 0)
		sc.castCard(t, mb)
		asked := false
		for i := 0; i < 40 && !sc.onBF(mb); i++ {
			d := s.Step()
			switch {
			case d.Kind == DkChoose && d.Sub == ChX:
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Mode() == 1 {
						s.Apply(d.Options[j])
						break
					}
				}
			case d.Kind == DkChoose && d.Sub == ChGenClone:
				asked = true
				var srcs []Obj
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Mode() == 0 {
						srcs = append(srcs, Obj(d.Options[j].Source()))
					}
				}
				if len(srcs) != 1 || srcs[0] != gerbils || d.Options[d.N-1].Mode() != 1 {
					t.Fatalf("clone options %v (Gerbils %d, Stormsplitter %d), want the Gerbils then itself", d.Options[:d.N], gerbils, storm)
				}
				if pick {
					s.Apply(d.Options[0])
				} else {
					s.Apply(d.Options[d.N-1])
				}
			case d.Kind == DkPriority:
				s.Apply(MakeAction(AkPass, 0, 0, 0))
			default:
				s.Apply(d.Options[0])
			}
		}
		if !asked || !sc.onBF(mb) {
			t.Fatalf("pick=%v: asked %v, on the battlefield %v", pick, asked, sc.onBF(mb))
		}
		sl := sc.slot(mb)
		pw, tg := s.ptGen(sc.k, sl)
		if pick {
			if s.cardAt(sl) != id["Jolly Gerbils"] || pw != 2 || tg != 3 {
				t.Fatalf("copy is card %d %d/%d, want Jolly Gerbils 2/3", s.cardAt(sl), pw, tg)
			}
			bird := -1
			for i, n := range sc.k.t.Subtypes {
				if n == "Bird" {
					bird = i
				}
			}
			if s.kw(sc.k, sl)&kop.KwFlying == 0 || bird < 0 || !s.hasSubGen(sc.k, sl, uint8(bird)) {
				t.Fatal("the copy lacks flying or the Bird subtype")
			}
		} else if s.cardAt(sl) != id["Mockingbird"] || pw != 1 || tg != 1 {
			t.Fatalf("declined: card %d %d/%d, want Mockingbird 1/1", s.cardAt(sl), pw, tg)
		}
	}
}
