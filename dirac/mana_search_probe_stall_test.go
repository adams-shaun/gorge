package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestScriptDFSStallPropagates: the window prefix DFS (scriptDFS) is the
// comparison search of scriptDeadEndAny. Its only gap is the Test Ctr Source
// from ctrStallScen, whose every chain tap latches StallTapExtra on the node's
// fork (tapExtraStall: the reflected carrier's extra is not added by the
// counter-cost activation). The DFS declares the stall on the real state at
// its leaf exhaustion, not as "no payment": the game draws as the unmodelled
// stall instead of silently losing the cast gorge offers
// (TestTapXScriptStallPropagates's mechanism).
func TestScriptDFSStallPropagates(t *testing.T) {
	sc, emberO, sl := ctrStallScen(t, false)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if !s.scriptCtr(k, 0).has(sl) {
		t.Fatal("precondition: the source is no live counter-cost gap")
	}
	// only the counter-cost gap: the DFS's own order over it
	if !s.scriptDeadEndAny(k, a, 0, slotMask{}, slotMask{}, slotMask{}, s.scriptCtr(k, 0), slotMask{}) {
		t.Fatal("the DFS paid an unpayable {C}{G} cast")
	}
	if !s.Stalled() || s.StallReason() != StallTapExtra {
		t.Fatalf("the DFS dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallTapExtra))
	}
}

// TestWindowAffordsTakeStallPropagates: the unless / ward window's
// affordability probe (windowAffordsTake) runs the window walk on a fork. A
// failed walk's latched stall is returned to the caller, never declared by the
// probe (a caller that keeps searching may still price the play through a
// sibling route); posePay's final drop declares it. On ctrStallScen the only
// source is the counter-cost one under the reflected tap-extra carrier: the
// walk taps it, latches StallTapExtra, and an unless {3} fails.
func TestWindowAffordsTakeStallPropagates(t *testing.T) {
	sc, _, _ := ctrStallScen(t, false)
	s, k := sc.s, sc.k
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	cost := manaOf(3, 0)
	ok, stall := s.windowAffordsTake(k, 0, &cost, true, true)
	if ok {
		t.Fatal("the window probe paid an unless {3} with one counter-cost source")
	}
	if stall != StallTapExtra {
		t.Fatalf("the probe reported the failed walk as plain unaffordable: stall %s, want %s",
			StallName(stall), StallName(StallTapExtra))
	}
	if s.Stalled() {
		t.Fatalf("the probe declared its fork's stall itself (want the drop site's): %s", StallName(s.StallReason()))
	}
	// the drop site: the payer is offered decline only and the latched stall is
	// declared on the real state, not silently swallowed
	in := &kop.Instr{}
	s.posePay(k, in, 0, kop.PayMana, pfWard, 3)
	if s.g.pickF&pfNoPay == 0 {
		t.Fatalf("the payer is not offered decline only (pickF %#x)", s.g.pickF)
	}
	if !s.Stalled() || s.StallReason() != StallTapExtra {
		t.Fatalf("the drop site swallowed the probe's stall: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallTapExtra))
	}
}

// gapStallScen: seat 0 holds a storage-battery mana source (a "{T}, remove
// its storage counters: Add that much {G}" ability, kop.StoreManaAb) that a
// reflected tap-extra carrier reaches, its one storage counter, and —
// withFilter — a first-only filter ({U}, {T}: Add {C}{C}{C}) funded by an
// Island. The battery is no plain tap source, so the window's walk picks it
// as a gap and storeGap's tapExtraStall latches StallTapExtra on the fork.
func gapStallScen(t *testing.T, withFilter bool) (*scen, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	battery := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Battery", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, ManaColors: kop.ColG, AFlags: kop.AfManaDynamic,
			ManaV: kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrStorage)},
			Cost:  kop.Cost{Flags: kop.CostTap | kop.CostLoyaltyX, Ctr: kop.CtrStorage}},
	}})
	judge := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Judge", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect, Filter: mine}},
	}})
	if withFilter {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Filter", Types: kop.TArtifact, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1}}},
				ManaColors: kop.ColC, ManaN: 3},
		}})
	}
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	o := sc.perm(battery, 0)
	sl := sc.slot(o)
	sc.s.perms[sl].seeCtr(sc.s)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 1)
	sc.perm(judge, 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	if !sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Fatal("precondition: the battery is not a live counter-cost gap")
	}
	if withFilter {
		sc.perm(v.Island, 0)
		if !sc.s.scriptFilt(sc.k, 0).any() {
			t.Fatal("precondition: the table lists no live filter")
		}
	}
	return sc, sl
}

