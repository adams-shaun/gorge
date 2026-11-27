package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSacrificedThisTurnFilteredRead: an ExSacrificedThisTurn with X.F (the
// widened shape; Dargo, the Shipwrecker and Evendo Brushrazer) walks the
// turn's zone-entry ledger. The entry is marked only by a real sacrifice
// (noteSac's pfSacNext consumed by removePerm), so a destroy does not count;
// the sacrificer is the controller it left under, not the owner; a token
// matches a Creature filter but not !token; and a copy token is skipped (CR
// 707.10h, gorge matchesZoneSpecCtx rejects an IsCopy object off the
// battlefield).
func TestSacrificedThisTurnFilteredRead(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature},
		kop.CardDef{Name: "Test Bauble", Types: kop.TArtifact})
	bear := CardID(len(v.Table.Cards))
	bauble := bear + 1
	f := append([]kop.Filter(nil), tbl.Filters...)
	f = append(f, kop.Filter{Types: kop.TCreature}, kop.Filter{Flags: kop.FNontoken})
	creature, nontoken := kop.FilterID(len(tbl.Filters)), kop.FilterID(len(tbl.Filters)+1)
	tbl.Filters = f
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.s.g.on = true // a token's departure records its entry only in a played game
	sc.s.enteredRing()
	read := func(ctrl uint8, fid kop.FilterID) int {
		return sc.s.sacrificedThisTurn(sc.k, &kop.Expr{Kind: kop.ExSacrificedThisTurn, F: fid}, evalCtx{ctrl: ctrl})
	}
	sac := func(c CardID, owner uint8) {
		o := sc.perm(c, owner)
		sl := sc.slot(o)
		sc.s.fireSac(sc.k, sl)
		sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	}
	if got := read(0, creature); got != 0 {
		t.Fatalf("start: creature count %d, want 0", got)
	}

	sac(bear, 0)
	if got := read(0, creature); got != 1 {
		t.Fatalf("one creature sacrifice: creature count %d, want 1", got)
	}
	if got := read(0, nontoken); got != 1 {
		t.Fatalf("one creature sacrifice: nontoken count %d, want 1", got)
	}

	// a sacrifice controlled by seat 1 counts for seat 1, not for seat 0
	sac(bauble, 1)
	if got := read(0, creature); got != 1 {
		t.Fatalf("seat 1's artifact: seat 0's creature count %d, want 1", got)
	}
	if got := read(1, nontoken); got != 1 {
		t.Fatalf("seat 1's artifact: seat 1's nontoken count %d, want 1", got)
	}

	// a token creature matches Creature but not !token
	tok := sc.perm(bear, 0)
	sc.s.objs[tok].of |= ofToken
	sl := sc.slot(tok)
	sc.s.fireSac(sc.k, sl)
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if got := read(0, creature); got != 2 {
		t.Fatalf("token sacrifice: creature count %d, want 2", got)
	}
	if got := read(0, nontoken); got != 1 {
		t.Fatalf("token sacrifice: nontoken count %d, want 1 (the token is skipped)", got)
	}

	// a token copy is skipped for every filter
	cp := sc.perm(bauble, 0)
	sc.s.objs[cp].of |= ofToken | ofCopy
	sl = sc.slot(cp)
	sc.s.fireSac(sc.k, sl)
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if got := read(0, creature); got != 2 {
		t.Fatalf("token copy sacrifice: creature count %d, want 2", got)
	}
	if got := read(0, nontoken); got != 1 {
		t.Fatalf("token copy sacrifice: nontoken count %d, want 1", got)
	}

	// a destroy is no sacrifice: the flag is set only by noteSac
	b := sc.perm(bear, 0)
	sc.s.leave(sc.k, sc.slot(b), kop.ZoneGraveyard, false)
	if got := read(0, creature); got != 2 {
		t.Fatalf("destroy: creature count %d, want 2", got)
	}

	// the turn reset clears the ledger
	sc.s.beginTurn(1)
	if got := read(0, creature); got != 0 {
		t.Fatalf("next turn: creature count %d, want 0", got)
	}
}
