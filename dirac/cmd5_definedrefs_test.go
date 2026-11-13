package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// cmd5HostScen is a BLB scenario whose Valley Floodcaller carries one extra
// activated ability running prog, so the table's programs reach the ops the
// test runs by hand (the arena and the prep flags read the table's programs).
func cmd5HostScen(t *testing.T, prog []kop.Instr) (*scen, CardID) {
	t.Helper()
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Carrot Cake", Count: 1}, {Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		cd := &tbl.Cards[id["Valley Floodcaller"]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbActivated, Prog: prog})
	})
	return sc, id["Valley Floodcaller"]
}

// TestDealDamageNoRegenMark: a creature dealt damage above 0 by an
// instruction carrying DmgNoRegen keeps its regeneration shield but cannot
// spend it, so lethal damage and a destroy kill it; an unmarked shielded
// creature regenerates; a prevented hit (0 damage) marks nothing; the mark
// ends at the turn's cleanup (Incinerate's "can't be regenerated this turn").
func TestDealDamageNoRegenMark(t *testing.T) {
	regen := kop.Instr{Op: kop.OpRegenerate, Obj: kop.RefSelf}
	hit := func(n int) kop.Instr {
		return kop.Instr{Op: kop.OpDealDamageEx, Obj: kop.RefTarget0, A: kop.Const(n), C: kop.DmgNoRegen}
	}
	sc, fc := cmd5HostScen(t, []kop.Instr{regen, hit(3)})
	s, k := sc.s, sc.k
	if k.gen.ix[mechNoRegen] == arenaNone {
		t.Fatal("the table's DmgNoRegen damage did not give the arena a mechNoRegen mask")
	}
	marked, control := sc.perm(fc, 1), sc.perm(fc, 1)
	s.res.e.ctrl = 0
	for _, o := range []Obj{marked, control} {
		s.res.e.obj, s.res.e.card = o, fc
		s.exec(k, &regen)
	}
	// a zero hit marks nothing
	s.res.e.tgt[0] = TCode(marked)
	zero := hit(0)
	s.exec(k, &zero)
	if s.mSl(k, mechNoRegen, sc.slot(marked)) {
		t.Fatal("a hit of 0 marked the creature")
	}
	h := hit(1)
	s.exec(k, &h)
	if !s.mSl(k, mechNoRegen, sc.slot(marked)) {
		t.Fatal("damage above 0 did not mark the creature")
	}
	if s.mSl(k, mechNoRegen, sc.slot(control)) {
		t.Fatal("the unhit creature is marked")
	}
	if n := s.FeatRegenShields(marked); n != 1 {
		t.Fatalf("the marked creature's shield count %d, want 1 (kept, not spendable)", n)
	}
	// destroying: the marked one dies with its shield, the control regenerates
	s.destroy(k, sc.slot(marked))
	s.destroy(k, sc.slot(control))
	if sc.onBF(marked) {
		t.Error("the marked creature regenerated")
	}
	if !sc.onBF(control) {
		t.Error("the unmarked shielded creature did not regenerate")
	}
	// lethal damage through the state-based pass: a fresh marked creature dies
	// despite its shield
	again := sc.perm(fc, 1)
	s.res.e.obj = again
	s.exec(k, &regen)
	s.res.e.tgt[0] = TCode(again)
	three := hit(3)
	s.exec(k, &three)
	s.sba(k)
	if sc.onBF(again) {
		t.Error("lethal damage on a marked, shielded creature did not kill it")
	}
	// the cleanup ends the mark
	other := sc.perm(fc, 1)
	s.res.e.obj = other
	s.res.e.tgt[0] = TCode(other)
	s.exec(k, &h)
	if !s.mSl(k, mechNoRegen, sc.slot(other)) {
		t.Fatal("setup: the mark is not set")
	}
	s.newTurnGen()
	if s.mSl(k, mechNoRegen, sc.slot(other)) {
		t.Error("the mark survived the turn's cleanup")
	}
}

