package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGrantSubsumeCtrGapPrune: the subsume run's prune reaches the grant
// search's gap branches. gorge's subsumeAnswers (rules/potential_script.go
// scriptSearch.visit) narrows the answers of every mana ask the run poses,
// the counter-cost gap branch's colour asks among them; a branch that
// enumerates the colours itself (ctrBranch) must keep only the colours the
// pool still lacks of the play's coloured-pip-only cost (subsumeCols'
// keep-rule), or the run spends its node budget on the covered colours where
// gorge's pruned walk reaches the paying line.
//
// Four makers ("{T}: Add {B}"; "{T}, Remove a charge counter: Add one mana
// of any color", one counter on each) beside a {G}{G}{U}{U}{U}{U} play: the plain
// run's descent through the printed {B} answers (which pay nothing) exhausts
// its 160-node budget, and the unpruned subsume run's colour asks keep every
// colour, so it exhausts the same way. Pruned, each maker's colour ask keeps
// {G} and {U} while they lack them: the run walks B, B, G, U, U and pays.
func TestGrantSubsumeCtrGapPrune(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Subtypes = append([]string(nil), v.Table.Subtypes...)
	tbl.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	hostSub := uint8(len(tbl.Subtypes))
	tbl.Subtypes = append(tbl.Subtypes, "Subsume Host")
	hostF := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Sub: hostSub + 1, Ctrl: kop.CtrlYou})
	grantCard := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Subsume Grant", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: hostF, Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1},
	}})
	host := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Subsume Host", Types: kop.TLand, Subtypes: kop.SubMask{}.With(int(hostSub))})

	tap := kop.Cost{Flags: kop.CostTap}
	wm := kop.CardDef{Name: "Subsume Wise Maker", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColB, ManaN: 1}, // {T}: Add {B}
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostRemoveCtr, N: 1, Ctr: kop.CtrCharge},
			ManaColors: kop.ColAny, ManaN: 1}, // {T}, Remove a charge counter: Add one mana of any color
	}}
	play := kop.CardDef{Name: "Subsume Play", Types: kop.TSorcery,
		Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 2, kop.U: 4}}}
	wmid := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, wm)
	playid := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, play)

	sc := newScenTableLib(t, &tbl, v.Plains, Config{NoElide: true})
	s, k := sc.s, sc.k
	sc.perm(grantCard, 0)
	ho := sc.perm(host, 0) // two granted {G} sources: the pool covers the {G} pips
	sc.perm(host, 0)       // early, so the pruned colour asks keep {U} alone and the walk is narrow
	for i := 0; i < 5; i++ {
		o := sc.perm(wmid, 0)
		if !sc.s.perms[sc.slot(o)].setCtrN(s, int(kop.CtrCharge), 1) {
			t.Fatal("the maker did not take its charge counter")
		}
	}
	if n := k.ctrAbOf(wmid); n == nil || !ctrAnyColour(n) {
		t.Fatalf("precondition: the maker's counter ability is not an any-colour counter-cost gap (%v)", n == nil)
	}
	playObj := sc.hand(playid, 0)
	a := MakeAction(AkCast, TCode(playObj), 0, 0)
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("grantPlayCost refused the cast")
	}
	d := s.grantPayDesc(k, a)
	// PRECONDITION: the printed sources alone cannot pay {G}{G}{U}{U}{U}{U} — the
	// makers' printed ability is {B} — so a prefix has to float the pips.
	if s.affordsUngrantedPlay(k, 0, a, &cost, ex, d) {
		t.Fatal("the printed sources price the play; the scenario needs the prefix")
	}
	// The plain run exhausts its node budget on the printed {B} answers: a
	// gorge search that ran out (scriptSearch.limited) with no fee-carrying
	// gap to fund (funding == 0) is the subsume run's gate.
	var g grantSrch
	// PRECONDITION: the hosts hold the live grant (the grant search's board)
	// and the makers are the counter-cost gaps the branch walks.
	var gi [maxGrants]int16
	if n := s.grantIdxs(k, sc.slot(ho), &gi); n != 1 {
		t.Fatalf("the host holds %d grants, want the live granted {G}", n)
	}
	if cm := s.scriptCtr(k, 0); cm.count() != 5 {
		t.Fatalf("scriptCtr lists %d makers, want the five counter-cost gaps", cm.count())
	}
	if s.grantSearchRun(k, 0, &cost, ex, d, need, &g, nil) {
		t.Fatalf("the plain run paid {G}{G}{U}{U}{U}{U} within the budget (%d nodes, %d steps)",
			g.nodes, g.np)
	}
	if !g.limited {
		t.Fatalf("the plain run did not exhaust its node budget (%d nodes)", g.nodes)
	}
	// The subsume run: its colour ask of the counter-cost branch keeps only
	// the colours the pool still lacks ({G} and {U}, never the covered {B}),
	// so it walks the paying line inside the budget where the unpruned run
	// spends the budget on the covered colours and drops the play.
	var g2 grantSrch
	if !s.grantSearch(k, 0, &cost, ex, d, need, &g2) {
		t.Fatalf("the subsume run did not pay the play (limited %v, %d nodes, funding %d, stall %d, np %d)", g2.limited, g2.nodes, g2.funding, g2.stall, g2.np)
	}
	if !g2.subsume {
		t.Fatalf("the play was paid by the plain run, not the subsume one (steps %d)", g2.np)
	}
	if g2.nodes >= potentialScriptNodes {
		t.Fatalf("the subsume run reached the budget again (%d nodes), want the prune to walk the paying line", g2.nodes)
	}
	if g2.np < 4 {
		t.Fatalf("the paying prefix has %d steps, want at least the host and two makers' taps", g2.np)
	}
	s.grantApply(k, &g2)
	if p := s.pl[0].pool; p[kop.G] != 2 || p[kop.U] != 4 {
		t.Fatalf("the prefix floated %v, want {G}{G}{U}{U}{U}{U} (the printed {B} floats beside)", p)
	}
}
