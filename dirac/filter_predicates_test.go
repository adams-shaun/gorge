package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMultiColorHistoricEquippedPredicates: PredMultiColor (gorge wordMultiColor), PredHistoric
// (gorge historicMatches) and PredEquipped (gorge equipped) on the
// battlefield and off it.
func TestMultiColorHistoricEquippedPredicates(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Baylen, the Haymaker", Count: 1}, {Name: "Downwind Ambusher", Count: 1}, {Name: "Short Bow", Count: 1}, {Name: "Plains", Count: 8}})
	add := func(p kop.Pred) kop.FilterID {
		sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: p})
		return kop.FilterID(len(sc.k.t.Filters) - 1)
	}
	multi, hist, eq := add(kop.PredMultiColor), add(kop.PredHistoric), add(kop.PredEquipped)
	baylen, amb, bow := sc.perm(id["Baylen, the Haymaker"], 0), sc.perm(id["Downwind Ambusher"], 0), sc.perm(id["Short Bow"], 0)
	for _, c := range []struct {
		f    kop.FilterID
		o    Obj
		want bool
	}{{multi, baylen, true}, {multi, amb, false}, {hist, baylen, true}, {hist, bow, true}, {hist, amb, false}, {eq, amb, false}} {
		if got := sc.s.matchPerm(sc.k, c.f, sc.slot(c.o), 0, NoSlot); got != c.want {
			t.Errorf("filter %d on %d: got %v want %v", c.f, c.o, got, c.want)
		}
	}
	sc.s.perms[sc.slot(bow)].attached = sc.slot(amb)
	if !sc.s.matchPerm(sc.k, eq, sc.slot(amb), 0, NoSlot) || sc.s.matchPerm(sc.k, eq, sc.slot(baylen), 0, NoSlot) {
		t.Error("equipped does not read the attached Equipment")
	}
	if !sc.s.matchCard(sc.k, multi, id["Baylen, the Haymaker"], 0, 0) || sc.s.matchCard(sc.k, multi, id["Downwind Ambusher"], 0, 0) ||
		!sc.s.matchCard(sc.k, hist, id["Short Bow"], 0, 0) || sc.s.matchCard(sc.k, hist, id["Downwind Ambusher"], 0, 0) ||
		sc.s.matchCard(sc.k, eq, id["Downwind Ambusher"], 0, 0) {
		t.Error("the card matcher misreads a predicate")
	}
}

// TestCreateTokenRememberObj: TokRememberObj appends every created token to
// the remembered set (gorge RememberTokens$), after what it already holds;
// without it nothing joins.
func TestCreateTokenRememberObj(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Carrot Cake", Count: 1}, {Name: "Plains", Count: 8}})
	tok := CardID(0)
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Types&kop.TToken != 0 {
			tok = CardID(i)
			break
		}
	}
	if tok == 0 {
		t.Fatal("no token in the BLB table")
	}
	sc.k.t.Exprs = append(sc.k.t.Exprs, kop.Expr{Kind: kop.ExConst, N: 2})
	two := kop.ValueRef{K: kop.VExpr, N: int16(len(sc.k.t.Exprs) - 1)}
	sc.s.res.e.ctrl = 0
	sc.s.g.nrem = 0
	in := kop.Instr{Op: kop.OpCreateTokenEx, Obj: kop.RefController, A: two, C: int16(tok), B: kop.Const(kop.TokRemember)}
	sc.s.execGen(sc.k, &in)
	if sc.s.g.nrem != 0 {
		t.Fatalf("TokRemember alone remembered %d objects", sc.s.g.nrem)
	}
	earlier := sc.perm(id["Plains"], 0)
	sc.s.remember(earlier)
	in.B = kop.Const(kop.TokRemember | kop.TokRememberObj)
	sc.s.execGen(sc.k, &in)
	if sc.s.g.nrem != 3 || sc.s.g.rem[0] != earlier || sc.s.objs[sc.s.g.rem[1]].card != tok || sc.s.objs[sc.s.g.rem[2]].card != tok {
		t.Fatalf("remembered %v, want the earlier object then both tokens", sc.s.g.rem[:sc.s.g.nrem])
	}
}

