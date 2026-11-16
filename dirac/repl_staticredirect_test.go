package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The printed static damage redirects (kop.ReplDamageRedirect,
// repl_staticredirect.go). The rbPlusDmg opener the card scan sets opens the
// replDamage sites, so these scen tables exercise the arms directly.

// rdrDeal is the noncombat damage n a source (a permanent at sl, or a slotless
// spell card) dealt to code after the replacements (replDamage, as the damage
// sites read it).
func rdrDeal(sc *scen, srcCtrl uint8, sl Slot, card CardID, code TCode, n int) int {
	return sc.s.replDamage(sc.k, srcCtrl, sl, card, code, false, n)
}

// rdrSpell adds a spell card to the table and returns its id (a slotless
// damage source's card).
func rdrSpell(tb *spTable, name string, ty kop.TypeMask, col kop.ColorMask) CardID {
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: name, Types: ty, Colors: col})
	return CardID(len(tb.tbl.Cards) - 1)
}

// rdrRes points the resolving entry at the spell (its card and object, kind
// spell), so a slotless source's keywords are read (redirKw).
func rdrRes(sc *scen, card CardID, o Obj) {
	sc.s.res.active = true
	sc.s.res.e.card = card
	sc.s.res.e.obj = o
	sc.s.res.e.kind = seSpell
}

// TestStaticRedirectToHost: Pariah's damage to you is dealt to the enchanted
// creature instead (the rewrite's recipient is the aura's attached object);
// Veteran Bodyguard redirects only while it is untapped and only an unblocked
// attacking creature's damage.
func TestStaticRedirectToHost(t *testing.T) {
	const them = TCode(PlayerTarget0 + 1)
	t.Run("pariah", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Pariah", kop.Repl{Kind: kop.ReplDamageRedirect, C: kop.RdrToYou | kop.RdrAsAttached})
		sc := tb.scen(t)
		aura := sc.perm(host, 1)
		bear := sc.perm(v.GrizzlyBears, 1)
		sc.s.perms[sc.slot(aura)].attached = sc.slot(bear)
		src := sc.perm(v.GrizzlyBears, 0)
		if got := rdrDeal(sc, 0, sc.slot(src), v.GrizzlyBears, them, 3); got != 0 {
			t.Fatalf("damage to seat 1 = %d, want 0 (redirected)", got)
		}
		if got := sc.s.perms[sc.slot(bear)].damage; got != 3 {
			t.Errorf("the enchanted creature took %d damage, want 3", got)
		}
		if got := sc.s.pl[1].life; got != 20 {
			t.Errorf("seat 1's life = %d, want 20 (the damage was dealt to the creature)", got)
		}
		// another player's damage is not the redirect's event
		if got := rdrDeal(sc, 0, NoSlot, 0, TCode(PlayerTarget0), 2); got != 2 {
			t.Errorf("damage to seat 0 = %d, want 2 (Pariah is seat 1's)", got)
		}
	})
	t.Run("veteran gates", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Veteran Bodyguard", kop.Repl{Kind: kop.ReplDamageRedirect,
			C:       kop.RdrToYou | kop.RdrAsSelf | kop.RdrGateUntapped | kop.RdrSrcUnblocked,
			Filter2: tb.filter(kop.Filter{Types: kop.TCreature})})
		sc := tb.scen(t)
		guard := sc.perm(host, 1)
		atk := sc.perm(v.GrizzlyBears, 0)
		asl := sc.slot(atk)
		sc.s.perms[asl].flags |= pfAttacking
		sc.s.perms[asl].atk = PlayerTarget0 + 1
		deal := func() *dmgRec {
			var r dmgRec
			r.dealt[asl], r.dealtPl[asl], r.toPl[1] = 3, 3, 3
			r.note(asl, NoSlot, 3)
			return &r
		}
		// untapped guard, unblocked attacker: the damage is dealt to the guard
		r := deal()
		sc.s.replCombatGen(sc.k, r)
		if got := r.ev[0].n; got != 0 {
			t.Errorf("an unblocked attacker's event left %d, want 0 (redirected)", got)
		}
		if got := r.toPerm[sc.slot(guard)]; got != 3 {
			t.Errorf("the guard's combat damage = %d, want 3", got)
		}
		// a tapped guard redirects nothing: the damage stands
		sc.s.perms[sc.slot(guard)].flags |= pfTapped
		r = deal()
		sc.s.replCombatGen(sc.k, r)
		if got := r.ev[0].n; got != 3 || r.toPerm[sc.slot(guard)] != 0 {
			t.Errorf("a tapped guard's event = %d (guard %d), want 3 / 0", got, r.toPerm[sc.slot(guard)])
		}
		sc.s.perms[sc.slot(guard)].flags &^= pfTapped
		// a blocked attacker's damage stands (unblocked is live at the
		// assignments; a removed blocker's tombstone keeps it blocked)
		sc.s.perms[asl].flags |= pfBlocked
		r = deal()
		sc.s.replCombatGen(sc.k, r)
		if got := r.ev[0].n; got != 3 || r.toPerm[sc.slot(guard)] != 0 {
			t.Errorf("a blocked attacker's event = %d (guard %d), want 3 / 0", got, r.toPerm[sc.slot(guard)])
		}
		if sc.s.g.stalled != 0 {
			t.Errorf("stalled %d, want no stall", sc.s.g.stalled)
		}
		// a creature source outside combat is never unblocked: the damage stands
		sc.s.perms[asl].flags &^= pfAttacking | pfBlocked
		if got := rdrDeal(sc, 0, asl, v.GrizzlyBears, them, 2); got != 2 {
			t.Errorf("noncombat damage from a creature source = %d, want 2", got)
		}
	})
}

