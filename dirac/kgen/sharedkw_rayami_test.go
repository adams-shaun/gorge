package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const rayami = "Rayami, First of the Fallen"

// TestRayamiLowers: the shared-keywords static lowers to one Continuous grant
// per keyword (the Protection family to its three modelled spellings), each
// gated on an ExStaticExileKw count, and builds without a refusal. The
// replacement half (a dies-to-exile counter rider) lowers beside it.
func TestRayamiLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{rayami})
	if len(lw.cards) != 1 {
		t.Fatalf("got %d cards", len(lw.cards))
	}
	c := lw.cards[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	// Flying First Strike Double Strike Deathtouch Haste Hexproof Indestructible
	// Lifelink Menace Reach Trample Vigilance, and Protection's three spellings
	const want = 12 + 3
	if len(c.Statics) != want {
		t.Fatalf("statics = %d, want %d", len(c.Statics), want)
	}
	for _, st := range c.Statics {
		if st.Cond.Kind != cValue || st.Cond.A.Kind != vExileKw {
			t.Errorf("static %v: cond %v is not an exile keyword count", st.Kw, st.Cond)
		}
	}
	if len(c.Repls) != 1 || c.Repls[0].Kind != "ExileOnLeave" || c.Repls[0].Ctr == "" {
		t.Errorf("repls = %+v, want one ExileOnLeave with a counter", c.Repls)
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs[0].refusals) != 0 {
		t.Fatalf("build refusals = %v", bs[0].refusals)
	}
	grants, gates := 0, 0
	for _, ab := range bs[0].def.Abilities {
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatContinuous || ab.Static.Kw == 0 {
			continue
		}
		grants++
		cd := pt.conds[ab.Static.Cond2]
		if cd.Kind == kop.CondValue && cd.A.K == kop.VExpr && pt.exprs[cd.A.N].Kind == kop.ExStaticExileKw {
			gates++
		}
	}
	if grants != gates || grants == 0 {
		t.Errorf("%d keyword grants, %d gated on ExStaticExileKw", grants, gates)
	}
}

// TestRayamiPoolRefusal: the family words share the exiled card's OWN
// spelling in gorge, so a pool creature printing one the kernel cannot grant
// (Black Knight's protection from white) refuses the card by name, while a
// modelled spelling (White Knight's protection from black) does not.
func TestRayamiPoolRefusal(t *testing.T) {
	reasons := func(extra string) []string {
		lw := lowerNamedPool(t, []string{rayami, extra})
		for _, c := range lw.cards {
			if c.Name == rayami {
				var rs []string
				for _, r := range c.Refusals {
					rs = append(rs, r.Reason)
				}
				return rs
			}
		}
		t.Fatalf("%s not lowered", rayami)
		return nil
	}
	if rs := reasons("White Knight"); len(rs) != 0 {
		t.Errorf("with White Knight: refusals %q", rs)
	}
	rs := reasons("Black Knight")
	if len(rs) != 1 || !strings.Contains(rs[0], "Black Knight") || !strings.Contains(rs[0], "Protection from white") {
		t.Errorf("with Black Knight: refusals %q, want one naming Black Knight's protection from white", rs)
	}
}