// TestAttacksAlone: Tf2Alone (gorge Alone$ True) fires "whenever a creature
// you control attacks alone" only when the declaration has one attacker.
func TestAttacksAloneFlag(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	for _, n := range []int{1, 2} {
		deck := []cardset.Entry{{Name: "Plains", Count: 20}}
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
		bear, watcher := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			kop.CardDef{Name: "Test Exalter", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttacks, Filter: kop.FilterID(len(tbl.Filters) - 1), Flags2: kop.Tf2Alone}, Prog: gain}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(watcher, 0)
		var atk []Obj
		for i := 0; i < n; i++ {
			atk = append(atk, sc.perm(bear, 0))
		}
		sc.combat(t, atk, nil)
		want, opp := int16(20), int16(20-2*n)
		if n == 1 {
			want = 21
		}
		if l, o := sc.s.pl[0].life, sc.s.pl[1].life; l != want || o != opp || sc.s.npend != 0 {
			t.Errorf("%d attackers: life %d / %d, %d pending; want %d / %d, none pending", n, l, o, sc.s.npend, want, opp)
		}
	}
}

// TestAttacksAttacked: Tf2AtkYou / Tf2AtkOpp (gorge Attacked$ You /
// Player.Opponent) read the attacked player from the trigger controller's
// seat; Tf2AtkYourPW alone never matches a player defender.
func TestAttacksAttacked(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	for _, c := range []struct {
		f        kop.TrigFlags2
		owner    uint8
		triggers bool
	}{{kop.Tf2AtkYou, 1, true}, {kop.Tf2AtkYou, 0, false}, {kop.Tf2AtkOpp, 0, true}, {kop.Tf2AtkOpp, 1, false}, {kop.Tf2AtkYourPW, 1, false}} {
		deck := []cardset.Entry{{Name: "Plains", Count: 20}}
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
		bear, watcher := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttacks, Filter: kop.FilterID(len(tbl.Filters) - 1), Flags2: c.f}, Prog: gain}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(watcher, c.owner)
		sc.combat(t, []Obj{sc.perm(bear, 0)}, nil)
		want := int16(20) // the attacker's controller
		if c.owner == 1 {
			want = 18 // the defender
		}
		if c.triggers {
			want++
		}
		if l := sc.s.pl[c.owner].life; l != want {
			t.Errorf("flags %#x, watcher of player %d: life %d, want %d", c.f, c.owner, l, want)
		}
	}
}

// TestWhoseEnchCtrl: kop.WhoseEnchCtrl (gorge Player.EnchantedController)
// names the controller of the permanent the listener is attached to; it
// fails closed through whoseOK (no slot) and when unattached.
func TestWhoseEnchCtrl(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 1}, {Name: "Short Bow", Count: 1}, {Name: "Plains", Count: 8}})
	bow, amb := sc.slot(sc.perm(id["Short Bow"], 0)), sc.slot(sc.perm(id["Downwind Ambusher"], 1))
	if whoseOK(kop.WhoseEnchCtrl, 0, 0) || sc.s.enchCtrlIs(bow, 1) || sc.s.enchCtrlIs(NoSlot, 1) {
		t.Fatal("an unattached or slotless EnchantedController matched")
	}
	sc.s.perms[bow].attached = amb
	if !sc.s.enchCtrlIs(bow, 1) || sc.s.enchCtrlIs(bow, 0) {
		t.Fatal("EnchantedController is not the bearer's controller")
	}
}

// TestAttackersDeclaredFilter: an AttackersDeclared trigger counts only the
// attackers matching its ValidAttackers$ filter (gorge
// AttackersDeclaredMatchedAttackers): none matching, no trigger.
func TestAttackersDeclaredFilter(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	for _, flier := range []bool{false, true} {
		deck := []cardset.Entry{{Name: "Plains", Count: 20}}
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
		bear, watcher := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags2: kop.F2WithFlying})
		bd := kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2}
		if flier {
			bd.Keywords = kop.KwFlying
		}
		tbl.Cards = append(tbl.Cards, bd,
			kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttackersDeclared, Filter: kop.FilterID(len(tbl.Filters) - 1), Whose: kop.WhoseYou}, Prog: gain}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(watcher, 0)
		sc.combat(t, []Obj{sc.perm(bear, 0)}, nil)
		want := int16(20)
		if flier {
			want = 21
		}
		if l := sc.s.pl[0].life; l != want || sc.s.pl[1].life != 18 {
			t.Errorf("flier=%v: life %d / %d, want %d / 18", flier, l, sc.s.pl[1].life, want)
		}
	}
}

