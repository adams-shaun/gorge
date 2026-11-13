package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Battles (face_battle.go) on a hand table: vanilla plus a transforming
// Siege (defense 3) whose back face is a 2/2 creature.

func battleScen(t *testing.T, defense int8) (*scen, CardID, CardID) {
	t.Helper()
	return battleScenBack(t, defense, kop.TCreature)
}

func battleScenBack(t *testing.T, defense int8, backTypes kop.TypeMask) (*scen, CardID, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Name = "battletest"
	tbl.Exprs = make([]kop.Expr, 1) // a generated-table prep (prep.gen)
	front := CardID(len(v.Table.Cards))
	back := front + 1
	bc := kop.CardDef{Name: "Test Back", Types: backTypes, Power: 2, Toughness: 2, FaceMode: kop.FaceTransform, Faces: []CardID{front, back}, Face: 1}
	if backTypes&kop.TSorcery != 0 {
		bc.Abilities = []kop.Ability{{}} // an empty resolve program
	}
	tbl.Cards = append(slices.Clone(v.Table.Cards),
		kop.CardDef{Name: "Test Siege", Types: kop.TBattle, Defense: defense, FaceMode: kop.FaceTransform, Faces: []CardID{front, back}},
		bc)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if !sc.k.gen.anyBattle {
		t.Fatal("anyBattle not set")
	}
	return sc, front, back
}

func (sc *scen) defense(o Obj) int { return sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrDefense)) }

// attackOffers counts the AkAttack options of attacker a naming target tgt.
func attackOffers(sc *scen, a, tgt Obj) (n int) {
	var d Decision
	sc.s.attackerOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if x := d.Options[i]; x.Kind() == AkAttack && x.Source() == TCode(uint8(a)) && x.Target() == TCode(uint8(tgt)) {
			n++
		}
	}
	return n
}

// battleCombat attacks tgt with the attackers, blocks (blocker, attacker
// pairs), and passes to main 2.
func battleCombat(t *testing.T, sc *scen, atk []Obj, tgt Obj, blocks [][2]Obj) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range atk {
		sc.do(t, AkAttack, TCode(a), TCode(tgt))
	}
	sc.do(t, AkDone, scAny, scAny)
	sc.passUntil(t, kop.StepBlockers)
	if len(blocks) > 0 {
		for _, b := range blocks {
			sc.do(t, AkBlock, TCode(b[0]), TCode(b[1]))
		}
		sc.do(t, AkDone, scAny, scAny)
	}
	sc.passUntil(t, kop.StepMain2)
}

func TestBattleEntersWithDefense(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	b0 := sc.perm(front, 0)
	b1 := sc.perm(front, 1)
	for _, c := range []struct {
		o    Obj
		prot uint8
	}{{b0, 1}, {b1, 0}} {
		if n := sc.defense(c.o); n != 3 {
			t.Errorf("obj %d enters with %d defense counters, want 3", c.o, n)
		}
		if pr, ok := sc.s.protector(sc.slot(c.o)); !ok || pr != c.prot {
			t.Errorf("obj %d protector %d (%v), want %d", c.o, pr, ok, c.prot)
		}
	}
	// a face-down entry grants neither (CR 708.5; gorge
	// TestBattleFaceDownEntryGrantsNothingAndDoesNotPark)
	o := sc.obj(front, 0)
	sl := sc.s.enterNF(sc.k, o, 0, false, kop.ZoneNone, 1)
	if sc.defense(o) != 0 {
		t.Errorf("face-down entry has %d defense counters", sc.defense(o))
	}
	if _, ok := sc.s.protector(sl); ok {
		t.Errorf("face-down entry has a protector")
	}
}

func TestBattleAttackOffers(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	bears := sc.perm(v.GrizzlyBears, 0)
	own := sc.perm(front, 0)    // protected by seat 1: attackable by seat 0
	theirs := sc.perm(front, 1) // protected by seat 0: never attackable by seat 0
	if attackOffers(sc, bears, own) != 1 || attackOffers(sc, bears, theirs) != 0 {
		t.Fatalf("offers own %d theirs %d, want 1 and 0", attackOffers(sc, bears, own), attackOffers(sc, bears, theirs))
	}
	// a control change keeps the protector: seat 1 now controls the battle
	// seat 1 protects, and seat 0 still may attack it
	sl := sc.slot(own)
	sc.s.ctrlM[0].clr(sl)
	sc.s.ctrlM[1].set(sl)
	sc.s.perms[sl].ctrl = 1
	if attackOffers(sc, bears, own) != 1 {
		t.Fatalf("after a control change the battle is not offered")
	}
}

func TestBattleCombatDamage(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	bears := sc.perm(v.GrizzlyBears, 0)
	b := sc.perm(front, 0)
	battleCombat(t, sc, []Obj{bears}, b, nil)
	if sc.defense(b) != 1 || sc.s.perms[sc.slot(b)].damage != 0 {
		t.Fatalf("unblocked 2 power: defense %d damage %d, want 1 and 0", sc.defense(b), sc.s.perms[sc.slot(b)].damage)
	}

	// a blocked attacker deals no battle damage
	sc, front, _ = battleScen(t, 3)
	bears = sc.perm(v.GrizzlyBears, 0)
	blk := sc.perm(v.GrizzlyBears, 1)
	b = sc.perm(front, 0)
	battleCombat(t, sc, []Obj{bears}, b, [][2]Obj{{blk, bears}})
	if sc.defense(b) != 3 {
		t.Fatalf("blocked attacker: defense %d, want 3", sc.defense(b))
	}

	// trample's excess lands on the battle: 6 power blocked by a 2/2 leaves
	// 4, more than the 3 defense, which defeats it
	sc, front, _ = battleScen(t, 3)
	dread := sc.perm(v.ColossalDreadmaw, 0)
	blk = sc.perm(v.GrizzlyBears, 1)
	b = sc.perm(front, 0)
	battleCombat(t, sc, []Obj{dread}, b, [][2]Obj{{blk, dread}})
	if sc.s.objs[b].zone != kop.ZoneExile {
		t.Fatalf("trample excess: battle zone %d, want exile", sc.s.objs[b].zone)
	}
}