// TestStaticRedirectFilters: Palisade Giant's Other excludes the giant
// itself; Treacherous Link's damage to the enchanted creature is dealt to its
// controller; Harsh Judgment's chosen-colour spell source, and its no-op when
// the source's controller is the recipient.
func TestStaticRedirectFilters(t *testing.T) {
	const you, them = TCode(PlayerTarget0), TCode(PlayerTarget0 + 1)
	t.Run("palisade other", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Palisade Giant", kop.Repl{Kind: kop.ReplDamageRedirect,
			C: kop.RdrToYou | kop.RdrToYouPerms | kop.RdrAsSelf})
		sc := tb.scen(t)
		giant := sc.perm(host, 1)
		bear := sc.perm(v.GrizzlyBears, 1)
		src := sc.perm(v.GrizzlyBears, 0)
		sl, card := sc.slot(src), CardID(v.GrizzlyBears)
		if got := rdrDeal(sc, 0, sl, card, them, 3); got != 0 {
			t.Fatalf("damage to seat 1 = %d, want 0 (redirected)", got)
		}
		if got := sc.s.perms[sc.slot(giant)].damage; got != 3 {
			t.Errorf("the giant took %d damage, want 3", got)
		}
		sc.s.perms[sc.slot(giant)].damage = 0
		if got := rdrDeal(sc, 0, sl, card, TCode(bear), 3); got != 0 {
			t.Fatalf("damage to another permanent of seat 1 = %d, want 0 (redirected)", got)
		}
		if got := sc.s.perms[sc.slot(giant)].damage; got != 3 {
			t.Errorf("the giant took %d damage, want 3", got)
		}
		// the giant's own damage is not the redirect's event (Other)
		sc.s.perms[sc.slot(giant)].damage = 0
		if got := rdrDeal(sc, 0, sl, card, TCode(giant), 2); got != 2 {
			t.Fatalf("damage to the giant = %d, want 2 (its own damage is not the event)", got)
		}
	})
	t.Run("treacherous link", func(t *testing.T) {
		tb := newSPTable()
		host := tb.host("Treacherous Link", kop.Repl{Kind: kop.ReplDamageRedirect,
			C: kop.RdrToEnchanted | kop.RdrAsTgtCtrl})
		bolt := rdrSpell(tb, "Test Bolt", kop.TInstant, kop.ColR)
		sc := tb.scen(t)
		aura := sc.perm(host, 1)
		bear := sc.perm(v.GrizzlyBears, 0)
		sc.s.perms[sc.slot(aura)].attached = sc.slot(bear)
		spell := sc.obj(bolt, 1)
		rdrRes(sc, bolt, spell)
		if got := rdrDeal(sc, 1, NoSlot, bolt, TCode(bear), 3); got != 0 {
			t.Fatalf("damage to the enchanted creature = %d, want 0 (redirected)", got)
		}
		if got := sc.s.perms[sc.slot(bear)].damage; got != 0 {
			t.Errorf("the creature took %d damage, want 0", got)
		}
		if got := sc.s.pl[0].life; got != 17 {
			t.Errorf("seat 0's life = %d, want 17 (the creature's controller takes the damage)", got)
		}
		// damage to another creature is not the redirect's event
		other := sc.perm(v.GrizzlyBears, 0)
		if got := rdrDeal(sc, 1, NoSlot, bolt, TCode(other), 2); got != 2 {
			t.Errorf("damage to another creature = %d, want 2", got)
		}
	})
	t.Run("harsh judgment", func(t *testing.T) {
		tb := newSPTable()
		f1 := tb.filter(kop.Filter{Types: kop.TInstant, Flags2: kop.F2ChosenColor})
		f2 := tb.filter(kop.Filter{Types: kop.TSorcery, Flags2: kop.F2ChosenColor})
		tb.tbl.Filters[f1].Or = f2
		host := tb.host("Harsh Judgment", kop.Repl{Kind: kop.ReplDamageRedirect,
			C: kop.RdrToYou | kop.RdrAsSrcCtrl | kop.RdrSrcChosen, Filter2: f1})
		red := rdrSpell(tb, "Test Red Bolt", kop.TInstant, kop.ColR)
		green := rdrSpell(tb, "Test Green Bolt", kop.TInstant, kop.ColG)
		sc := tb.scen(t)
		hj := sc.perm(host, 1)
		sc.s.perms[sc.slot(hj)].chosen = objWord(3) // red (ColR is WUBRG's fourth bit)
		redObj := sc.obj(red, 0)
		greenObj := sc.obj(green, 0)
		setRes := func(card CardID, o Obj) { rdrRes(sc, card, o) }
		// a red spell's damage to seat 1 is dealt to its controller
		setRes(red, redObj)
		if got := rdrDeal(sc, 0, NoSlot, red, them, 3); got != 0 {
			t.Fatalf("damage from a red spell = %d, want 0 (redirected)", got)
		}
		if got := sc.s.pl[0].life; got != 17 {
			t.Errorf("seat 0's life = %d, want 17 (the spell's controller takes it)", got)
		}
		if got := sc.s.pl[1].life; got != 20 {
			t.Errorf("seat 1's life = %d, want 20", got)
		}
		// a green spell's damage stands
		setRes(green, greenObj)
		if got := rdrDeal(sc, 0, NoSlot, green, them, 3); got != 3 {
			t.Errorf("damage from a green spell = %d, want 3 (not the chosen colour)", got)
		}
		// a no-op rewrite: your own red spell's damage to you
		setRes(red, redObj)
		if got := rdrDeal(sc, 1, NoSlot, red, them, 3); got != 3 {
			t.Errorf("your own red spell's damage to you = %d, want 3 (the rewrite is a no-op)", got)
		}
		if sc.s.g.stalled != 0 {
			t.Errorf("stalled %d, want no stall", sc.s.g.stalled)
		}
		_ = you
	})
}

// TestStaticRedirectStall: a static redirect matching an event beside another
// damage replacement (a doubler) is gorge's CR 616.1 order ask: the kernel
// stalls StallRedirOrder rather than answer.
func TestStaticRedirectStall(t *testing.T) {
	const them = TCode(PlayerTarget0 + 1)
	tb := newSPTable()
	host := tb.host("Pariah", kop.Repl{Kind: kop.ReplDamageRedirect, C: kop.RdrToYou | kop.RdrAsAttached})
	twin := tb.host("Test Twinflame", kop.Repl{Kind: kop.ReplDoubleDamageToOpps})
	sc := tb.scen(t)
	aura := sc.perm(host, 1)
	bear := sc.perm(v.GrizzlyBears, 1)
	sc.s.perms[sc.slot(aura)].attached = sc.slot(bear)
	src := sc.perm(v.GrizzlyBears, 0)
	sc.perm(twin, 0)
	if got := rdrDeal(sc, 0, sc.slot(src), v.GrizzlyBears, them, 3); got != 0 || sc.s.g.stalled != StallRedirOrder {
		t.Errorf("damage beside a doubler = %d stalled %d, want 0 / %d", got, sc.s.g.stalled, StallRedirOrder)
	}
}
