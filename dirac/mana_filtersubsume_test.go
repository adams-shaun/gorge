package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestFiltSubsumeColourAsk: the filter search's subsume run keeps only the
// colours the floating pool still lacks of the play's coloured-pip-only cost
// (gorge potentialPrefixScript's third search, subsumeAnswers). A covered
// colour pays nothing more than a lacking one, so its line is dominated; the
// plain run keeps every answer whole. The ask is a live filter gap's colour
// ask, and the compared answers differ: the pool holds the green pip and
// lacks the blue.
func TestFiltSubsumeColourAsk(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Subtypes = append([]string(nil), v.Table.Subtypes...)
	tbl.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	tap := kop.Cost{Flags: kop.CostTap}
	filt := kop.CardDef{Name: "Subsume Filter", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1}, // {T}: Add {C}
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}},
			ManaColors: kop.ColAny, ManaN: 1}, // {1}, {T}: Add one mana of any color
	}}
	fid := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, filt)
	sc := newScenTableLib(t, &tbl, v.Plains, Config{NoElide: true})
	s, k := sc.s, sc.k
	o := sc.perm(fid, 0)
	sc.perm(v.Plains, 0) // one other mana source pays the filter's {1} fee
	sl := sc.slot(o)
	// PRECONDITION: the card is a live filter gap, the ask the run narrows.
	if !s.scriptFilt(k, 0).has(sl) {
		t.Fatalf("the filter card is not a live filter gap (filt %v); filterTapMana=%v firstManaAb=%d flags=%#x filt=%d ownerSlots=%d", s.scriptFilt(k, 0), filterTapMana(&tbl.Cards[fid], &tbl.Cards[fid].Abilities[1]), firstManaAb(&tbl.Cards[fid]), k.gen.flags[fid], k.gen.filt(fid), s.ctrlM[0].count())
	}
	fs := filtSearch{p: 0, subsume: true, costCol: [kop.NumManaKinds]uint8{kop.G: 1, kop.U: 1}}
	s.pl[0].pool[kop.G] = 1
	// PRECONDITION: the answers differ: the pool covers green and lacks blue.
	if fs.subsumeKeeps(s, kop.G) {
		t.Fatal("the subsume run kept a covered colour")
	}
	if !fs.subsumeKeeps(s, kop.U) {
		t.Fatal("the subsume run dropped a colour the pool lacks")
	}
	plain := fs
	plain.subsume = false
	if !plain.subsumeKeeps(s, kop.G) || !plain.subsumeKeeps(s, kop.U) {
		t.Fatal("the plain run pruned a colour ask (gorge branches on it whole)")
	}
}