// TestHybStrandStallPropagates: a hybrid cast's manual window dry-runs
// (windowDry) and, when the walk would take a gap (the storage battery),
// probes the real walk on a fork (tapForHyb). The probe taps the battery and
// latches StallTapExtra; no witness pays the {U/R}{U/R}, so the cast is
// stranded and tapForHyb declares the probe's stall before reversing it: the
// game draws as the unmodelled stall instead of a silent reversal.
func TestHybStrandStallPropagates(t *testing.T) {
	sc, _ := filtRestrStallScen(t, kop.ColR|kop.ColG)
	s, k := sc.s, sc.k
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	// A red pip no source makes: the dry walk stops at the untapped R/G
	// restricted source (a gap), and the full probe's batch allocation fails.
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.R: 2}}
	if s.tapForHyb(k, 0, &cost, 0, NoObj, false) {
		t.Fatal("tapForHyb paid a two-red cost no source makes")
	}
	if !s.Stalled() || s.StallReason() != StallRestrSrc {
		t.Fatalf("the strand dropped the probe's stall silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallRestrSrc))
	}
}

// filtRestrStallScen: seat 0 holds nine restricted sources with distinct
// restriction kinds (each "{T}: Add {W} or {U}, spend only on your
// instants", the non-spell flag so an unless/ward window admits them) and a
// first-only filter ({U}, {T}: Add {C}{C}{C}) with an Island to fund its fee.
// One source's mana per restriction kind is already floating restricted
// until the kind table is full, so the next untapped source's window
// activation cannot allocate a batch: windowGapAnyColour latches
// StallRestrSrc on the walk's fork.
func filtRestrStallScen(t *testing.T, mask kop.ColorMask) (*scen, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	var srcs [32]CardID
	for i := range srcs {
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Ctrl: kop.CtrlYou})
		fid := kop.FilterID(len(tbl.Filters) - 1)
		srcs[i] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Courtyard " + string(rune('A'+i)), Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: mask, ManaN: 1, ManaRestrict: fid},
		}})
	}
	filter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Filter", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1}}},
			ManaColors: kop.ColC, ManaN: 3},
	}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	last := NoSlot
	for i := range srcs {
		sl := sc.slot(sc.perm(srcs[i], 0))
		c := sc.s.cardAt(sl)
		ab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
		if !sc.s.floatRestrAb(sc.k, 0, sl, ab, int(kop.W), 1) {
			last = sl // the restriction-kind table is full: this source's window activation latches
			break
		}
		sc.s.perms[sl].flags |= pfTapped // the tap that produced it
	}
	if last == NoSlot {
		t.Fatal("precondition: every restricted source's mana floated (the kind table never filled)")
	}
	sc.perm(filter, 0)
	sc.perm(v.Island, 0) // the filter's {U} fee
	if !sc.k.gen.anyFiltFirst {
		t.Fatal("precondition: the table lists no first-only filter")
	}
	if !sc.s.scriptFilt(sc.k, 0).any() {
		t.Fatal("precondition: the filter is no live census gap")
	}
	if sc.s.restrN(0) == 0 {
		t.Fatal("precondition: no restricted batch floating")
	}
	return sc, last
}

// TestWindowFiltAffordsStallPropagates: the ward window's filter-first
// affordability probe (windowFiltAffords) runs the window walk on a fork.
// A failed walk's latched stall is returned to the caller, never declared by
// the probe (the caller's disjunction may still price the ward through a
// sibling route). On the board the walk takes the first not-yet-floated restricted source,
// whose batch allocation fails (StallRestrSrc), and a ward {20} stays short.
func TestWindowFiltAffordsStallPropagates(t *testing.T) {
	sc, _ := filtRestrStallScen(t, kop.ColW|kop.ColU)
	s, k := sc.s, sc.k
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	cost := manaOf(20, 0)
	ok, stall := s.windowFiltAffords(k, 0, &cost)
	if ok {
		t.Fatal("the window probe paid a ward {20} with twelve mana")
	}
	if stall != StallRestrSrc {
		t.Fatalf("the probe reported the failed walk as plain unaffordable: stall %s, want %s",
			StallName(stall), StallName(StallRestrSrc))
	}
	if s.Stalled() {
		t.Fatalf("the probe declared its fork's stall itself (want the drop site's): %s", StallName(s.StallReason()))
	}
}
