package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMonstrosityTriggerAmount: OpMonstrosity reads TriggerCount$Amount
// (VTriggerAmount) for the counter count and the BecomeMonstrous event,
// marks only once, and no-ops for a non-positive amount. No build-supported
// monstrosity card binds it (Vitality Hunter is refused at emit for its
// dynamic target count; Hydra Broodmaster for its token/TriggerCount body),
// so this is a hand-built table in the pattern of TestAttackPlayerOnce. The
// kgen TriggerCount$Amount arm's CounterAddedOnce case is covered by
// TestCounterAddedOnceTriggerAmount.
func TestMonstrosityTriggerAmount(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredMonstrous})
	monstrous := kop.FilterID(len(tbl.Filters) - 1)
	body := []kop.Instr{{Op: kop.OpMonstrosity, Obj: kop.RefSelf, A: kop.ValueRef{K: kop.VTriggerAmount}}}
	w := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Monstrosity", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{
			{Kind: kop.AbActivated, Prog: body},
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomeMonstrous},
				Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.ValueRef{K: kop.VTriggerAmount}}}},
		}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	p := sc.perm(w, 0)
	s.res.e.kind, s.res.e.ctrl, s.res.e.obj, s.res.e.card = seAbility, 0, p, w
	s.res.e.srcVer = s.objs[p].ver
	sl := sc.slot(p)

	s.res.e.trigAmt = 3
	s.exec(sc.k, &body[0])
	if n := s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 3 {
		t.Fatalf("%d +1/+1 counters, want 3", n)
	}
	if !s.monstrousAt(sl) {
		t.Fatal("not monstrous after OpMonstrosity")
	}
	if !s.matchPerm(sc.k, monstrous, sl, 0, NoSlot) {
		t.Fatal("PredMonstrous does not match the monstrous permanent")
	}
	if s.npend != 1 || s.pend[0].trigAmt != 3 {
		t.Fatalf("%d pending, amount %d, want one BecomeMonstrous trigger with amount 3", s.npend, s.pend[0].trigAmt)
	}
	life := s.pl[0].life

	s.npend = 0
	s.res.e.trigAmt = 5
	s.exec(sc.k, &body[0])
	if n := s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 3 {
		t.Fatalf("a second OpMonstrosity added counters: %d, want 3", n)
	}
	if s.npend != 0 {
		t.Fatalf("a second OpMonstrosity fired %d triggers, want 0", s.npend)
	}

	q := sc.perm(w, 0)
	s.res.e.obj, s.res.e.srcVer = q, s.objs[q].ver
	s.res.e.trigAmt = 0
	s.exec(sc.k, &body[0])
	if n := s.perms[sc.slot(q)].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("a zero amount placed %d counters, want 0", n)
	}
	if s.monstrousAt(sc.slot(q)) {
		t.Fatal("a zero amount marked the permanent monstrous")
	}
	if s.pl[0].life != life {
		t.Fatalf("life changed from %d to %d without resolving the trigger", life, s.pl[0].life)
	}
}

// monstrosityTable builds a two-Plains table with one test creature whose
// activated ability is OpMonstrosity with amount A (and, when trig, a
// BecomeMonstrous trigger that gains its controller A life).
func monstrosityTable(t *testing.T, amt int, trig bool) (*kop.Table, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	w := CardID(len(tbl.Cards))
	ab := []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpMonstrosity, Obj: kop.RefSelf, A: kop.Const(amt)}}}}
	if trig {
		ab = append(ab, kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomeMonstrous},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.ValueRef{K: kop.VTriggerAmount}}}})
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Monstrosity", Types: kop.TCreature, Power: 2, Toughness: 2, Abilities: ab})
	return tbl, w
}

