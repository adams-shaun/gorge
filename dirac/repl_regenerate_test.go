package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRegenerateShieldsStack: gorge effRegenerate adds one "Shield" marker
// per resolution and ReplaceDestruction spends one per destruction (CR
// 701.19a): two shields survive two destructions (tapped, damage removed,
// out of combat), the third destruction kills; a DestroyAll spends a
// shield too; the cleanup removes the rest.
func TestRegenerateShieldsStack(t *testing.T) {
	// The test runs OpRegenerate by hand; the table gets one so the mechanic-bit
	// arena holds the regeneration mask (arena.go).
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		cd := &tbl.Cards[id["Valley Floodcaller"]]
		cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpRegenerate, Obj: kop.RefSelf}}})
	})
	fc := id["Valley Floodcaller"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	regen := kop.Instr{Op: kop.OpRegenerate, Obj: kop.RefSelf}
	sc.s.exec(sc.k, &regen)
	sc.s.exec(sc.k, &regen)
	if n := sc.s.FeatRegenShields(a); n != 2 {
		t.Fatalf("shields %d, want 2", n)
	}
	pa := &sc.s.perms[sc.slot(a)]
	pa.damage = 1
	pa.flags |= pfAttacking
	for i := 0; i < 2; i++ {
		sc.s.destroy(sc.k, sc.slot(a))
		if !sc.onBF(a) {
			t.Fatalf("destruction %d killed a shielded creature", i+1)
		}
	}
	pa = &sc.s.perms[sc.slot(a)]
	if pa.damage != 0 || pa.flags&pfAttacking != 0 || !sc.tapped(a) || sc.s.FeatRegenShields(a) != 0 {
		t.Fatalf("regenerated: damage %d flags %#x shields %d", pa.damage, pa.flags, sc.s.FeatRegenShields(a))
	}
	sc.s.destroy(sc.k, sc.slot(a))
	if sc.onBF(a) {
		t.Fatal("the unshielded creature survived")
	}
	// DestroyAll: the shielded one regenerates, the other dies.
	sc.s.res.e.obj = b
	sc.s.exec(sc.k, &regen)
	c := sc.perm(fc, 0)
	all := kop.Instr{Op: kop.OpDestroyAll, F: firstInstr(t, sc, fc, kop.OpUntapAll).F}
	sc.s.exec(sc.k, &all)
	if !sc.onBF(b) || sc.onBF(c) {
		t.Fatalf("DestroyAll: shielded on battlefield %v, unshielded %v", sc.onBF(b), sc.onBF(c))
	}
	// shields stack (eight at most since kcaps2x); the cleanup clears them
	for i := 0; i < 6; i++ {
		sc.s.exec(sc.k, &regen)
	}
	if n := sc.s.FeatRegenShields(b); n != 6 {
		t.Fatalf("shields %d, want 6", n)
	}
	sc.s.spendRegenShield(sc.slot(b))
	if n := sc.s.FeatRegenShields(b); n != 5 {
		t.Fatalf("shields %d after one spent, want 5", n)
	}
}

// TestDestroyNoRegen: NoRegen$ True (OpDestroy C=1, OpDestroyAll C&2)
// destroys a shielded creature (gorge effDestroy/effDestroyAll skip
// ReplaceDestruction); RememberDestroyed$ without NoRegen$ regenerates.
func TestDestroyNoRegen(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 4}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a, b := sc.perm(fc, 0), sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	regen := kop.Instr{Op: kop.OpRegenerate, Obj: kop.RefSelf}
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	sc.s.exec(sc.k, &regen)
	sc.s.destroyNoRegen(sc.k, sc.slot(a))
	if sc.onBF(a) {
		t.Fatal("NoRegen destroy: the shielded creature regenerated")
	}
	f := firstInstr(t, sc, fc, kop.OpUntapAll).F
	sc.s.res.e.obj = b
	sc.s.exec(sc.k, &regen)
	rem := kop.Instr{Op: kop.OpDestroyAll, F: f, C: 1}
	sc.s.exec(sc.k, &rem)
	if !sc.onBF(b) {
		t.Fatal("RememberDestroyed DestroyAll: the shielded creature did not regenerate")
	}
	sc.s.exec(sc.k, &regen)
	nr := kop.Instr{Op: kop.OpDestroyAll, F: f, C: 2}
	sc.s.exec(sc.k, &nr)
	if sc.onBF(b) {
		t.Fatal("NoRegen DestroyAll: the shielded creature regenerated")
	}
}

// kverify-c4: kernel fixes the verification lockstep proved (the games
// themselves are dirac/oracle TestMixedArmsCountedLockstep and TestMixedArmsRegressions).

func replRegenerateScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "mixedarms")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the mixedarms table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestRegenNinthShieldStalls: pfRegenX plus genState.regenHi hold eight
// regeneration shields (four before kcaps2x; Fog of Gnats' fifth was the
// kverify-c4 hit); a ninth stalls loud instead of being dropped (gorge
// stacks every Shield marker). Spending counts back down across the
// regenHi bit.
func TestRegenNinthShieldStalls(t *testing.T) {
	sc, id := replRegenerateScen(t, "Fog of Gnats")
	sl := sc.slot(sc.perm(id["Fog of Gnats"], 0))
	for i := 0; i < 8; i++ {
		sc.s.addRegenShield(sl)
		if n := sc.s.regenShieldsAt(sl); n != i+1 || sc.s.Stalled() {
			t.Fatalf("shield %d: count %d, stalled %v", i+1, n, sc.s.Stalled())
		}
	}
	for want := 7; want >= 0; want-- {
		sc.s.spendRegenShield(sl)
		if n := sc.s.regenShieldsAt(sl); n != want {
			t.Fatalf("after a spend: %d shields, want %d", n, want)
		}
	}
	for i := 0; i < 8; i++ {
		sc.s.addRegenShield(sl)
	}
	sc.s.addRegenShield(sl)
	if sc.s.StallReason() != StallRegen {
		t.Fatalf("a ninth shield: stall %q, want regen", StallName(sc.s.StallReason()))
	}
}

// TestRegenShieldsDoNotSurviveSlotReuse: genState.regenHi's third bit is
// keyed by slot, so a slot freed before cleanup and reused by a new
// permanent must not hand it the old permanent's extra shields.
func TestRegenShieldsDoNotSurviveSlotReuse(t *testing.T) {
	sc, id := replRegenerateScen(t, "Fog of Gnats")
	sl := sc.slot(sc.perm(id["Fog of Gnats"], 0))
	for i := 0; i < 8; i++ {
		sc.s.addRegenShield(sl)
	}
	if n := sc.s.regenShieldsAt(sl); n != 8 {
		t.Fatalf("eight shields: %d", n)
	}
	sc.s.removePerm(sc.k, sl, kop.ZoneGraveyard, false)
	o := sc.perm(id["Fog of Gnats"], 0)
	sl = sc.slot(o)
	sc.s.addRegenShield(sl)
	if n := sc.s.regenShieldsAt(sl); n != 1 {
		t.Fatalf("a reused slot starts with %d shields, want 1", n)
	}
}

// TestSkipUntapTwiceIsOneMark: Homarid Warrior's "doesn't untap during your next
// untap step" made twice needs no second mark, since both effects end with that step
// and gorge asks no order between them. Oketra's Last Mercy twice likewise.
func TestSkipUntapTwiceIsOneMark(t *testing.T) {
	sc, id := replRegenerateScen(t, "Homarid Warrior")
	o := sc.perm(id["Homarid Warrior"], 0)
	sc.s.res.e = stackEntry{kind: seAbility, obj: o, srcVer: sc.s.objs[o].ver, card: id["Homarid Warrior"], ctrl: 0}
	in := kop.Instr{Op: kop.OpSkipUntap, Obj: kop.RefSelf}
	sc.s.skipUntap(sc.k, &in)
	sc.s.skipUntap(sc.k, &in)
	if sc.s.g.stalled != 0 || sc.s.perms[sc.slot(o)].cseen&cseenSkipUntap0 == 0 {
		t.Fatalf("two effects: stall %q, mark %v", StallName(sc.s.g.stalled), sc.s.perms[sc.slot(o)].cseen&cseenSkipUntap0 != 0)
	}
	sc2, _ := replRegenerateScen(t, "Oketra's Last Mercy")
	land := kop.Instr{Op: kop.OpSkipUntap, C: 1}
	sc2.s.skipUntap(sc2.k, &land)
	sc2.s.skipUntap(sc2.k, &land)
	if sc2.s.g.stalled != 0 || sc2.s.g.noUntapL&1 == 0 {
		t.Fatalf("two lands effects: stall %q, mark %d", StallName(sc2.s.g.stalled), sc2.s.g.noUntapL)
	}
}

// TestDiscoverTailKeepsQueuedCast: the discover tail (OpDiscover C=1)
// sends the declined discovery to the hand, but not a card the free cast
// picked: it is still exiled only because the kernel casts it after the
// resolution (gorge casts it at once; Walk with the Ancestors discovering
// Huatli's Raptor).
func TestDiscoverTailKeepsQueuedCast(t *testing.T) {
	sc, id := replRegenerateScen(t, "Huatli's Raptor", "Cenote Scout")
	cast := sc.obj(id["Huatli's Raptor"], 0)
	sc.s.place(cast, kop.ZoneExile, false)
	declined := sc.obj(id["Cenote Scout"], 0)
	sc.s.place(declined, kop.ZoneExile, false)
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0}
	tail := kop.Instr{Op: kop.OpDiscover, C: 1}

	sc.s.g.nrem = 0
	sc.s.remember(cast)
	sc.s.g.freeQ[0], sc.s.g.nfq = cast, 1
	sc.s.discover(sc.k, &tail)
	if z := sc.s.objs[cast].zone; z != kop.ZoneExile {
		t.Fatalf("the queued cast left exile: zone %d", z)
	}

	sc.s.g.nrem, sc.s.g.nfq = 0, 0
	sc.s.remember(declined)
	sc.s.discover(sc.k, &tail)
	if z := sc.s.objs[declined].zone; z != kop.ZoneHand {
		t.Fatalf("the declined discovery: zone %d, want the hand", z)
	}
}
