package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestL7LostSourceKeepsEarlierLayers mirrors TestL7GorgeLostSourceKeepsEarlierLayers
// (CR 613.6, 613.8a): a lord that loses all abilities to an Aura keeps every
// part of a static that began in layers 1 to 5 (a type, subtype or colour
// part, with the P/T or pump riding in the same ability), and loses one that
// begins in layer 6 or 7.
func TestL7LostSourceKeepsEarlierLayers(t *testing.T) {
	other := kop.Filter{Types: kop.TCreature, Flags: kop.FOther}
	for _, tc := range []struct {
		name  string
		build func(l *l7scen)
		check func(t *testing.T, l *l7scen, wolf Obj, lost bool)
	}{
		{"type and base P/T", func(l *l7scen) {
			idx := l.override(kop.Override{AddTypes: kop.TEnchantment, HasPT: true, SetP: 2, SetT: 2})
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(other), Override: uint16(idx + 1)})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
			if ty := l.types(wolf); ty != kop.TCreature|kop.TEnchantment || p != 2 || tt != 2 {
				t.Errorf("lost %v: types %#x base %d/%d, want an enchantment creature 2/2", lost, ty, p, tt)
			}
		}},
		{"colour", func(l *l7scen) {
			idx := l.override(kop.Override{HasColors: true, SetColors: kop.ColR})
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(other), Override: uint16(idx + 1)})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			if c := l.sc.s.colorsAt(l.sc.k, l.sc.slot(wolf)); c != kop.ColR {
				t.Errorf("lost %v: colours %#x, want red", lost, c)
			}
		}},
		{"subtype grant with a pump", func(l *l7scen) {
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatContinuous, Filter: l.filter(other), AddSub: l.mask("Hero"), A: 1, B: 1})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			p, tt := l.sc.s.ptGen(l.sc.k, l.sc.slot(wolf))
			bp, bt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
			if !l.hasSub(wolf, "Hero") || p != bp+1 || tt != bt+1 {
				t.Errorf("lost %v: Hero %v P/T %d/%d over base %d/%d, want a Hero with +1/+1", lost, l.hasSub(wolf, "Hero"), p, tt, bp, bt)
			}
		}},
		{"base P/T alone", func(l *l7scen) {
			idx := l.override(kop.Override{HasPT: true, SetP: 2, SetT: 2})
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(other), Override: uint16(idx + 1)})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			p, tt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
			printed := &l.sc.k.cards[l.id["Ambush Wolf"]]
			want := [2]int{2, 2}
			if lost {
				want = [2]int{int(printed.power), int(printed.tough)}
			}
			if p != want[0] || tt != want[1] {
				t.Errorf("lost %v: base %d/%d, want %d/%d", lost, p, tt, want[0], want[1])
			}
		}},
		{"all creature types with a pump", func(l *l7scen) {
			idx := l.override(kop.Override{AllCreatureTypes: true})
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatContinuous, Filter: l.filter(other), A: 2, B: 2, OutlivesLoss: true})
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatOverride, Filter: l.filter(other), Override: uint16(idx + 1)})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			p, tt := l.sc.s.ptGen(l.sc.k, l.sc.slot(wolf))
			bp, bt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
			if p-bp != 2 || tt-bt != 2 {
				t.Errorf("lost %v: pump %d/%d, want +2/+2 (the all-creature-types part began in layer 4)", lost, p-bp, tt-bt)
			}
		}},
		{"pump alone", func(l *l7scen) {
			l.addStatic("Savannah Lions", kop.Static{Kind: kop.StatContinuous, Filter: l.filter(other), A: 1, B: 1})
		}, func(t *testing.T, l *l7scen, wolf Obj, lost bool) {
			p, tt := l.sc.s.ptGen(l.sc.k, l.sc.slot(wolf))
			bp, bt := l.sc.s.basePTGen(l.sc.k, l.sc.slot(wolf))
			want := 1
			if lost {
				want = 0
			}
			if p-bp != want || tt-bt != want {
				t.Errorf("lost %v: pump %d/%d, want +%d/+%d", lost, p-bp, tt-bt, want, want)
			}
		}},
	} {
		for _, how := range []string{"kept", "aura", "one-shot"} {
			lost := how != "kept"
			l := newL7(t)
			tc.build(l)
			l.auraOverride("Witness Protection", kop.Override{LoseAbilities: true})
			l.start(t)
			lions := l.perm("Savannah Lions")
			wolf := l.perm("Ambush Wolf")
			switch how {
			case "aura":
				l.attach("Witness Protection", lions)
			case "one-shot":
				l.sc.s.perms[l.sc.slot(lions)].ovr.flags |= ovLoseAll
				l.sc.s.refreshTy(l.sc.k)
			}
			t.Run(fmt.Sprintf("%s %s", tc.name, how), func(t *testing.T) { tc.check(t, l, wolf, lost) })
		}
	}
}