// TestMonstrosityOnce: a second activation on an already-monstrous
// permanent adds no counters, fires no BecomeMonstrous trigger and leaves
// the designation set (gorge's resolve-time MonstrositySet skip).
func TestMonstrosityOnce(t *testing.T) {
	tbl, w := monstrosityTable(t, 2, true)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	p := sc.perm(w, 0)
	body := tbl.Cards[w].Abilities[0].Prog
	s.res.e.kind, s.res.e.ctrl, s.res.e.obj, s.res.e.card = seAbility, 0, p, w
	s.res.e.srcVer = s.objs[p].ver
	sl := sc.slot(p)

	s.exec(sc.k, &body[0])
	if n := s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("%d +1/+1 counters, want 2", n)
	}
	if !s.monstrousAt(sl) {
		t.Fatal("not monstrous after the first OpMonstrosity")
	}
	if s.npend != 1 {
		t.Fatalf("%d pending triggers, want 1", s.npend)
	}
	s.npend = 0

	s.exec(sc.k, &body[0])
	if n := s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("a second OpMonstrosity added counters: %d, want 2", n)
	}
	if s.npend != 0 {
		t.Fatalf("a second OpMonstrosity fired %d triggers, want 0", s.npend)
	}
	if !s.monstrousAt(sl) {
		t.Fatal("a second OpMonstrosity cleared the designation")
	}
}

// TestMonstrousClearedOnLeave: removePerm clears the monstrous bit, so a
// blinked permanent comes back non-monstrous and a slot reused by another
// permanent does not inherit the designation.
func TestMonstrousClearedOnLeave(t *testing.T) {
	tbl, w := monstrosityTable(t, 2, false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	p := sc.perm(w, 0)
	body := tbl.Cards[w].Abilities[0].Prog
	s.res.e.kind, s.res.e.ctrl, s.res.e.obj, s.res.e.card = seAbility, 0, p, w
	s.res.e.srcVer = s.objs[p].ver
	sl := sc.slot(p)

	s.exec(sc.k, &body[0])
	if !s.monstrousAt(sl) {
		t.Fatal("not monstrous after OpMonstrosity")
	}

	// Blink: exile the monstrous permanent, then return the same object.
	s.removePerm(sc.k, sl, kop.ZoneExile, false)
	if s.monstrousAt(sl) {
		t.Fatal("the monstrous bit survived the permanent leaving the battlefield")
	}
	sl2 := s.enter(sc.k, p, 0, false, kop.ZoneNone)
	if s.monstrousAt(sl2) {
		t.Fatal("the blinked permanent came back monstrous")
	}

	// Reuse: mark it monstrous, remove it, and put a different permanent
	// into the freed slot.
	s.res.e.obj, s.res.e.srcVer = p, s.objs[p].ver
	s.exec(sc.k, &body[0])
	if !s.monstrousAt(sl2) {
		t.Fatal("not monstrous before the slot reuse")
	}
	s.removePerm(sc.k, sl2, kop.ZoneGraveyard, false)
	q := sc.obj(w, 0)
	sl3 := s.enter(sc.k, q, 0, false, kop.ZoneNone)
	if sl3 != sl2 {
		t.Fatalf("the reused slot is %d, want the freed %d", sl3, sl2)
	}
	if s.monstrousAt(sl3) {
		t.Fatal("a permanent entering a reused slot inherited the monstrous bit")
	}
}

// TestCounterAddedOnceTriggerAmount: TrigCounterAddedOnce carries the
// number of counters as VTriggerAmount, so kgen's TriggerCount$Amount arm
// (Nest of Scarabs, Shalai and Hallar, Terrasymbiosis, Casey Jones, Back
// Alley Brute) reads the event's amount, not a stale one.
func TestCounterAddedOnceTriggerAmount(t *testing.T) {
	tbl, w := monstrosityTable(t, 1, false)
	prog := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.ValueRef{K: kop.VTriggerAmount}}}
	tbl.Cards[w].Abilities = append(tbl.Cards[w].Abilities,
		kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce}, Prog: prog})
	b := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 1})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	watcher := sc.perm(w, 0)
	bear := sc.perm(b, 0)

	s.fireObj(sc.k, kop.TrigCounterAddedOnce, sc.slot(bear), bear, 0, 3)
	if s.npend != 1 || s.pend[0].trigAmt != 3 {
		t.Fatalf("%d pending, amount %d, want one counter-added trigger with amount 3", s.npend, s.pend[0].trigAmt)
	}
	life := s.pl[0].life
	s.res.e.kind, s.res.e.ctrl, s.res.e.obj, s.res.e.card = seAbility, 0, watcher, w
	s.res.e.srcVer = s.objs[watcher].ver
	s.res.e.trigAmt = 3
	s.exec(sc.k, &prog[0])
	if got := s.pl[0].life - life; got != 3 {
		t.Fatalf("gained %d life, want 3 (the counter event's amount)", got)
	}
}