func TestBattleSpellDamage(t *testing.T) {
	sc, front, _ := battleScen(t, 3)
	sc.lands(0, v.Mountain, 1)
	b := sc.perm(front, 0)
	shock := sc.hand(v.Shock, 0)
	sc.do(t, AkCast, TCode(shock), scAny)
	sc.do(t, AkTarget, scAny, TCode(b))
	sc.passBoth(t)
	if sc.defense(b) != 1 || sc.s.perms[sc.slot(b)].damage != 0 {
		t.Fatalf("Shock: defense %d damage %d, want 1 and 0", sc.defense(b), sc.s.perms[sc.slot(b)].damage)
	}
}

func TestBattleDefeatExiles(t *testing.T) {
	sc, front, _ := battleScen(t, 2)
	sc.lands(0, v.Mountain, 1)
	b := sc.perm(front, 1)
	shock := sc.hand(v.Shock, 0)
	sc.do(t, AkCast, TCode(shock), scAny)
	sc.do(t, AkTarget, scAny, TCode(b))
	sc.passBoth(t)
	s := sc.s
	s.Step() // the state-based actions run before the next priority
	ob := &s.objs[b]
	if ob.zone != kop.ZoneExile || ob.card != front {
		t.Fatalf("defeated battle zone %d card %d, want exile face 0", ob.zone, ob.card)
	}
	if n := s.pl[1].exileN; n != 1 || s.pl[1].exile[0] != b || s.pl[1].graveN != 0 {
		t.Fatalf("exile %d grave %d, want the battle in exile only", n, s.pl[1].graveN)
	}
	if s.g.nmay != 1 {
		t.Fatalf("%d play records, want 1", s.g.nmay)
	}
	if r := s.g.may[0]; r.until != kop.DurDefeated || r.obj != b || r.ver != ob.ver || r.ctrl != 1 {
		t.Fatalf("record %+v", r)
	}
}

// defeatToAsk defeats the battle at front (seat 1's, protected by seat 0)
// with a Shock and returns it at the CR 310.11 ask.
func defeatToAsk(t *testing.T, sc *scen, front CardID) Obj {
	t.Helper()
	sc.lands(0, v.Mountain, 1)
	b := sc.perm(front, 1)
	shock := sc.hand(v.Shock, 0)
	sc.do(t, AkCast, TCode(shock), scAny)
	sc.do(t, AkTarget, scAny, TCode(b))
	sc.passBoth(t)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDefeatCast || d.Player != 1 {
		t.Fatalf("after the defeat: decision %v/%d player %d, want the owner's defeat ask", d.Kind, d.Sub, d.Player)
	}
	return b
}

func TestBattleDefeatCastYes(t *testing.T) {
	sc, front, back := battleScen(t, 2)
	b := defeatToAsk(t, sc, front)
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 1))
	sc.buildToStack(t)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneStack || ob.card != back || sc.s.sp != 1 {
		t.Fatalf("yes: zone %d card back %v stack %d, want the free back face on the stack", ob.zone, ob.card == back, sc.s.sp)
	}
	sc.resolveAll(t)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneBattlefield || ob.card != back || sc.s.cardAt(sc.slot(b)) != back {
		t.Fatalf("yes: zone %d card back %v, want a face-1 permanent", ob.zone, ob.card == back)
	}
	if sc.s.g.nmay != 0 {
		t.Fatalf("%d play records left, want 0", sc.s.g.nmay)
	}
}

func TestBattleDefeatCastNo(t *testing.T) {
	sc, front, _ := battleScen(t, 2)
	b := defeatToAsk(t, sc, front)
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 0))
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneExile || ob.card != front {
		t.Fatalf("no: zone %d card front %v, want exile face 0", ob.zone, ob.card == front)
	}
	if sc.s.g.nmay != 0 {
		t.Fatalf("%d play records left, want 0", sc.s.g.nmay)
	}
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChDefeatCast {
		t.Fatal("a second defeat ask was offered")
	}
}

func TestBattleDefeatCastSorcery(t *testing.T) {
	sc, front, back := battleScenBack(t, 2, kop.TSorcery)
	b := defeatToAsk(t, sc, front)
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 1))
	sc.buildToStack(t)
	if ob := &sc.s.objs[b]; ob.card != back {
		t.Fatal("the sorcery back face is not on the stack")
	}
	sc.resolveAll(t)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneGraveyard || ob.card != back {
		t.Fatalf("sorcery back: zone %d card back %v, want the graveyard at face 1", ob.zone, ob.card == back)
	}
}

func TestBattleDefeatReversalFront(t *testing.T) {
	sc, front, back := battleScen(t, 2)
	b := defeatToAsk(t, sc, front)
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 1))
	sc.s.Step()
	if ob := &sc.s.objs[b]; ob.card != back {
		t.Fatal("the free cast did not turn the card to its back face")
	}
	sc.s.reverseCast(sc.k)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneExile || ob.card != front {
		t.Fatalf("reversed: zone %d card front %v, want exile face 0", ob.zone, ob.card == front)
	}
}
