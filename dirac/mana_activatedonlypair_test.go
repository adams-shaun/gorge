package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckActivatedOnlyPair is the kernel-side half of
// TestActivatedOnlyRestrictedPair (dirac_test): Sunken Citadel ({T}: Add one
// mana of the chosen colour; {T}: Add two of it, spend only to activate
// abilities of land sources) is a restricted pair unit whose larger
// alternative takes the permanent's chosen colour at runtime, and Steelswarm
// Operator ({T}: Add {U} for artifact spells; {T}: Add {U}{U} for artifact
// abilities) holds two restricted abilities, of which a payment plans the one
// its class admits (CR 106.6, 601.2g-h, 602.2b).
func CheckActivatedOnlyPair(t *testing.T, tbl *kop.Table, citadel, operatorID, mountain, stone, bolt, artSpell CardID) {
	t.Helper()
	sc := newScenTableLib(t, tbl, mountain, Config{})
	g := sc.k.gen

	// (a) Citadel: one widened record; its view makes the chosen colour, two
	// of it for an activation of a land source, none for a spell.
	co := sc.perm(citadel, 0)
	csl := sc.slot(co)
	sc.s.perms[csl].chosen = 3         // red
	sc.s.perms[csl].flags &^= pfTapped // it enters tapped: the view needs it usable
	rec := g.restrRecOf(citadel)
	if rec == nil || rec.kind != rkWiden {
		t.Fatalf("Citadel's restricted record = %+v, want a widened source", rec)
	}
	v := &g.rs.views[rec.view]
	if !v.wideChosen || v.wideN != 2 {
		t.Fatalf("Citadel's view wideN %d wideChosen %v, want 2 chosen", v.wideN, v.wideChosen)
	}
	var base, got avail
	sc.s.availFor(sc.k, 0, &base)
	if base.total != 1 {
		t.Fatalf("Citadel alone plans %d mana, want its census one", base.total)
	}
	land := sc.perm(mountain, 0)
	artifact := sc.perm(stone, 0)
	dAct := payDesc{class: pdAct, src: sc.slot(land)}
	if _, kd, ab := sc.s.restrView(sc.k, 0, csl, sc.s.manaPCGen(sc.k, csl), &dAct); kd != rkWiden || ab == nil {
		t.Fatalf("a land ability does not admit Citadel's restricted ability (kind %d): the test reads nothing", kd)
	}
	got, _, _ = sc.s.restrSrcAvail(sc.k, 0, dAct, &base, NoSlot)
	if got.total != 2 || got.mask[got.n-1]&(1<<3) == 0 {
		t.Errorf("Citadel for a land ability: %d mana %v, want its one replaced by two {R}", got.total, got.mask[:got.n])
	}
	dSpell := payDesc{class: pdSpell, card: bolt, src: NoSlot}
	if _, kd, _ := sc.s.restrView(sc.k, 0, csl, sc.s.manaPCGen(sc.k, csl), &dSpell); kd != 0 {
		t.Errorf("a spell payment admits Citadel's activation-only ability (kind %d)", kd)
	}
	got, _, _ = sc.s.restrSrcAvail(sc.k, 0, dSpell, &base, NoSlot)
	if got.total != 1 {
		t.Errorf("Citadel for a spell: %d mana, want its census one only", got.total)
	}
	_ = co

	// (b) Operator: two records; the spell payment admits the {U}, the
	// artifact activation the {U}{U}, and a nonartifact activation neither.
	op := sc.perm(operatorID, 0)
	osl := sc.slot(op)
	n := 0
	for i := range g.rs.recs {
		if g.rs.recs[i].card == operatorID {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("Operator has %d restricted records, want 2", n)
	}
	dArtSpell := payDesc{class: pdSpell, card: artSpell, src: NoSlot}
	if _, kd, ab := sc.s.restrView(sc.k, 0, osl, sc.s.manaPCGen(sc.k, osl), &dArtSpell); kd != rkExt || ab == nil || ab.ManaN != 1 || ab.ManaRestrictAb != 0 {
		t.Errorf("Operator's spell view kind %d ab %+v, want the {U} spell-only ability", kd, ab)
	}
	if v, _, _ := sc.s.restrView(sc.k, 0, osl, sc.s.manaPCGen(sc.k, osl), &dSpell); v != nil {
		t.Errorf("a nonartifact spell is admitted by the Operator's artifact-only {U}")
	}
	got, _, _ = sc.s.restrSrcAvail(sc.k, 0, dArtSpell, &base, NoSlot)
	if got.total != base.total+1 || !availHasMask(got, 1<<1) {
		t.Errorf("Operator for an artifact spell: %d mana %v, want one {U} over the base %d", got.total, got.mask[:got.n], base.total)
	}
	dArt := payDesc{class: pdAct, src: sc.slot(artifact)}
	if _, kd, ab := sc.s.restrView(sc.k, 0, osl, sc.s.manaPCGen(sc.k, osl), &dArt); kd != rkExt || ab == nil || ab.ManaN != 2 {
		t.Fatalf("an artifact activation does not admit the Operator's {U}{U} (kind %d ab %+v)", kd, ab)
	}
	got, _, _ = sc.s.restrSrcAvail(sc.k, 0, dArt, &base, NoSlot)
	if got.total != base.total+2 || !availHasMask(got, 1<<1) {
		t.Errorf("Operator for an artifact activation: %d mana %v, want two {U} over the base %d", got.total, got.mask[:got.n], base.total)
	}
	dLand := payDesc{class: pdAct, src: sc.slot(land)}
	if v, kd, _ := sc.s.restrView(sc.k, 0, osl, sc.s.manaPCGen(sc.k, osl), &dLand); v != nil || kd != 0 {
		t.Errorf("a land activation (not an artifact) admits the Operator's ability (kind %d)", kd)
	}
}

// availHasMask reports whether a holds a unit of one of mask's colours.
func availHasMask(a avail, m uint8) bool {
	for i := 0; i < a.n; i++ {
		if a.mask[i]&m != 0 {
			return true
		}
	}
	return false
}
