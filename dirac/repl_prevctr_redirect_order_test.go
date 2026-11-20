package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestCombatRedirectWithCounterHostInTable: a card in the table that carries a
// ReplPreventDamageCounters replacement must not reorder the combat pass
// unless a live host of it receives an event of that pass. The pass's walk
// order (replOrderOrder) covers only the events that existed before the loop,
// while a combat redirect appends a new event mid-pass: under a reordered walk
// that appended event is indexed through the stale order (ord[j] = 0), so event
// 0 is reprocessed and the redirected damage is never dealt to its new
// recipient's protection.
//
// The Phantom stays in the library (only its table-wide bit is set) and an Ext
// DmgRedirect record redirects the damage to seat 1 to a creature with
// protection from the red source, so the redirected event must be prevented.
func TestCombatRedirectWithCounterHostInTable(t *testing.T) {
	tb := newSPTable()
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Red Ward", Types: kop.TCreature, Power: 2, Toughness: 2, ProtColors: kop.ColR})
	ward := CardID(len(tb.tbl.Cards) - 1)
	tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Phantom", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplPreventDamageCounters, C: kop.ReplPrevSelfRemoveOne}}}})
	// the source deals red damage, which the ward is protected from
	tb.tbl.Cards[v.GrizzlyBears].Colors = kop.ColR

	sc := tb.scen(t)
	wardObj := sc.perm(ward, 1)
	src := sc.perm(v.GrizzlyBears, 0)
	ws, ss := sc.slot(wardObj), sc.slot(src)
	// the mbc redirect (Gideon's Sacrifice) lands seat 1's damage on the ward
	sc.k.gen.anyRedir = true
	sc.s.g.redir[0] = redirRec{tgt: Obj(wardObj), ctl: 1, kind: kop.DmgRedirectScope}
	sc.s.perms[ss].atk = PlayerTarget0 + 1 // the player the source's combat event hits

	// preconditions: the redirect and protection arms are open, the table
	// carries the counter host's replacement, and no live counter host exists
	// (only the library card has it), so the table-wide bit alone arms byOrd.
	if !sc.k.gen.anyRedir || sc.k.gen.anyRepl&rbProtCol == 0 {
		t.Fatalf("precondition: redirect arm %v, protection bit %v", sc.k.gen.anyRedir, sc.k.gen.anyRepl&rbProtCol != 0)
	}
	if sc.k.gen.anyRepl&rbPrevCtrSelf == 0 {
		t.Fatal("precondition: the table does not carry a ReplPreventDamageCounters card")
	}
	if sc.s.selfDamageHost(sc.k, ws, rbPrevCtrSelf) {
		t.Fatal("precondition: the redirect target is a live counter host (the redirect would stall)")
	}
	if !sc.s.protColFrom(sc.k, ws, kop.ColR) {
		t.Fatal("precondition: the redirect target is not protected from the source's colour")
	}
	if _, h := sc.s.redirFind(PlayerTarget0+1, 1); h != 1 {
		t.Fatalf("precondition: the redirect record matches %d times, want 1", h)
	}

	var r dmgRec
	r.dealt[ss], r.dealtPl[ss], r.toPl[1] = 3, 3, 3
	r.note(ss, NoSlot, 3)
	var ord [MaxPerm]uint8
	if r.nev != 1 || sc.s.replOrderOrder(&r, &ord) != 1 {
		t.Fatalf("precondition: pass has %d events, order count %d", r.nev, sc.s.replOrderOrder(&r, &ord))
	}

	sc.s.replCombatGen(sc.k, &r)
	if r.nev != 2 {
		t.Fatalf("the redirect appended %d events, want 2 (the original and the redirected); stalled=%s ev=%v", r.nev, StallName(sc.s.g.stalled), r.ev[:r.nev])
	}
	if r.toPl[1] != 0 || r.ev[0].n != 0 {
		t.Errorf("the original event is %d (seat 1 toPl %d), want 0 (redirected)", r.ev[0].n, r.toPl[1])
	}
	if r.toPerm[ws] != 0 || r.ev[1].n != 0 {
		t.Errorf("the redirected event is %d with %d damage on the protected target, want 0 / 0", r.ev[1].n, r.toPerm[ws])
	}
	if sc.s.g.stalled != 0 {
		t.Errorf("stalled (%s), want no stall", StallName(sc.s.g.stalled))
	}
}
