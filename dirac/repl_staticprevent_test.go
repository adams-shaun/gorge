package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// spTable is vanilla's table plus the given hosts (one card each, in order)
// and the filters spFilter adds.
type spTable struct {
	tbl kop.Table
}

func newSPTable() *spTable {
	t := &spTable{tbl: *v.Table}
	t.tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	t.tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	t.tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	t.tbl.Conds = []kop.Cond{}
	return t
}

func (t *spTable) filter(f kop.Filter) kop.FilterID {
	t.tbl.Filters = append(t.tbl.Filters, f)
	return kop.FilterID(len(t.tbl.Filters) - 1)
}

// host adds an enchantment carrying the static prevention r.
func (t *spTable) host(name string, r kop.Repl) CardID {
	t.tbl.Cards = append(t.tbl.Cards, kop.CardDef{Name: name, Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: r}}})
	return CardID(len(t.tbl.Cards) - 1)
}

func (t *spTable) scen(tb testing.TB) *scen {
	tb.Helper()
	return newScenTable(tb.(*testing.T), &t.tbl, Config{NoElide: true})
}

// spDeal is the noncombat damage n a source dealt to code after the
// replacements (replDamage, as damageTo reads it).
func spDeal(sc *scen, srcCtrl uint8, src Obj, code TCode, n int) int {
	sl := NoSlot
	card := CardID(0)
	if src != 0 {
		sl = sc.slot(src)
		card = sc.s.cardAt(sl)
	}
	return sc.s.replDamage(sc.k, srcCtrl, sl, card, code, false, n)
}

// TestStaticPreventFilteredRecipients: a full prevention applies to the
// recipients and sources its filters name, and to nothing else.
func TestStaticPreventFilteredRecipients(t *testing.T) {
	const you, them = TCode(PlayerTarget0), TCode(PlayerTarget0 + 1)
	t.Run("recipient creature filter", func(t *testing.T) {
		// Bubble Matrix: all damage that would be dealt to creatures
		tb := newSPTable()
		host := tb.host("Test Matrix", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPAnyPerm,
			Filter: tb.filter(kop.Filter{Types: kop.TCreature})})
		sc := tb.scen(t)
		h := sc.perm(host, 0)
		giant := sc.perm(v.HillGiant, 1)
		bear := sc.perm(v.GrizzlyBears, 0)
		if !sc.onBF(h) || sc.k.gen == nil || !sc.k.gen.uses.has(useStaticPrev) {
			t.Fatal("precondition: the static prevention host is not in play on a table that reads it")
		}
		if got := spDeal(sc, 1, giant, TCode(bear), 3); got != 0 {
			t.Fatalf("damage to a creature = %d, want 0", got)
		}
		if got := spDeal(sc, 1, giant, you, 3); got != 3 {
			t.Fatalf("damage to a player = %d, want 3 (the filter names creatures only)", got)
		}
	})
	t.Run("you creatures, noncombat", func(t *testing.T) {
		// Mark of Asylum: noncombat damage to creatures you control
		tb := newSPTable()
		host := tb.host("Test Asylum", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPNonCombat,
			Filter: tb.filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})})
		sc := tb.scen(t)
		sc.perm(host, 0)
		giant := sc.perm(v.HillGiant, 1)
		mine, theirs := sc.perm(v.GrizzlyBears, 0), sc.perm(v.GrizzlyBears, 1)
		if got := spDeal(sc, 1, giant, TCode(mine), 3); got != 0 {
			t.Fatalf("noncombat damage to my creature = %d, want 0", got)
		}
		if got := spDeal(sc, 1, giant, TCode(theirs), 3); got != 3 {
			t.Fatalf("noncombat damage to their creature = %d, want 3", got)
		}
		// combat damage is not prevented: a combat event is read by replCombatGen
		if got := sc.s.replDamage(sc.k, 1, sc.slot(giant), v.HillGiant, TCode(mine), true, 3); got != 3 {
			t.Fatalf("combat damage through replDamage = %d, want 3", got)
		}
	})
	t.Run("player recipient and gate", func(t *testing.T) {
		// Personal Sanctuary: during your turn, damage to you
		tb := newSPTable()
		host := tb.host("Test Sanctuary", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPYou | kop.ReplSPYourTurn})
		sc := tb.scen(t)
		sc.perm(host, 0)
		giant := sc.perm(v.HillGiant, 1)
		if sc.s.active != 0 {
			t.Fatal("precondition: seat 0 is not the active player")
		}
		if got := spDeal(sc, 1, giant, you, 3); got != 0 {
			t.Fatalf("damage to me on my turn = %d, want 0", got)
		}
		if got := spDeal(sc, 1, giant, them, 3); got != 3 {
			t.Fatalf("damage to the other player = %d, want 3", got)
		}
		sc.s.active = 1
		if got := spDeal(sc, 1, giant, you, 3); got != 3 {
			t.Fatalf("damage to me on their turn = %d, want 3", got)
		}
	})
	t.Run("enchanted source", func(t *testing.T) {
		// Muzzle: all damage dealt by the enchanted creature
		tb := newSPTable()
		host := tb.host("Test Muzzle", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPAnyPerm,
			Filter2: tb.filter(kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})})
		sc := tb.scen(t)
		m := sc.perm(host, 0)
		giant, bear := sc.perm(v.HillGiant, 1), sc.perm(v.GrizzlyBears, 1)
		if got := spDeal(sc, 1, giant, you, 3); got != 3 {
			t.Fatalf("damage by a creature the Aura is not on = %d, want 3", got)
		}
		sc.s.attachTo(sc.k, sc.slot(m), sc.slot(giant))
		if sc.s.perms[sc.slot(m)].attached != sc.slot(giant) {
			t.Fatal("precondition: the Aura is not attached to the Giant")
		}
		if got := spDeal(sc, 1, giant, you, 3); got != 0 {
			t.Fatalf("damage by the enchanted creature = %d, want 0", got)
		}
		if got := spDeal(sc, 1, giant, TCode(bear), 3); got != 0 {
			t.Fatalf("damage by the enchanted creature to a creature = %d, want 0", got)
		}
		if got := spDeal(sc, 1, bear, you, 2); got != 2 {
			t.Fatalf("damage by another creature = %d, want 2", got)
		}
	})
	t.Run("damage amount gate", func(t *testing.T) {
		// Callous Giant: damage of at most 3 to this permanent
		tb := newSPTable()
		host := tb.host("Test Giant", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPYou | kop.ReplSPDmgLE, A: 3})
		sc := tb.scen(t)
		sc.perm(host, 0)
		giant := sc.perm(v.HillGiant, 1)
		if got := spDeal(sc, 1, giant, you, 3); got != 0 {
			t.Fatalf("3 damage = %d, want 0", got)
		}
		if got := spDeal(sc, 1, giant, you, 4); got != 4 {
			t.Fatalf("4 damage = %d, want 4 (past the gate)", got)
		}
	})
}

