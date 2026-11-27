package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTrigPlayerCtrlOffBattlefield: the card matcher's PredTrigPlayerCtrl arm
// (matchCardOneAt) reads a card off the battlefield by its OWNER: a card there
// is controlled by its owner, so the trigger's damaged player owns Deluxe
// Dragster's graveyard target. The arm must answer the trigger player's card
// and not another player's, and a spell entry must not read a trigger player.
func TestTrigPlayerCtrlOffBattlefield(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	fl := kop.Filter{Pred: kop.PredTrigPlayerCtrl, Types: kop.TInstant, Zone: kop.ZoneGraveyard}
	own, other := sc.obj(v.Shock, 0), sc.obj(v.Shock, 1)
	sc.s.place(own, kop.ZoneGraveyard, false)
	sc.s.place(other, kop.ZoneGraveyard, false)
	if sc.s.objs[own].zone != kop.ZoneGraveyard || sc.s.objs[other].zone != kop.ZoneGraveyard {
		t.Fatal("the graveyard precondition did not hold")
	}
	match := func(o Obj, owner, ctrl uint8) bool {
		return sc.s.matchCardOneAt(sc.k, &fl, v.Shock, owner, ctrl, o, nil, false)
	}

	if match(own, 0, 0) {
		t.Error("a card matched PredTrigPlayerCtrl with no trigger entry")
	}
	sc.s.ctl = ctlBuild
	sc.s.bld().e = stackEntry{kind: seTrigger, trigPl: 0}
	if !match(own, 0, 0) {
		t.Error("the trigger player's own graveyard card did not match")
	}
	if match(other, 1, 0) {
		t.Error("another player's graveyard card matched the trigger player")
	}
	sc.s.bld().e = stackEntry{kind: seSpell, trigPl: 0}
	if match(own, 0, 0) {
		t.Error("a spell entry read a trigger player")
	}
	sc.s.bld().e = stackEntry{kind: seTrigger, trigPl: 1}
	if match(own, 0, 0) || !match(other, 1, 0) {
		t.Error("the trigger player's other-seat card did not reverse the owner test")
	}
}

// TestOppProtectFilterWord: FwOppProtect (filter_oppprotect.go) reads the
// battlefield battle's recorded protector against the reading controller. An
// opponent protector matches; no protector, a non-battle permanent, the
// evaluator itself, a departed protector and an off-battlefield card fail
// closed.
func TestOppProtectFilterWord(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	lit := kop.FLit{W: kop.FwOppProtect}
	battle := sc.perm(front, 0)
	bear := sc.perm(v.GrizzlyBears, 0) // a record on a non-battle must not match
	check := func(name string, sl Slot, ctrl uint8, want bool) {
		t.Helper()
		c := sc.s.permFctx(sc.k, sl, ctrl, NoSlot, layerFull)
		if got := sc.s.fwRun(&c, &lit); got != want {
			t.Errorf("%s (ctrl %d): got %v, want %v", name, ctrl, got, want)
		}
	}

	bsl := sc.slot(battle)
	// the battle entered under seat 0 with seat 1 (the sole opponent) as its
	// protector (CR 310.10; face_battle.go's enter).
	check("opponent protector", bsl, 0, true)
	check("protector is the evaluator", bsl, 1, false)

	sc.s.perms[bsl].chosen = protChosen | 0 // the evaluator protects it
	check("the evaluator as protector", bsl, 0, false)

	sc.s.perms[bsl].chosen = protChosen | 1
	sc.s.perms[sc.slot(bear)].chosen = protChosen | 1
	check("non-battle with a record", sc.slot(bear), 0, false)

	sc.s.pl[1].lost = true // the protector left the game
	check("departed protector", bsl, 0, false)
	sc.s.pl[1].lost = false

	sc.s.perms[bsl].chosen = 255 // no recorded protector
	check("no protector", bsl, 0, false)

	sc.s.perms[bsl].chosen = protChosen | 1
	c := sc.s.permFctx(sc.k, bsl, 0, NoSlot, layerFull)
	c.zone, c.sl = kop.ZoneGraveyard, NoSlot // the same card read off the battlefield
	if sc.s.fwRun(&c, &lit) {
		t.Error("a card off the battlefield matched FwOppProtect")
	}
}
