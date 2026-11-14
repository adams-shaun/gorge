package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDamageAllReplOrderCrossSeatTwoAsks: a damage-all sweep (Pyroclasm)
// over a board where BOTH seats' Rhox Pummeler (a SHIELD counter) has a
// Crystal Barricade beside it. A shield and a prevention do not commute (the
// order decides whether the counter is spent), so each Rhox event poses a
// CR 616.1 order ask even under the bare-prevention rule, which only skips
// asks between bare full preventions. The permanents are created so that
// slot order runs opposite to gorge's sweep (p1's board entered first, p0
// casts): gorge's effDamageAll walks seat 0's battlefield in entry order,
// then seat 1's, so p0's Rhox asks before p1's although p1's slots are the
// lower ones. The Barricade is the answer each time, so no shield is spent.
func TestDamageAllReplOrderCrossSeatTwoAsks(t *testing.T) {
	sc, id := shieldScen(t, "Pyroclasm", "Rhox Pummeler", "Crystal Barricade")
	s := sc.s
	// p1's board first: its slots are lower than p0's
	r1 := sc.perm(id["Rhox Pummeler"], 1)
	b1 := sc.perm(id["Crystal Barricade"], 1)
	r0 := sc.perm(id["Rhox Pummeler"], 0)
	b0 := sc.perm(id["Crystal Barricade"], 0)
	if sc.slot(r1) > sc.slot(r0) {
		t.Fatalf("probe shape: r1 slot %d r0 slot %d, want p1's slots lower", sc.slot(r1), sc.slot(r0))
	}
	for _, r := range []Obj{r0, r1} {
		if n := s.Counters(r, kop.CtrShield); n != 1 {
			t.Fatalf("Rhox Pummeler %d enters with %d SHIELD, want 1", r, n)
		}
	}
	sc.lands(0, id["Mountain"], 3)
	py := sc.hand(id["Pyroclasm"], 0)
	sc.do(t, AkCast, TCode(py), scAny)

	var asked []Obj
	for i := 0; i < 16; i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp > 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChGenReplOrder {
			want, wb, wp := r0, b0, uint8(0)
			if len(asked) == 1 {
				want, wb, wp = r1, b1, 1
			}
			if d.Player != wp || d.N != 2 {
				t.Fatalf("ask %d: kind %d sub %d player %d opts %v, want player %d's two options (the Barricade and the shield)", len(asked), d.Kind, d.Sub, d.Player, d.Options[:d.N], wp)
			}
			if Obj(s.g.nc.code) != want {
				t.Fatalf("ask %d: event for %d, want %d (gorge sweeps seat %d first)", len(asked), s.g.nc.code, want, wp)
			}
			if s.sp != 1 || s.objs[py].zone != kop.ZoneStack {
				t.Fatalf("ask %d: stack %d spell zone %d, want the spell still resolving", len(asked), s.sp, s.objs[py].zone)
			}
			var pick *Action
			for j := 0; j < int(d.N); j++ {
				switch src := Obj(d.Options[j].Source()); src {
				case wb:
					pick = &d.Options[j]
				case want:
				default:
					t.Fatalf("ask %d: option source %d is neither the Barricade %d nor the Rhox %d", len(asked), src, wb, want)
				}
			}
			if pick == nil {
				t.Fatalf("ask %d: no option for the Barricade %d (opts %v)", len(asked), wb, d.Options[:d.N])
			}
			asked = append(asked, want)
			s.Apply(*pick)
			continue
		}
		if d.Kind == DkPriority && s.sp == 0 {
			break
		}
		t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
	}
	if len(asked) != 2 || asked[0] != r0 || asked[1] != r1 {
		t.Fatalf("asks %v, want one per Rhox event in gorge's sweep order (%d, %d)", asked, r0, r1)
	}
	if s.objs[py].zone != kop.ZoneGraveyard || s.sp != 0 {
		t.Fatalf("after the answers: spell zone %d stack %d, want it resolved", s.objs[py].zone, s.sp)
	}
	for _, r := range []Obj{r0, r1} {
		if !sc.onBF(r) || s.perms[sc.slot(r)].damage != 0 || s.Counters(r, kop.CtrShield) != 1 {
			t.Fatalf("rhox %d: on battlefield %v damage %d shield %d, want it prevented by the Barricade with its counter kept", r, sc.onBF(r), s.perms[sc.slot(r)].damage, s.Counters(r, kop.CtrShield))
		}
	}
}