// TestStaticPreventShieldAmount: a prevent-N shield takes N off each damage
// event it matches, never more than the event holds, and the rest is dealt.
func TestStaticPreventShieldAmount(t *testing.T) {
	const you = TCode(PlayerTarget0)
	t.Run("Sphere of Law", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2,
			Filter2: tb.filter(kop.Filter{Colors: kop.ColR})})
		sc := tb.scen(t)
		sc.perm(host, 0)
		giant, bear := sc.perm(v.HillGiant, 1), sc.perm(v.GrizzlyBears, 1)
		for _, tc := range []struct {
			src     Obj
			n, want int
		}{{giant, 3, 1}, {giant, 2, 0}, {giant, 1, 0}, {bear, 3, 3}} {
			if got := spDeal(sc, 1, tc.src, you, tc.n); got != tc.want {
				t.Errorf("%d damage from slot %d = %d, want %d", tc.n, sc.slot(tc.src), got, tc.want)
			}
		}
	})
	t.Run("all but one and a counted amount", func(t *testing.T) {
		tb := newSPTable()
		hyp := tb.host("Test Hyperion", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPAllBut, A: 1})
		sc := tb.scen(t)
		sc.perm(hyp, 0)
		giant := sc.perm(v.HillGiant, 1)
		for _, tc := range []struct{ n, want int }{{5, 1}, {2, 1}, {1, 1}} {
			if got := spDeal(sc, 1, giant, you, tc.n); got != tc.want {
				t.Errorf("all but one of %d = %d, want %d", tc.n, got, tc.want)
			}
		}
		tb = newSPTable()
		cnt := tb.host("Test Counted", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPAmtV, V: kop.Const(3)})
		sc = tb.scen(t)
		sc.perm(cnt, 0)
		giant = sc.perm(v.HillGiant, 1)
		if got := spDeal(sc, 1, giant, you, 5); got != 2 {
			t.Errorf("a counted 3 off 5 = %d, want 2", got)
		}
	})
	t.Run("combat damage", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2,
			Filter2: tb.filter(kop.Filter{Colors: kop.ColR})})
		sc := tb.scen(t)
		sc.perm(host, 1)
		giant, bear := sc.perm(v.HillGiant, 0), sc.perm(v.GrizzlyBears, 0)
		before := sc.s.pl[1].life
		sc.combat(t, []Obj{giant, bear}, nil)
		if st := sc.s.g.stalled; st != 0 {
			t.Fatalf("stalled (%s)", StallName(st))
		}
		if lost := int(before - sc.s.pl[1].life); lost != 1+2 {
			t.Fatalf("the defender lost %d, want 3 (the red Giant's 3 less 2, the green Bears' 2)", lost)
		}
	})
	t.Run("two shields pose the order ask", func(t *testing.T) {
		tb := newSPTable()
		law := tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2})
		urza := tb.host("Test Armor", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 1})
		sc := tb.scen(t)
		a, b := sc.perm(law, 0), sc.perm(urza, 0)
		giant := sc.perm(v.HillGiant, 1)
		s := sc.s
		e := &s.res.e
		e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, giant, s.objs[giant].ver, s.objs[giant].card, 1
		life := s.pl[0].life
		if n := s.damageTo(sc.k, you, 5); n != 5 {
			t.Fatalf("damageTo returned %d, want 5 (parked)", n)
		}
		ro := &s.g.ro
		if ro.n != 2 || ro.spv&3 != 3 || ro.an != 5 || s.pl[0].life != life {
			t.Fatalf("ask: n %d spv %#x an %d, life moved %d; want 2 candidates, both static prevention, 5 pending, no damage yet", ro.n, ro.spv, ro.an, life-s.pl[0].life)
		}
		var d Decision
		s.replOrderOptions(&d)
		if d.Player != 0 || d.N != 2 {
			t.Fatalf("options: player %d, %d options; want the damaged player with 2", d.Player, d.N)
		}
		if ro.cand[0] != a && ro.cand[1] != a || ro.cand[0] != b && ro.cand[1] != b {
			t.Fatalf("candidates %v name neither host", ro.cand)
		}
		first := 0
		if ro.cand[first] != a {
			first = 1
		}
		s.applyReplOrder(MakeAction(AkChoose, TCode(a), 0, uint8(first))) // the Sphere's 2 first, then Armor's 1 on its own
		if got := int(life - s.pl[0].life); got != 2 {
			t.Fatalf("the player took %d, want 2 (5 less 2 less 1)", got)
		}
		if ro.n != 0 || s.g.nc.n != 0 {
			t.Fatalf("ask left behind: n %d nc %d", ro.n, s.g.nc.n)
		}
	})
	t.Run("two bare full preventions pose no ask", func(t *testing.T) {
		tb := newSPTable()
		h1 := tb.host("Test Mute", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPYou})
		h2 := tb.host("Test Mute Too", kop.Repl{Kind: kop.ReplPreventDamageFilt, C: kop.ReplSPYou})
		sc := tb.scen(t)
		sc.perm(h1, 0)
		sc.perm(h2, 0)
		giant := sc.perm(v.HillGiant, 1)
		if got := spDeal(sc, 1, giant, you, 4); got != 0 {
			t.Fatalf("damage = %d, want 0", got)
		}
		if st := sc.s.g.stalled; st != 0 || sc.s.g.ro.n != 0 {
			t.Fatalf("stalled %d, ask candidates %d; want neither", st, sc.s.g.ro.n)
		}
	})
	t.Run("a doubler beside it refuses the pair", func(t *testing.T) {
		tb := newSPTable()
		tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2})
		tb.host("Test Tyrant", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
		sc := tb.scen(t)
		if why := sc.k.gen.initStall(2); why != StallReplPlus {
			t.Fatalf("initStall = %s, want the order-ask refusal", StallName(why))
		}
	})
}

