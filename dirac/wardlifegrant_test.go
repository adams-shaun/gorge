package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWardLifeGranted: Hexing Squelcher's Continuous static grants other
// creatures "Ward—Pay 2 life" (kop.Static.WardLife). The targeted creature
// has its own printed ward {2}, so both instances fire, the printed one
// first and then the granted one (CR 702.21a, 113.2c), and the granted one
// asks the printed life ward's pay-N-life question. Before, splitWardGrant
// read only Ward:<N>, so the static carried no amount and the granted life
// ward never queued.
func TestWardLifeGranted(t *testing.T) {
	sc, id := synthScen(t, "std-ecl", "Hexing Squelcher", "Bristlebane Battler", "Sear", "Mountain", "Swamp")
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyWardGrant {
		t.Fatal("precondition: the table has no granted ward")
	}
	hex := sc.perm(id["Hexing Squelcher"], 0)
	battler := sc.perm(id["Bristlebane Battler"], 0)
	// the grant is lowered to data, not a keyword bit: each granting static
	// is its own ward instance with its own amount
	grant := false
	for i := range k.t.Cards[id["Hexing Squelcher"]].Abilities {
		a := &k.t.Cards[id["Hexing Squelcher"]].Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
			continue
		}
		if a.Static.WardLife != 2 || a.Static.Ward != 0 {
			t.Fatalf("Hexing Squelcher static ward %d life %d, want the granted pay-2-life ward", a.Static.Ward, a.Static.WardLife)
		}
		if a.Static.Kw&kop.KwWard == 0 {
			t.Fatal("the life ward grant does not carry the ward keyword")
		}
		grant = true
	}
	if !grant {
		t.Fatal("Hexing Squelcher has no Continuous static")
	}
	if !sc.onBF(hex) || !sc.onBF(battler) {
		t.Fatal("precondition: Hexing Squelcher and Bristlebane Battler are not on the battlefield")
	}
	spell := sc.obj(id["Sear"], 1)
	e := &stackEntry{obj: spell, ctrl: 1}
	s.npend = 0
	s.targetedOne(k, e, uint8(battler))
	if s.npend != 2 {
		t.Fatalf("%d ward triggers, want the printed {2} and the granted pay 2 life", s.npend)
	}
	if got := s.pend[0].trigAmt; got != 2 {
		t.Fatalf("printed ward amount %d, want 2", got)
	}
	granted := s.pend[1].trigAmt
	want := int16(2) | wardGrantedBits | wardGrantedLifeBit
	if granted != want {
		t.Fatalf("granted ward amount %#x, want %#x (pay 2 life)", granted, want)
	}
	if !wardIsGranted(granted) || !wardIsGrantedLife(granted) {
		t.Fatalf("granted ward amount %#x is not marked as a granted life ward", granted)
	}

	// The resolved ask is the printed life ward's: pay 2 life, once the
	// entry's grant marker is decoded to -2.
	s.stack[0] = stackEntry{kind: seSpell, obj: spell, ctrl: 1}
	s.sp = 1
	pt := s.pend[1]
	s.res.e = stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl,
		trigObj: pt.trigObj, trigPl: pt.trigPl, trigAmt: pt.trigAmt}
	s.resolveWard(k)
	if s.res.e.trigAmt != -2 {
		t.Fatalf("resolved granted life ward amount %d, want -2", s.res.e.trigAmt)
	}
	if s.res.ask != raGenPay || s.g.pickZ != kop.PayLife || s.g.pickLeft != 2 || s.g.pickP != 1 {
		t.Fatalf("granted life ward ask %d/%d/%d/%d, want the pay 2 life ask", s.res.ask, s.g.pickZ, s.g.pickLeft, s.g.pickP)
	}
}
