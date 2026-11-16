package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStaticRedirectBesideEffectPrevention: a printed static redirect beside an
// Ext PreventNext depleting shield or an Ext PreventColor record is a CR 616.1
// candidate pair, so the kernel stalls for gorge's order ask instead of
// applying the prevention and dropping the redirect.
func TestStaticRedirectBesideEffectPrevention(t *testing.T) {
	const them = TCode(PlayerTarget0 + 1)
	cases := []struct {
		name string
		rec  prevRec
	}{
		{"depleting shield", prevRec{col: kop.ColR, sc: prevNextSc, ctl: them}},
		{"colour prevention", prevRec{col: kop.ColG}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := newSPTable()
			host := tb.host("Pariah", kop.Repl{Kind: kop.ReplDamageRedirect, C: kop.RdrToYou | kop.RdrAsAttached})
			sc := tb.scen(t)
			aura := sc.perm(host, 1)
			bear := sc.perm(v.GrizzlyBears, 1)
			sc.s.perms[sc.slot(aura)].attached = sc.slot(bear)
			src := sc.perm(v.GrizzlyBears, 0)
			sc.s.g.prevCol[0] = c.rec
			sc.k.gen.anyRepl |= rbPrevCol
			if e, ok := sc.s.rdrEvOf(0, sc.slot(src), v.GrizzlyBears, them, false); !ok ||
				sc.s.statRdrCollect(sc.k, &e, new([rdrMax]rdrCand)) != 1 {
				t.Fatal("precondition: the redirect does not match the event")
			}
			if c.rec.sc == 0 && !sc.s.prevColHit(sc.k, sc.slot(src), v.GrizzlyBears, prevTgt{1, NoSlot}, false) {
				t.Fatal("precondition: the colour record does not prevent the source's damage")
			}
			if got := rdrDeal(sc, 0, sc.slot(src), v.GrizzlyBears, them, 3); got != 0 || sc.s.g.stalled != StallRedirOrder {
				t.Errorf("damage beside %s = %d stalled %d, want 0 / %d", c.name, got, sc.s.g.stalled, StallRedirOrder)
			}
			if got := sc.s.perms[sc.slot(bear)].damage; got != 0 {
				t.Errorf("the enchanted creature took %d, want 0 (the ask comes first)", got)
			}
		})
	}
}
