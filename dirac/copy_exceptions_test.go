package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 613 copy-exception scenarios (copy_exceptions.go): a copy's exceptions
// are layer-1 copiable values (CR 707.9b), as gorge's CopyExcept folds them
// into the copy's basis face.

// offspringToken casts Iridescent Vinelasher with its offspring cost paid
// and returns the 1/1 token copy.
func offspringToken(t *testing.T, sc *scen, id map[string]CardID) Obj {
	t.Helper()
	for i := 0; i < 3; i++ {
		sc.perm(id["Swamp"], 0)
	}
	vl := sc.hand(id["Iridescent Vinelasher"], 0)
	sc.castMode(t, vl, 1)
	sc.resolveAll(t)
	for _, sl := range sc.copiesOf(id["Iridescent Vinelasher"]) {
		if o := sc.s.perms[sl].obj; sc.s.objs[o].of&ofToken != 0 {
			if p, tg := sc.s.ptGen(sc.k, sl); p != 1 || tg != 1 {
				t.Fatalf("offspring token is %d/%d, want 1/1", p, tg)
			}
			return o
		}
	}
	t.Fatal("no offspring token")
	return NoObj
}

// TestL6OffspringTokenCopiedByAnotherCopy: Mockingbird entering as a copy
// of an Offspring token is a 1/1 (the token's 1/1 is a copiable value), and
// a Bird with flying (its own exceptions, applied after the template's).
func TestL6OffspringTokenCopiedByAnotherCopy(t *testing.T) {
	sc, id := giftScen(t, "Iridescent Vinelasher", "Mockingbird", "Swamp", "Island")
	s := sc.s
	tok := offspringToken(t, sc, id)
	s.pl[0].pool[kop.U] = 4
	mb := sc.hand(id["Mockingbird"], 0)
	sc.castCard(t, mb)
	asked := false
	for i := 0; i < 40 && !sc.onBF(mb); i++ {
		d := s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChX:
			best := 0
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Mode() > d.Options[best].Mode() {
					best = j
				}
			}
			s.Apply(d.Options[best])
		case d.Kind == DkChoose && d.Sub == ChGenClone:
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Mode() == 0 && Obj(d.Options[j].Source()) == tok {
					s.Apply(d.Options[j])
					asked = true
					break
				}
			}
			if !asked {
				t.Fatalf("the offspring token is not a clone option: %v", d.Options[:d.N])
			}
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			s.Apply(d.Options[0])
		}
	}
	if !asked || !sc.onBF(mb) {
		t.Fatalf("asked %v, on the battlefield %v", asked, sc.onBF(mb))
	}
	sl := sc.slot(mb)
	pw, tg := s.ptGen(sc.k, sl)
	if s.cardAt(sl) != id["Iridescent Vinelasher"] || pw != 1 || tg != 1 {
		t.Fatalf("Mockingbird copy is card %d %d/%d, want Iridescent Vinelasher 1/1", s.cardAt(sl), pw, tg)
	}
	bird := -1
	for i, n := range sc.k.t.Subtypes {
		if n == "Bird" {
			bird = i
		}
	}
	if s.kw(sc.k, sl)&kop.KwFlying == 0 || bird < 0 || !s.hasSubGen(sc.k, sl, uint8(bird)) {
		t.Fatal("the copy lacks its own flying or Bird exception")
	}
}

// TestL6SetPTOverOffspringEndsAtCleanup: a one-shot "becomes 4/4 until end
// of turn" (OpSetPT, layer 7b) on an Offspring token overrides its 1/1
// only while it lasts; at cleanup the token is 1/1 again, not 4/4 (the 1/1
// was kept in the same override fields as the 7b effect).
func TestL6SetPTOverOffspringEndsAtCleanup(t *testing.T) {
	sc, id := giftScen(t, "Iridescent Vinelasher", "Swamp")
	s := sc.s
	tok := offspringToken(t, sc, id)
	sl := s.objs[tok].slot
	p := &s.perms[sl]
	// OpSetPT's in.C == 1 arm (interp.go)
	p.ovr.setP, p.ovr.setT = 4, 4
	p.ovr.flags |= ovSetPTEOT
	if pw, tg := s.ptGen(sc.k, sl); pw != 4 || tg != 4 {
		t.Fatalf("token is %d/%d under the set, want 4/4", pw, tg)
	}
	s.finishCleanup(sc.k)
	if pw, tg := s.ptGen(sc.k, sl); pw != 1 || tg != 1 {
		t.Fatalf("token is %d/%d after cleanup, want 1/1", pw, tg)
	}
}
