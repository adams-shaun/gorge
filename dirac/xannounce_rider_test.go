package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXAnnouncementCountsARiderSource: Twitching Doll's "{T}: Add one mana of
// any color. Put a nest counter on this" is a counter-rider mana ability, so
// the payment planner defers it and its card is no census unit; CR 601.2g's
// window can still tap it, and gorge's xAsk reads that window
// (castWindowUnits). Valgavoth's Onslaught ({X}{X}{G}) is therefore offered
// X = 2 from four floating mana plus the Doll, where the same pool alone pays
// only X = 1 (mana_rider.go, mana_tapsdefer.go). The X window has no auto-fill
// option, so gorge's answerer (payexec.WindowAnswer) taps the Doll for the
// fifth mana: the announced X = 2 is paid and the spell stays on the stack,
// the Doll tapped with its nest counter.
func TestXAnnouncementCountsARiderSource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valgavoth's Onslaught", Count: 1}, {Name: "Twitching Doll", Count: 1}}
	type run struct {
		sc   *scen
		doll Obj
		vo   Obj
		xs   []int
		d    Decision
	}
	// four floating mana pay X = 1's {1}{1}{G}; the fifth unit, the Doll's
	// any-colour mana, is what makes X = 2's five payable.
	cast := func(t *testing.T, withDoll bool) run {
		t.Helper()
		sc, id := xScen(t, "std-dsk", deck)
		r := run{sc: sc, doll: NoObj}
		if withDoll {
			r.doll = sc.perm(id["Twitching Doll"], 0)
			// The preconditions the assertion depends on: the Doll is the
			// counter-rider shape, untapped where the window looks, and
			// deferred out of the planner census into the prefix gap.
			if _, _, ok := cardManaRider(&sc.k.t.Cards[id["Twitching Doll"]]); !ok {
				t.Fatal("precondition: Twitching Doll's mana ability is not the counter-rider shape")
			}
			if sc.tapped(r.doll) {
				t.Fatal("precondition: Twitching Doll is tapped")
			}
			if !sc.s.planDeferred(sc.k, 0, sc.slot(r.doll)) || !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(r.doll)) {
				t.Fatal("precondition: the Doll is not a deferred source and prefix gap")
			}
		}
		sc.s.pl[0].pool[kop.W] = 2
		sc.s.pl[0].pool[kop.G] = 1
		sc.s.pl[0].pool[kop.B] = 1
		r.vo = sc.hand(id["Valgavoth's Onslaught"], 0)
		sc.do(t, AkCast, uint8(r.vo), 0xff)
		r.d = sc.s.Step()
		if r.d.Kind != DkChoose || r.d.Sub != ChX {
			t.Fatalf("decision kind %d sub %d, want the X ask", r.d.Kind, r.d.Sub)
		}
		for i := 0; i < int(r.d.N); i++ {
			r.xs = append(r.xs, int(r.d.Options[i].Mode()))
		}
		return r
	}
	// Without the Doll the pool alone pays only X = 1, so the extra value
	// comes from the Doll and not from the cost.
	if xs := fmt.Sprint(cast(t, false).xs); xs != "[0 1]" {
		t.Fatalf("precondition: X options with the pool alone %s, want [0 1]", xs)
	}
	r := cast(t, true)
	if xs := fmt.Sprint(r.xs); xs != "[0 1 2]" {
		t.Fatalf("X options %s, want [0 1 2]", xs)
	}
	// Answer the offered X = 2: the window must tap the Doll for it.
	sc := r.sc
	sc.s.Apply(r.d.Options[2])
	for i := 0; i < 8 && sc.s.ctl != ctlPriority; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 1 || sc.s.objs[r.vo].zone != kop.ZoneStack || sc.s.stack[0].x != 2 {
		t.Fatalf("after X = 2: sp %d, Onslaught zone %d: want it on the stack with X = 2 (the cast reversed)", sc.s.sp, sc.s.objs[r.vo].zone)
	}
	if n := sc.s.perms[sc.slot(r.doll)].ctrN(sc.s, int(kop.CtrNest)); !sc.tapped(r.doll) || n != 1 {
		t.Fatalf("Doll tapped %v, nest counters %d: want it tapped for X = 2 with one nest counter", sc.tapped(r.doll), n)
	}
}
