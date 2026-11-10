package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// untapLockScen is a two-player scenario whose table carries a spell with the
// two untap-lock programs (OpSkipUntap C 2 and C 3 over RefSelf).
func untapLockScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 18}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lc := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Savannah Lions" {
			lc = CardID(i)
		}
	}
	step := kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefSelf, C: kop.SkipUntapLockStep}
	anyU := kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefSelf, C: kop.SkipUntapLockAny}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Lock", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{step, anyU}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyUntapBlock || !sc.k.gen.anySkipUntap {
		t.Fatalf("prep flags block %v skip %v, want all true", sc.k.gen.anyUntapBlock, sc.k.gen.anySkipUntap)
	}
	return sc, lc
}

// lockOn makes the lock of kind c on target, held by src under player ctrl
// (the target rides the resolving ability's triggering object).
func (sc *scen) lockOn(target, src Obj, ctrl uint8, c int16) {
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.trigObj = ctrl, src, target
	sc.s.untapLockAdd(sc.k, &kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefTriggerObj, C: c})
}

// TestUntapLock: a Duration$ AsLongAsControl untap lock (Ty Lee, Chi Blocker:
// the untap step; Spider-Woman, Secret Agent: every untap) holds the target
// tapped while its source is on the battlefield under the player who made it;
// the source changing controller ends it for good, the source leaving makes
// it not live, the target leaving drops it, and a fifth record stalls.
func TestUntapLock(t *testing.T) {
	sc, lc := untapLockScen(t)
	s := sc.s
	src, tgt := sc.perm(lc, 0), sc.perm(lc, 1)
	slot := sc.slot(tgt)
	untapStep := func() {
		s.active, s.step = 1, kop.StepUntap
		s.beginStep(sc.k)
	}
	tap := func() { s.perms[sc.slot(tgt)].flags |= pfTapped }

	sc.lockOn(tgt, src, 0, kop.SkipUntapLockStep)
	if s.g.nul != 1 {
		t.Fatalf("records %d, want 1", s.g.nul)
	}
	tap()
	untapStep()
	if !sc.tapped(tgt) {
		t.Fatal("step lock: the locked creature untapped while its source is under the lock's player")
	}
	if s.untapBlocked(sc.k, slot) {
		t.Fatal("step lock: an effect's untap is blocked, but the step form holds only in the untap step")
	}

	// the source changes controller: the lock ends for good
	s.ctrlMove(sc.slot(src), 1)
	if s.g.nul != 0 {
		t.Fatalf("records %d after the source changed controller, want 0", s.g.nul)
	}
	s.ctrlMove(sc.slot(src), 0)
	untapStep()
	if sc.tapped(tgt) {
		t.Fatal("the lock came back with control (CR 611.2b: gone for good)")
	}

	// the any-event form blocks an effect's untap and the untap step
	tap()
	sc.lockOn(tgt, src, 0, kop.SkipUntapLockAny)
	if !s.untapBlocked(sc.k, slot) {
		t.Fatal("any lock: an effect's untap is not blocked")
	}
	untapStep()
	if !sc.tapped(tgt) {
		t.Fatal("any lock: the untap step untapped the creature")
	}
	// the source leaves: not live, so the creature untaps and nothing blocks
	s.leave(sc.k, sc.slot(src), kop.ZoneGraveyard, false)
	if s.untapBlocked(sc.k, slot) {
		t.Fatal("any lock: still blocking with its source in the graveyard")
	}
	untapStep()
	if sc.tapped(tgt) {
		t.Fatal("the untap step left the creature tapped with its source gone")
	}
	// the target leaves: its record goes
	if s.g.nul != 1 {
		t.Fatalf("records %d with the source gone and the target in play, want 1 (kept: a card source can return)", s.g.nul)
	}
	s.leave(sc.k, slot, kop.ZoneGraveyard, false)
	if s.g.nul != 0 {
		t.Fatalf("records %d after the target left, want 0", s.g.nul)
	}
}

// TestUntapLockCapacity: four records at once, a fifth stalls
// StallUntapLock, and the same record twice is one.
func TestUntapLockCapacity(t *testing.T) {
	sc, lc := untapLockScen(t)
	s := sc.s
	src := sc.perm(lc, 0)
	var tg [5]Obj
	for i := range tg {
		tg[i] = sc.perm(lc, 1)
	}
	for i := 0; i < 4; i++ {
		sc.lockOn(tg[i], src, 0, kop.SkipUntapLockStep)
	}
	sc.lockOn(tg[3], src, 0, kop.SkipUntapLockStep)
	if s.g.nul != 4 || s.g.stalled != 0 {
		t.Fatalf("records %d stalled %d, want 4 and 0 (a repeated record is one)", s.g.nul, s.g.stalled)
	}
	sc.lockOn(tg[4], src, 0, kop.SkipUntapLockStep)
	if s.g.nul != 4 || s.g.stalled != StallUntapLock {
		t.Fatalf("records %d stalled %d, want 4 and %d", s.g.nul, s.g.stalled, StallUntapLock)
	}
}