// TestStaticPreventBatchPasses: the damage passes that re-execute their op on
// a pure plus-N order ask (damageBatch with park, replPlusBatch) never take a
// static prevention's ask through that protocol (its answer would be thrown
// away): a competition, or a lone optional line, is a loud stall with no ask
// posed, and a lone shield still applies.
func TestStaticPreventBatchPasses(t *testing.T) {
	const you = TCode(PlayerTarget0)
	// resolving: the Giant (seat 1) is the resolving entry's source
	resolving := func(sc *scen, src Obj) {
		s := sc.s
		e := &s.res.e
		e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, src, s.objs[src].ver, s.objs[src].card, 1
	}
	for _, tc := range []struct {
		name  string
		repls []kop.Repl
	}{
		{"two shields", []kop.Repl{{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2}, {Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 1}}},
		{"a lone optional shield", []kop.Repl{{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPOptional, A: 2}}},
	} {
		t.Run("damage map "+tc.name, func(t *testing.T) {
			tb := newSPTable()
			var hosts []CardID
			for i, r := range tc.repls {
				hosts = append(hosts, tb.host("Test Shield "+string(rune('A'+i)), r))
			}
			sc := tb.scen(t)
			for _, h := range hosts {
				sc.perm(h, 0)
			}
			giant := sc.perm(v.HillGiant, 1)
			resolving(sc, giant)
			s := sc.s
			if s.g.stalled != 0 {
				t.Fatalf("precondition: stalled at start (%s)", StallName(s.g.stalled))
			}
			s.damageBatch(sc.k, []TCode{you}, []int{5}, true)
			if s.g.plusBatchAsked || s.g.ro.n != 0 {
				t.Fatalf("posed through the plus-N re-execute protocol: plusBatchAsked %v, ask candidates %d", s.g.plusBatchAsked, s.g.ro.n)
			}
			if st := s.g.stalled; st != StallReplPlus {
				t.Fatalf("stalled %s, want the loud order-ask stall", StallName(st))
			}
		})
		t.Run("hit batch "+tc.name, func(t *testing.T) {
			tb := newSPTable()
			crt := tb.filter(kop.Filter{Types: kop.TCreature})
			var hosts []CardID
			for i, r := range tc.repls {
				r.Filter = crt
				hosts = append(hosts, tb.host("Test Shield "+string(rune('A'+i)), r))
			}
			sc := tb.scen(t)
			for _, h := range hosts {
				sc.perm(h, 0)
			}
			giant, bear := sc.perm(v.HillGiant, 1), sc.perm(v.GrizzlyBears, 0)
			s := sc.s
			hs := []dmgHit{{src: sc.slot(giant), dst: sc.slot(bear), n: 3, ctrl: 1}}
			if s.replPlusBatch(sc.k, hs) || s.g.plusBatchAsked || s.g.ro.n != 0 {
				t.Fatalf("replPlusBatch parked: plusBatchAsked %v, ask candidates %d", s.g.plusBatchAsked, s.g.ro.n)
			}
			s.dealHits(sc.k, hs)
			if st := s.g.stalled; st != StallReplPlus {
				t.Fatalf("stalled %s, want the loud order-ask stall", StallName(st))
			}
		})
	}
	t.Run("a lone shield applies", func(t *testing.T) {
		tb := newSPTable()
		h := tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2})
		sc := tb.scen(t)
		sc.perm(h, 0)
		giant := sc.perm(v.HillGiant, 1)
		resolving(sc, giant)
		s := sc.s
		life := s.pl[0].life
		s.damageBatch(sc.k, []TCode{you}, []int{5}, true)
		if st := s.g.stalled; st != 0 || s.g.ro.n != 0 {
			t.Fatalf("stalled %d, ask candidates %d; want neither", st, s.g.ro.n)
		}
		if got := int(life - s.pl[0].life); got != 3 {
			t.Fatalf("the player took %d, want 3 (5 less 2)", got)
		}
	})
}