// TestExprCompare: kop.ExCompare (gorge evalCompare) picks the then arm when
// the compared value meets the threshold under Op, else the else arm.
func TestExprCompare(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	x := &sc.k.t.Exprs
	*x = append(*x, kop.Expr{Kind: kop.ExBranches, Sub: [2]kop.ValueRef{kop.Const(10), kop.Const(20)}})
	br := kop.ValueRef{K: kop.VExpr, N: int16(len(*x) - 1)}
	for _, c := range []struct {
		op   kop.CmpOp
		want int
	}{{kop.CmpEQ, 10}, {kop.CmpGE, 10}, {kop.CmpLE, 10}, {kop.CmpGT, 20}, {kop.CmpLT, 20}} {
		*x = append(*x, kop.Expr{Kind: kop.ExCompare, N: 3, Op: c.op, Sub: [2]kop.ValueRef{kop.Const(3), br}})
		v := kop.ValueRef{K: kop.VExpr, N: int16(len(*x) - 1)}
		if got := sc.s.evalV(sc.k, v, evalCtx{}); got != c.want {
			t.Errorf("3 %v 3: got %d, want %d", c.op, got, c.want)
		}
	}
}

// TestAttacksAlone: an Attacks trigger with Tf2Alone (gorge Alone$ True,
// the Exalted expansion) fires only when exactly one creature attacks.
func TestAttacksAlone(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Agate-Blade Assassin", Count: 2}, {Name: "Plains", Count: 8}})
	c := id["Agate-Blade Assassin"]
	abs := append([]kop.Ability(nil), sc.k.t.Cards[c].Abilities...)
	abs[0].Trig.Flags2 |= kop.Tf2Alone
	sc.k.t.Cards[c].Abilities = abs
	a, b := sc.perm(c, 0), sc.perm(c, 0)
	declare := func(objs ...Obj) int {
		for _, o := range []Obj{a, b} {
			sc.s.perms[sc.slot(o)].flags &^= pfAttacking
		}
		for _, o := range objs {
			p := &sc.s.perms[sc.slot(o)]
			p.flags |= pfAttacking
			p.atk = TCode(PlayerTarget1)
		}
		sc.s.nAttackers = uint8(len(objs))
		sc.s.npend = 0
		sc.s.fireAttacksGen(sc.k)
		return int(sc.s.npend)
	}
	if n := declare(a); n != 1 {
		t.Fatalf("one attacker alone: %d triggers, want 1", n)
	}
	if n := declare(a, b); n != 0 {
		t.Fatalf("two attackers: %d Alone triggers, want 0", n)
	}
	abs[0].Trig.Flags2 &^= kop.Tf2Alone
	if n := declare(a, b); n != 2 {
		t.Fatalf("two attackers without Alone: %d triggers, want 2", n)
	}
}

// TestPredEnchanted: PredEnchanted (gorge enchanted: an Aura permanent is
// attached) matches a creature with an Aura attached, not one without, and
// never a card off the battlefield.
func TestPredEnchanted(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Feather of Flight", Count: 1}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredEnchanted})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	a, b := sc.perm(id["Valley Floodcaller"], 0), sc.perm(id["Valley Floodcaller"], 0)
	aura := sc.perm(id["Feather of Flight"], 0)
	if sc.s.matchPerm(sc.k, f, sc.slot(a), 0, NoSlot) {
		t.Fatal("an unenchanted creature matched")
	}
	sc.s.perms[sc.slot(aura)].attached = sc.slot(a)
	if !sc.s.matchPerm(sc.k, f, sc.slot(a), 0, NoSlot) || sc.s.matchPerm(sc.k, f, sc.slot(b), 0, NoSlot) {
		t.Fatal("enchanted does not read the attached Aura")
	}
	if sc.s.matchCard(sc.k, f, id["Valley Floodcaller"], 0, 0) {
		t.Fatal("a card off the battlefield matched enchanted")
	}
}

