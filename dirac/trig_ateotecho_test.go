package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// ateotEchoScen is a generated table with a Panharmonicon static (triggered
// abilities of a creature you control trigger an additional time, Twinflame
// Travelers' shape) beside a creature token carrying a copy's own AtEOTTrig$
// sacrifice record (creator == the token, as the copy mint writes it) and a
// creature carrying an AtEOT$ Exile rider sourced on another object.
func ateotEchoScen(t *testing.T) (sc *scen, own, rider Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	twin := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Twinflame", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatPanharmonicon, Filter: yours}}}})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc = newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyPanharm {
		t.Fatal("precondition: the table does not arm the Panharmonicon echo (gen.anyPanharm)")
	}
	tw := sc.perm(twin, 0)
	own = sc.perm(bear, 0)
	rider = sc.perm(bear, 0)
	s := sc.s
	s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: own, ver: s.objs[own].ver, creator: own}
	s.dly[1] = delayed{act: uint8(kop.OpExile), ctrl: 0, obj: rider, ver: s.objs[rider].ver, creator: tw}
	s.ndly = 2
	if !s.dly[0].ownTrig() || s.dly[1].ownTrig() {
		t.Fatal("precondition: ownTrig does not tell the token's own trigger from the AtEOT$ rider")
	}
	if !sc.onBF(own) || !sc.onBF(rider) {
		t.Fatal("precondition: the tokens are not on the battlefield")
	}
	if s.panharmEchoes(sc.k, trigListener{obj: own, sl: sc.slot(own), ctrl: 0, card: s.cardAt(sc.slot(own))}, kop.TrigPhase, NoObj, 0) != 1 {
		t.Fatal("precondition: the static does not match the token")
	}
	return sc, own, rider
}

// pendFor counts the pending triggers whose referent (trigObj) is o.
func pendFor(s *State, o Obj) int {
	n := 0
	for i := uint8(0); i < s.npend; i++ {
		if s.pend[i].ab == abDelayed && s.pend[i].trigObj == o {
			n++
		}
	}
	return n
}

// TestAtEOTTrigOwnSacrificeEchoed: a copy token's own "at the beginning of
// the end step, sacrifice this token" is the token's triggered ability (CR
// 707.2), so a matching Panharmonicon static makes it trigger an additional
// time (CR 603.2d; gorge checkGrantedAtEOTTriggers -> queueTrigger). An
// AtEOT$ rider is its creator's delayed trigger and is not echoed.
func TestAtEOTTrigOwnSacrificeEchoed(t *testing.T) {
	sc, own, rider := ateotEchoScen(t)
	sc.s.fireDelayed(sc.k)
	if got := pendFor(sc.s, own); got != 2 {
		t.Fatalf("the token's own end-step sacrifice queued %d times, want 2 (once plus one echo)", got)
	}
	if got := pendFor(sc.s, rider); got != 1 {
		t.Fatalf("the AtEOT$ rider queued %d times, want 1 (never echoed)", got)
	}
}

// TestAtEOTTrigOwnSacrificeNoEchoMutation: with the echo switched off the
// token's own trigger is queued once (the test above can tell the two apart).
func TestAtEOTTrigOwnSacrificeNoEchoMutation(t *testing.T) {
	sc, own, _ := ateotEchoScen(t)
	DebugPanharmNoEcho = true
	defer func() { DebugPanharmNoEcho = false }()
	sc.s.fireDelayed(sc.k)
	if got := pendFor(sc.s, own); got != 1 {
		t.Fatalf("with the echo off the token's own end-step sacrifice queued %d times, want 1", got)
	}
}