// TestStaticPreventOptionalAsk: a lone Optional$ shield (Battletide Alchemist)
// is asked of its controller, not the damaged player; the pick prevents its
// amount and the skip option (mode ro.n) deals all of it.
func TestStaticPreventOptionalAsk(t *testing.T) {
	const you = TCode(PlayerTarget0)
	for _, tc := range []struct {
		name string
		skip bool
		want int
	}{{"pick", false, 3}, {"skip", true, 5}} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newSPTable()
			h := tb.host("Test Alchemist", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPOptional, A: 2})
			sc := tb.scen(t)
			host := sc.perm(h, 1)
			giant := sc.perm(v.HillGiant, 1)
			s := sc.s
			e := &s.res.e
			e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, giant, s.objs[giant].ver, s.objs[giant].card, 1
			life := s.pl[0].life
			if n := s.damageTo(sc.k, you, 5); n != 5 {
				t.Fatalf("damageTo returned %d, want 5 (parked)", n)
			}
			ro := &s.g.ro
			if ro.n != 1 || ro.spv&spOptionalAsk == 0 || ro.cand[0] != host || s.pl[0].life != life {
				t.Fatalf("ask: n %d spv %#x cand %v, life moved %d; want the lone optional host asked, no damage yet", ro.n, ro.spv, ro.cand[0], life-s.pl[0].life)
			}
			var d Decision
			s.replOrderOptions(&d)
			if d.Player != 1 || d.N != 2 {
				t.Fatalf("options: player %d, %d options; want the host's controller (1) with the line and its skip", d.Player, d.N)
			}
			var skip Action
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Mode() == ro.n {
					skip = d.Options[i]
				}
			}
			if skip == 0 {
				t.Fatalf("no skip option with mode %d in %v", ro.n, d.Options[:d.N])
			}
			pick := MakeAction(AkChoose, TCode(host), 0, 0)
			if tc.skip {
				pick = skip
			}
			s.applyReplOrder(pick)
			if got := int(life - s.pl[0].life); got != tc.want {
				t.Fatalf("the player took %d, want %d", got, tc.want)
			}
			if ro.n != 0 || s.g.nc.n != 0 {
				t.Fatalf("ask left behind: n %d nc %d", ro.n, s.g.nc.n)
			}
		})
	}
}