// TestPredHasKeyword: the with<Keyword> filter (PredHasKeyword, PredN the
// keyword's bit index) reads the derived keywords on the battlefield and
// the printed ones off it (gorge objectHasKeyword).
func TestPredHasKeyword(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Wall", Types: kop.TCreature, Power: 0, Toughness: 4, Keywords: kop.KwDefender},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Pred: kop.PredHasKeyword, PredN: 10})
	f := kop.FilterID(len(tbl.Filters) - 1)
	if kop.KwDefender != 1<<10 {
		t.Fatal("KwDefender is not bit 10")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	w, b := sc.perm(n, 0), sc.perm(n+1, 0)
	if !sc.s.matchPerm(sc.k, f, sc.slot(w), 0, NoSlot) || sc.s.matchPerm(sc.k, f, sc.slot(b), 0, NoSlot) {
		t.Fatal("withDefender on the battlefield")
	}
	if !sc.s.matchCard(sc.k, f, n, 0, 0) || sc.s.matchCard(sc.k, f, n+1, 0, 0) {
		t.Fatal("withDefender off the battlefield")
	}
}

// TestPredAttachedActiveCtrl: PredAttached matches an Aura attached to the source,
// not one on something else, an unattached one nor a card; PredActiveCtrl matches the active player's
// permanents and, off the battlefield, the cards they own.
func TestPredAttachedActiveCtrl(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Feather of Flight", Count: 2}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredAttached}, kop.Filter{Pred: kop.PredActiveCtrl})
	att, act := kop.FilterID(len(sc.k.t.Filters)-2), kop.FilterID(len(sc.k.t.Filters)-1)
	fc, fof := id["Valley Floodcaller"], id["Feather of Flight"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 1)
	aura, loose := sc.perm(fof, 0), sc.perm(fof, 0)
	sc.s.perms[sc.slot(aura)].attached = sc.slot(a)
	if !sc.s.matchPerm(sc.k, att, sc.slot(aura), 0, sc.slot(a)) || sc.s.matchPerm(sc.k, att, sc.slot(aura), 0, sc.slot(b)) ||
		sc.s.matchPerm(sc.k, att, sc.slot(loose), 0, sc.slot(a)) || sc.s.matchPerm(sc.k, att, sc.slot(aura), 0, NoSlot) ||
		sc.s.matchCard(sc.k, att, fof, 0, 0) {
		t.Fatal("Attached")
	}
	sc.s.active = 0
	if !sc.s.matchPerm(sc.k, act, sc.slot(a), 1, NoSlot) || sc.s.matchPerm(sc.k, act, sc.slot(b), 0, NoSlot) ||
		!sc.s.matchCard(sc.k, act, fc, 0, 1) || sc.s.matchCard(sc.k, act, fc, 1, 0) {
		t.Fatal("ActivePlayerCtrl")
	}
}

// TestHasCountersPredicate: PredHasCounters (gorge wordHasCounters:
// hasRealCounters) reads a counter of any stored kind, but not the regeneration
// shield or the deathtouch mark (CR 122.1), a Class level, an attachment, or
// anything off the battlefield.
func TestHasCountersPredicate(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 1}, {Name: "Short Bow", Count: 1}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredHasCounters})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	amb, bow := sc.perm(id["Downwind Ambusher"], 0), sc.perm(id["Short Bow"], 0)
	s := sc.s
	has := func() bool { return s.matchPerm(sc.k, f, sc.slot(amb), 0, NoSlot) }
	if has() {
		t.Fatal("a bare creature has counters")
	}
	p := &s.perms[sc.slot(amb)]
	p.addCtrN(s, int(kop.CtrP1P1), 1)
	if !has() {
		t.Error("a +1/+1 counter is not read")
	}
	p.subCtrN(s, int(kop.CtrP1P1), 1)
	p.addCtrN(s, int(kop.CtrStun), 1)
	if !has() {
		t.Error("a stun counter is not read")
	}
	p.subCtrN(s, int(kop.CtrStun), 1)
	if has() {
		t.Error("a drained kind still reads as a counter")
	}
	p.addCtrN(s, int(kop.CtrLevel), 1)
	if has() {
		t.Error("a Class level counts as a counter")
	}
	p.subCtrN(s, int(kop.CtrLevel), 1)
	p.flags |= pfRegen
	if has() {
		t.Error("the regeneration shield reads as a counter")
	}
	p.flags &^= pfRegen
	p.markDT(1)
	if has() {
		t.Error("the deathtouch mark reads as a counter")
	}
	p.flags &^= pfDeathtouched
	s.perms[sc.slot(bow)].attached = sc.slot(amb)
	if has() {
		t.Error("an attachment reads as a counter")
	}
	if s.matchCard(sc.k, f, id["Downwind Ambusher"], 0, 0) {
		t.Error("a card off the battlefield has counters")
	}
}

