package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileToGraveyardTargetMove: the lowering Bronzebeak Foragers and
// Gelatinous Cube use for "put target card exiled with this into its owner's
// graveyard" is a plain OpReturn carrying the exile origin mask (B.N
// 1<<ZoneExile) and the graveyard destination (C). The target is read in
// exile, and an object that left exile before the move is skipped (the
// origin mask is the CR 400.7 precondition).
func TestExileToGraveyardTargetMove(t *testing.T) {
	const srcCard, mothCard, deadCard = 0, 1, 2
	tbl := &kop.Table{
		Name: "exilegravetarget",
		Cards: []kop.CardDef{
			{Name: "T Source", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{
					Kind:     kop.AbActivated,
					Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtCard, Filter: 1}},
					NTargets: 1,
					Prog: []kop.Instr{{
						Op:  kop.OpReturn,
						Obj: kop.RefTarget0,
						B:   kop.Const(1 << kop.ZoneExile),
						C:   int16(kop.ZoneGraveyard),
					}},
				}}},
			{Name: "T Moth", Types: kop.TCreature, Power: 0, Toughness: 2},
			{Name: "T Dead", Types: kop.TCreature, Power: 1, Toughness: 1},
		},
		Filters: []kop.Filter{{}, {Zone: kop.ZoneExile, Types: kop.TCreature}},
	}
	sc := newScenTableLib(t, tbl, mothCard, Config{NoElide: true})
	src := sc.perm(srcCard, 0)
	moth := sc.obj(mothCard, 1)
	sc.s.place(moth, kop.ZoneExile, false)
	dead := sc.obj(deadCard, 1)
	sc.s.place(dead, kop.ZoneGraveyard, false)

	// the activation is offered and its target read sees only the exile card
	sc.do(t, AkActivate, TCode(src), scAny)
	d := sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	var sawMoth, sawDead bool
	for i := 0; i < int(d.N); i++ {
		switch d.Options[i].Target() {
		case TCode(moth):
			sawMoth = true
		case TCode(dead):
			sawDead = true
		}
	}
	if !sawMoth {
		t.Fatalf("the exile card was not a target: %v", d.Options[:d.N])
	}
	if sawDead {
		t.Errorf("the graveyard card was a target: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkTarget, 0, TCode(moth), 0))
	for i := 0; i < 20 && sc.s.objs[moth].zone != kop.ZoneGraveyard; i++ {
		if d := sc.s.Step(); d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		t.Fatalf("stopped at decision kind %d before the move", d.Kind)
	}
	if z := sc.s.objs[moth].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the exile card ended in zone %d, want the graveyard", z)
	}

	// the origin mask: a target that left exile before the move is skipped
	moth2 := sc.obj(mothCard, 1)
	sc.s.place(moth2, kop.ZoneExile, false)
	sc.do(t, AkActivate, TCode(src), scAny)
	sc.do(t, AkTarget, scAny, TCode(moth2))
	sc.s.place(moth2, kop.ZoneHand, false)
	// resolve the ability: pass priority until the stack is empty (a fixed
	// count would walk the game into its draw step)
	for i := 0; i < 20 && sc.s.sp > 0; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("stopped at decision kind %d before the skipped move", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.s.sp != 0 {
		t.Fatalf("the ability never resolved: stack %d", sc.s.sp)
	}
	if z := sc.s.objs[moth2].zone; z != kop.ZoneHand {
		t.Errorf("a target that left exile moved to zone %d, want it to stay in hand", z)
	}
}