// TestDealDamageFromAnyTarget: OpDealDamageFrom's trigger-object mode with a
// target slot (Scourge of Valkas: "it deals X damage to any target") deals to
// a player or a permanent from the trigger object, and a trigger object that
// left the battlefield deals from its last known information (a lifelink
// source still gains its controller the life).
func TestDealDamageFromAnyTarget(t *testing.T) {
	deal := kop.Instr{Op: kop.OpDealDamageFrom, Obj: kop.RefTriggerObj, A: kop.Const(2), B: kop.Const(1), C: int16(kop.RefTarget0)}
	sc, fc := cmd5HostScen(t, []kop.Instr{deal})
	s, k := sc.s, sc.k
	src := sc.perm(fc, 0)
	victim := sc.perm(fc, 1)
	s.res.e.ctrl, s.res.e.obj, s.res.e.card, s.res.e.trigObj = 0, src, fc, src
	s.res.e.srcVer, s.res.e.nt = s.objs[src].ver, 1

	// to a player
	s.res.e.tgt[0] = TCode(PlayerTarget0 + 1)
	life := s.pl[1].life
	s.exec(k, &deal)
	if s.pl[1].life != life-2 {
		t.Errorf("player target: life %d, want %d", s.pl[1].life, life-2)
	}

	// to a permanent, from the live source
	s.res.e.tgt[0] = TCode(victim)
	s.exec(k, &deal)
	if d := s.perms[sc.slot(victim)].damage; d != 2 {
		t.Errorf("permanent target: damage %d, want 2", d)
	}
	if d := s.perms[sc.slot(src)].damage; d != 0 {
		t.Errorf("the source took %d, want 0 (one-sided)", d)
	}

	// from a source that left: last known information, lifelink read from it
	gone := sc.perm(fc, 0)
	s.leave(k, sc.slot(gone), kop.ZoneGraveyard, false)
	before := s.perms[sc.slot(victim)].damage
	life0 := s.pl[0].life
	s.dmgSrcObject(k, victim, gone, NoSlot, kop.KwLifelink, 0, false, 3)
	if d := s.perms[sc.slot(victim)].damage; d != before+3 {
		t.Errorf("left source: damage %d, want %d", d, before+3)
	}
	if s.pl[0].life != life0+3 {
		t.Errorf("left source with lifelink: controller life %d, want %d", s.pl[0].life, life0+3)
	}
	// a recipient that left the battlefield takes nothing
	s.leave(k, sc.slot(victim), kop.ZoneGraveyard, false)
	s.dmgSrcObject(k, victim, gone, NoSlot, 0, 0, false, 3)
	if s.objs[victim].zone != kop.ZoneGraveyard {
		t.Error("a recipient that left the battlefield moved again")
	}
}

// TestTokenSizeFromRememberedManaValue: an X/X token whose size is the
// remembered card's mana value (Skyclave Apparition's leave trigger for the
// exiled card's owner): a mana value 3 card makes a 3/3 for its owner; a
// mana value 0 card makes a 0/0 that dies to the state-based pass (CR 704.5f).
func TestTokenSizeFromRememberedManaValue(t *testing.T) {
	var tok CardID
	var pl CardID
	build := func(sc *scen, fc CardID) []kop.Instr {
		tbl := sc.k.t
		for i := range tbl.Cards {
			if tbl.Cards[i].Types&kop.TToken != 0 && tbl.Cards[i].Types&kop.TCreature != 0 {
				tok = CardID(i)
				break
			}
		}
		tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExMVOf, Ref: kop.RefRemembered})
		mv := kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}
		tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, ValPT: true, PA: mv, TA: mv})
		return []kop.Instr{
			{Op: kop.OpExt, A: kop.Const(int(kop.ExtTokenPT)), C: int16(len(tbl.Overrides) - 1)},
			{Op: kop.OpCreateTokenEx, Obj: kop.RefRememberedOwner, A: kop.Const(1), C: int16(tok)},
			{Op: kop.OpClearRemembered},
		}
	}
	run := func(t *testing.T, exiled CardID) (power int, alive bool, owner uint8) {
		sc, fc := cmd5HostScen(t, nil)
		prog := build(sc, fc)
		s, k := sc.s, sc.k
		cd := &k.t.Cards[fc]
		cd.Abilities[len(cd.Abilities)-1].Prog = prog
		if tok == 0 {
			t.Fatal("no creature token in the BLB table")
		}
		src := sc.perm(fc, 0)
		x := sc.obj(exiled, 1) // the exiled card: player 1's
		s.detach(x)
		s.place(x, kop.ZoneExile, false)
		s.g.rem[0], s.g.nrem = x, 1
		s.res = resCtx{active: true, lastTok: NoObj, eachObj: NoObj}
		s.res.e = stackEntry{kind: seTrigger, ctrl: 0, obj: src, card: fc, ab: uint8(len(cd.Abilities) - 1), srcVer: s.objs[src].ver}
		s.runProg(k)
		o := s.res.lastTok
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			t.Fatalf("no token on the battlefield (lastTok %d)", o)
		}
		sl := s.objs[o].slot
		owner = s.perms[sl].ctrl
		power = s.power(k, sl)
		s.sba(k)
		return power, s.objs[o].zone == kop.ZoneBattlefield, owner
	}
	_ = pl
	sc0, fc0 := cmd5HostScen(t, nil)
	_ = sc0
	if c := sc0.k.t.Cards[fc0].Cost; c.Generic == 0 && c.Pips == [6]uint8{} {
		t.Fatal("setup: the host card has no mana cost")
	}
	// mana value 3 (the host's own cost) versus 0 (a land)
	p, alive, owner := run(t, fc0)
	if mv := int(sc0.k.t.Cards[fc0].Cost.Generic) + int(pips(sc0.k.t.Cards[fc0].Cost)); p != mv || !alive || owner != 1 {
		t.Errorf("mana value %d card: token power %d alive %v controller %d, want %d, alive, 1", mv, p, alive, owner, mv)
	}
	plains := cmd5LandID(t, sc0)
	p, alive, owner = run(t, plains)
	if p != 0 || alive || owner != 1 {
		t.Errorf("mana value 0 card: token power %d alive %v controller %d, want 0, dead, 1", p, alive, owner)
	}
}

func pips(c kop.ManaCost) (n int) {
	for _, p := range c.Pips {
		n += int(p)
	}
	return n
}

func cmd5LandID(t *testing.T, sc *scen) CardID {
	t.Helper()
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Name == "Plains" {
			return CardID(i)
		}
	}
	t.Fatal("no Plains")
	return 0
}