// TestModifiedPredicate: PredModified (CR 700.9; gorge modifiedPermanent) reads a
// counter of any stored kind but not a Class level, an attached Equipment,
// and an attached Aura only while the permanent's controller controls it;
// nothing off the battlefield is modified.
func TestModifiedPredicate(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 1}, {Name: "Short Bow", Count: 1}, {Name: "Feather of Flight", Count: 1}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredModified})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	amb, bow, feather := sc.perm(id["Downwind Ambusher"], 0), sc.perm(id["Short Bow"], 0), sc.perm(id["Feather of Flight"], 0)
	s := sc.s
	mod := func() bool { return s.matchPerm(sc.k, f, sc.slot(amb), 0, NoSlot) }
	if mod() {
		t.Fatal("a bare creature is modified")
	}
	p := &s.perms[sc.slot(amb)]
	p.addCtrN(s, int(kop.CtrP1P1), 1)
	if !mod() {
		t.Error("a +1/+1 counter does not modify")
	}
	p.subCtrN(s, int(kop.CtrP1P1), 1)
	p.addCtrN(s, int(kop.CtrStun), 1)
	if !mod() {
		t.Error("a stun counter does not modify")
	}
	p.subCtrN(s, int(kop.CtrStun), 1)
	p.addCtrN(s, int(kop.CtrLevel), 1)
	if mod() {
		t.Error("a Class level counts as a counter")
	}
	p.subCtrN(s, int(kop.CtrLevel), 1)
	p.flags |= pfRegen
	if mod() {
		t.Error("a regeneration shield counts as a counter")
	}
	p.flags &^= pfRegen
	s.perms[sc.slot(bow)].attached = sc.slot(amb)
	if !mod() {
		t.Error("an attached Equipment does not modify")
	}
	s.perms[sc.slot(bow)].attached = NoSlot
	s.perms[sc.slot(feather)].attached = sc.slot(amb)
	if !mod() {
		t.Error("an attached Aura the controller controls does not modify")
	}
	s.perms[sc.slot(feather)].ctrl = 1
	if mod() {
		t.Error("an opponent's Aura modifies")
	}
	if s.matchCard(sc.k, f, id["Downwind Ambusher"], 0, 0) {
		t.Error("a card off the battlefield is modified")
	}
}

// Attached reads the source (gorge objectAttachedToSource): Zack Fair moves only the
// Equipment that was on him as he was sacrificed, and leaves another creature's.
func TestAttachedReadsTheSource(t *testing.T) {
	for _, equipped := range []bool{false, true} {
		set := cardsettest.Set(t, "attachchoices")
		deck := []cardset.Entry{{Name: "Zack Fair", Count: 1}, {Name: "Bonesplitter", Count: 1}, {Name: "Plains", Count: 1}, {Name: "Grizzly Bears", Count: 1}}
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(id["Plains"], 0)
		zack, mine := sc.perm(id["Zack Fair"], 0), sc.perm(id["Grizzly Bears"], 0)
		theirs, glove := sc.perm(id["Grizzly Bears"], 1), sc.perm(id["Bonesplitter"], 1)
		sc.s.perms[sc.slot(glove)].attached = sc.slot(theirs)
		var own Obj
		if equipped {
			own = sc.perm(id["Bonesplitter"], 0)
			sc.s.perms[sc.slot(own)].attached = sc.slot(zack)
		}
		if !sc.activateAb(zack, 0) {
			t.Fatal("Zack's ability not offered")
		}
		sc.s.Step()
		sc.s.Apply(MakeAction(AkTarget, 0, TCode(mine), 0))
		sc.resolveAll(t)
		if sc.onBF(zack) {
			t.Fatal("Zack was not sacrificed")
		}
		if got := sc.s.perms[sc.slot(glove)].attached; got != sc.slot(theirs) {
			t.Fatalf("equipped=%v: the opponent's Equipment is on slot %d", equipped, got)
		}
		if equipped && sc.s.perms[sc.slot(own)].attached != sc.slot(mine) {
			t.Fatalf("Zack's Equipment is on slot %d, want the target", sc.s.perms[sc.slot(own)].attached)
		}
	}
}
