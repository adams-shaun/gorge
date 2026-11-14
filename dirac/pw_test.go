package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func (sc *scen) count(d *Decision, kind ActionKind) int {
	n := 0
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == kind {
			n++
		}
	}
	return n
}

func TestLoyaltyAbilities(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	w := sc.perm(v.EmberWalker, 0)
	sl := sc.slot(w)
	if sc.s.perms[sl].ctrLow(int(kop.CtrLoyalty)) != 3 {
		t.Fatalf("loyalty %d", sc.s.perms[sl].ctrLow(int(kop.CtrLoyalty)))
	}
	bears := sc.perm(v.GrizzlyBears, 1)
	// -3 is affordable: destroy the bears
	sc.do(t, AkActivate, TCode(w), scAny)           // first offered ability: +1
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1)) // 1 damage to seat 1
	d := sc.s.Step()
	if sc.s.perms[sl].ctrLow(int(kop.CtrLoyalty)) != 4 {
		t.Fatalf("+1 not paid on activation")
	}
	if sc.count(&d, AkActivate) != 0 {
		t.Fatalf("a second loyalty ability is offered the same turn")
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.do(t, AkPass, scAny, scAny)
	sc.s.Step()
	if sc.s.pl[1].life != 19 {
		t.Fatalf("life %d", sc.s.pl[1].life)
	}
	// not at instant speed: on seat 1's priority nothing is offered for it
	_ = bears
}

func TestAttackPlaneswalker(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	g := sc.perm(v.HillGiant, 0)
	w := sc.perm(v.EmberWalker, 1)
	sc.passUntil(t, kop.StepAttackers)
	d := sc.s.Step()
	if d.Kind != DkAttackers || sc.count(&d, AkAttack) != 2 {
		t.Fatalf("want 2 attack options (player, walker), got %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkAttack, TCode(g), TCode(w), 0))
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepMain2)
	sc.s.Step()
	if sc.onBF(w) {
		t.Fatalf("walker survived 3 damage at loyalty 3")
	}
	if sc.s.pl[1].life != 20 {
		t.Fatalf("player took damage meant for the walker")
	}
}

func TestOncePerTurn(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Plains, 3)
	kn := sc.perm(v.DrillKnight, 0)
	sc.do(t, AkActivate, TCode(kn), scAny)
	d := sc.s.Step()
	if sc.count(&d, AkActivate) != 0 {
		t.Fatalf("once-per-turn ability offered twice")
	}
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.do(t, AkPass, scAny, scAny)
	if p, _ := sc.s.pt(sc.k, sc.slot(kn)); p != 3 {
		t.Fatalf("power %d", p)
	}
}

func TestLegendRule(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	a := sc.perm(v.LegendaryBear, 0)
	b := sc.perm(v.LegendaryBear, 0)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.N != 2 || d.Player != 0 {
		t.Fatalf("legend decision %+v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 0))
	if sc.onBF(a) || !sc.onBF(b) || sc.s.objs[a].zone != kop.ZoneGraveyard {
		t.Fatalf("legend rule kept the wrong one")
	}
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("no priority after the legend rule")
	}
}
