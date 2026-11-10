package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// scenario builds a State at the start of seat 0's main phase 1 (turn 2,
// so nothing is skipped) with the given permanents (not summoning sick),
// hands, and 20 Plains in each library.
type scen struct {
	s *State
	k *prep
}

func newScen(t *testing.T, cfg Config) *scen {
	t.Helper()
	return newScenTable(t, v.Table, cfg)
}

// newScenTable is newScen over table tbl (a copy of vanilla's with extra
// cards appended: its Plains id is vanilla's).
func newScenTable(t *testing.T, tbl *kop.Table, cfg Config) *scen {
	t.Helper()
	return newScenTableLib(t, tbl, v.Plains, cfg)
}

// newScenTableLib is newScenTable with an explicit library card (a pool
// table built by kgen does not share vanilla's Plains id).
func newScenTableLib(t *testing.T, tbl *kop.Table, libCard CardID, cfg Config) *scen {
	t.Helper()
	var lib [20]CardID
	for i := range lib {
		lib[i] = libCard
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, cfg)
	// empty the hands back into the libraries
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn = 2
	s.active = 0
	s.step = kop.StepMain1
	s.prio = 0
	s.passes = 0
	s.ctl = ctlPriority
	return &scen{s: s, k: s.k()}
}

func (sc *scen) obj(c CardID, owner uint8) Obj {
	s := sc.s
	o := Obj(s.nobj)
	s.nobj++
	s.objs[o] = object{card: c, chosen: 255, slot: NoSlot, acts: owner << objOwnerShift}
	return o
}

// perm puts card c onto the battlefield under p (untapped, not sick).
func (sc *scen) perm(c CardID, p uint8) Obj {
	o := sc.obj(c, p)
	sl := sc.s.enter(sc.k, o, p, false, kop.ZoneNone)
	sc.s.perms[sl].flags &^= pfSick
	sc.s.npend = 0
	return o
}

func (sc *scen) hand(c CardID, p uint8) Obj {
	o := sc.obj(c, p)
	sc.s.place(o, kop.ZoneHand, false)
	return o
}

func (sc *scen) slot(o Obj) Slot { return sc.s.objs[o].slot }

func (sc *scen) onBF(o Obj) bool { return sc.s.objs[o].zone == kop.ZoneBattlefield }

// do steps to the next decision and applies the option matching want
// (kind + source + target), failing if it is not offered.
func (sc *scen) do(t *testing.T, kind ActionKind, src, tgt uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == kind && (src == 0xff || a.Source() == src) && (tgt == 0xff || a.Target() == tgt) {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("option kind %d src %d tgt %d not offered; decision kind %d player %d opts %v", kind, src, tgt, d.Kind, d.Player, d.Options[:d.N])
}

// passTo passes priority until the game reaches step st of the current
// turn (or a non-priority decision).
func (sc *scen) passUntil(t *testing.T, st kop.Step) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if sc.s.step == st && sc.s.ctl != ctlStepBegin {
			return
		}
		d := sc.s.Step()
		if d.Kind != DkPriority {
			return
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatalf("never reached step %d", st)
}

// combat runs a whole combat: attackers, then blocks (pairs blocker,
// attacker), and passes to main 2.
func (sc *scen) combat(t *testing.T, attackers []Obj, blocks [][2]Obj) {
	t.Helper()
	sc.passUntil(t, kop.StepAttackers)
	for _, a := range attackers {
		sc.do(t, AkAttack, uint8(a), 0xff)
	}
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepBlockers)
	if len(blocks) > 0 || sc.s.ctl == ctlBlockers {
		for _, b := range blocks {
			sc.do(t, AkBlock, uint8(b[0]), uint8(b[1]))
		}
		sc.do(t, AkDone, 0xff, 0xff)
	}
	sc.passUntil(t, kop.StepMain2)
}

func TestFirstStrikeBeatsDeathtouch(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	kn := sc.perm(v.YouthfulKnight, 0) // 2/1 first strike
	rat := sc.perm(v.TyphoidRats, 1)   // 1/1 deathtouch
	sc.combat(t, []Obj{kn}, [][2]Obj{{rat, kn}})
	if !sc.onBF(kn) || sc.onBF(rat) {
		t.Fatalf("knight alive=%v rat alive=%v; want knight alive, rat dead", sc.onBF(kn), sc.onBF(rat))
	}
}

func TestDeathtouchTradesWithoutFirstStrike(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	g := sc.perm(v.HillGiant, 0)     // 3/3
	rat := sc.perm(v.TyphoidRats, 1) // 1/1 deathtouch
	sc.combat(t, []Obj{g}, [][2]Obj{{rat, g}})
	if sc.onBF(g) || sc.onBF(rat) {
		t.Fatalf("giant alive=%v rat alive=%v; want both dead", sc.onBF(g), sc.onBF(rat))
	}
}

func TestTrampleDeathtouch(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	dm := sc.perm(v.ColossalDreadmaw, 0) // 6/6 trample
	sl := sc.slot(dm)
	sc.s.perms[sl].eotKw |= kop.KwDeathtouch
	wall := sc.perm(v.TundraWall, 1) // 0/4
	sc.combat(t, []Obj{dm}, [][2]Obj{{wall, dm}})
	if sc.onBF(wall) {
		t.Fatalf("wall survived trample+deathtouch")
	}
	if got := sc.s.pl[1].life; got != 15 {
		t.Fatalf("defender life %d, want 15 (1 lethal to the wall, 5 trampled over)", got)
	}
}

func TestTrampleWithoutDeathtouch(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	dm := sc.perm(v.ColossalDreadmaw, 0) // 6/6 trample
	wall := sc.perm(v.TundraWall, 1)     // 0/4
	sc.combat(t, []Obj{dm}, [][2]Obj{{wall, dm}})
	if sc.onBF(wall) || sc.s.pl[1].life != 18 {
		t.Fatalf("wall alive=%v life=%d; want dead, 18", sc.onBF(wall), sc.s.pl[1].life)
	}
}

func TestDoubleStrikeLifelink(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	ace := sc.perm(v.FencingAce, 0) // 1/1 double strike
	sc.s.perms[sc.slot(ace)].eotKw |= kop.KwLifelink
	sc.combat(t, []Obj{ace}, nil)
	if sc.s.pl[1].life != 18 || sc.s.pl[0].life != 22 {
		t.Fatalf("life %d/%d, want 22/18", sc.s.pl[0].life, sc.s.pl[1].life)
	}
}

func TestDoubleStrikeKillsFirstStrikeBlocker(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	ace := sc.perm(v.FencingAce, 0)    // 1/1 double strike
	kn := sc.perm(v.YouthfulKnight, 1) // 2/1 first strike
	sc.combat(t, []Obj{ace}, [][2]Obj{{kn, ace}})
	if sc.onBF(ace) || sc.onBF(kn) {
		t.Fatalf("ace alive=%v knight alive=%v; both deal first-strike damage simultaneously, both die", sc.onBF(ace), sc.onBF(kn))
	}
}

func TestMenaceNeedsTwoBlockers(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	og := sc.perm(v.BoggartBrute, 0) // 3/2 menace
	b1 := sc.perm(v.GrizzlyBears, 1)
	// one possible blocker: the menace attacker is not blockable at all
	sc.combat(t, []Obj{og}, nil)
	if !sc.onBF(og) || !sc.onBF(b1) || sc.s.pl[1].life != 17 {
		t.Fatalf("menace with one blocker: ogre=%v bears=%v life=%d", sc.onBF(og), sc.onBF(b1), sc.s.pl[1].life)
	}
	sc2 := newScen(t, Config{NoElide: true, AutoDivide: true})
	og = sc2.perm(v.BoggartBrute, 0)
	b1 = sc2.perm(v.GrizzlyBears, 1)
	b2 := sc2.perm(v.GrizzlyBears, 1)
	sc2.combat(t, []Obj{og}, [][2]Obj{{b1, og}, {b2, og}})
	// 3 damage: lethal 2 to the first blocker, the rest (1) to the last
	if sc2.onBF(og) || sc2.onBF(b1) || !sc2.onBF(b2) || sc2.s.pl[1].life != 20 {
		t.Fatalf("double block: ogre=%v b1=%v b2=%v life=%d", sc2.onBF(og), sc2.onBF(b1), sc2.onBF(b2), sc2.s.pl[1].life)
	}
}

func TestFlyingReach(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	dr := sc.perm(v.WindDrake, 0) // 2/2 flying
	bears := sc.perm(v.GrizzlyBears, 1)
	spider := sc.perm(v.GiantSpider, 1) // 2/4 reach
	k := sc.k
	if sc.s.canBlock(k, sc.slot(bears), sc.slot(dr)) {
		t.Fatalf("a ground creature can block a flier")
	}
	if !sc.s.canBlock(k, sc.slot(spider), sc.slot(dr)) {
		t.Fatalf("reach can't block a flier")
	}
	sc.combat(t, []Obj{dr}, [][2]Obj{{spider, dr}})
	if sc.onBF(dr) || !sc.onBF(spider) {
		t.Fatalf("drake=%v spider=%v; want drake dead", sc.onBF(dr), sc.onBF(spider))
	}
}

func TestVigilanceAndDefender(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	an := sc.perm(v.SerraAngel, 0)
	wall := sc.perm(v.TundraWall, 0)
	if sc.s.canAttack(sc.k, sc.slot(wall)) {
		t.Fatalf("defender can attack")
	}
	sc.combat(t, []Obj{an}, nil)
	if sc.s.perms[sc.slot(an)].flags&pfTapped != 0 {
		t.Fatalf("vigilance attacker tapped")
	}
	if sc.s.pl[1].life != 16 {
		t.Fatalf("life %d want 16", sc.s.pl[1].life)
	}
}

func TestLightningBoltAndDiesTrigger(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.perm(v.Mountain, 0)
	bolt := sc.hand(v.LightningBolt, 0)
	myr := sc.perm(v.PerilousMyr, 1)
	sc.do(t, AkCast, uint8(bolt), 0xff)
	sc.do(t, AkTarget, 0xff, uint8(myr))
	// both pass: bolt resolves, myr dies, its trigger targets
	sc.do(t, AkPass, 0xff, 0xff)
	sc.do(t, AkPass, 0xff, 0xff)
	// SBA: the myr dies; its trigger asks seat 1 for a target: seat 0's face
	sc.do(t, AkTarget, 0xff, PlayerTarget0)
	if sc.onBF(myr) {
		t.Fatalf("myr survived bolt")
	}
	sc.do(t, AkPass, 0xff, 0xff)
	sc.do(t, AkPass, 0xff, 0xff)
	if sc.s.pl[0].life != 18 {
		t.Fatalf("life %d want 18", sc.s.pl[0].life)
	}
}

func TestDamageDivisionDecision(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	og := sc.perm(v.HillGiant, 0) // 3/3
	b1 := sc.perm(v.GrizzlyBears, 1)
	b2 := sc.perm(v.GrizzlyBears, 1)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, uint8(og), 0xff)
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkBlock, uint8(b1), uint8(og))
	sc.do(t, AkBlock, uint8(b2), uint8(og))
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepDamage)
	d := sc.s.Step()
	// splits of 3 among 2 in lexicographic order: 0,3 1,2 2,1 3,0
	if d.Kind != DkChoose || d.Sub != ChDivide || d.Player != 0 || d.N != 4 || d.Options[0].Source() != uint8(og) {
		t.Fatalf("division decision %d n=%d %v", d.Kind, d.N, d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1]) // 1 to b1, 2 to b2
	sc.passUntil(t, kop.StepMain2)
	sc.s.Step()
	if !sc.onBF(b1) || sc.onBF(b2) || sc.onBF(og) {
		t.Fatalf("b1=%v b2=%v giant=%v", sc.onBF(b1), sc.onBF(b2), sc.onBF(og))
	}
}

func TestSplitAt(t *testing.T) {
	var out [3]uint8
	want := [][3]uint8{{0, 0, 2}, {0, 1, 1}, {0, 2, 0}, {1, 0, 1}, {1, 1, 0}, {2, 0, 0}}
	if n := splits(2, 3); n != len(want) {
		t.Fatalf("splits(2,3)=%d", n)
	}
	for i, w := range want {
		SplitAt(2, 3, i, out[:])
		if out != w {
			t.Fatalf("split %d = %v want %v", i, out, w)
		}
	}
}

func TestMenaceDoneWithheld(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	og := sc.perm(v.BoggartBrute, 0)
	b1 := sc.perm(v.GrizzlyBears, 1)
	b2 := sc.perm(v.GrizzlyBears, 1)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, uint8(og), 0xff)
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepBlockers)
	sc.do(t, AkBlock, uint8(b1), uint8(og))
	d := sc.s.Step()
	if d.N != 1 || d.Options[0] != MakeAction(AkBlock, uint8(b2), uint8(og), 0) {
		t.Fatalf("after a lone block on menace only the second block may follow: %v", d.Options[:d.N])
	}
}

// TestEndCombatFresh: CR 117.3a -- with no attackers, end of combat opens
// with the active player holding priority and no passes counted.
func TestEndCombatFresh(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.perm(v.HillGiant, 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkDone, 0xff, 0xff)
	for i := 0; i < 10; i++ {
		d := sc.s.Step()
		if sc.s.step == kop.StepEndCombat {
			if d.Player != sc.s.active || sc.s.passes != 0 {
				t.Fatalf("end of combat opened with prio %d passes %d", d.Player, sc.s.passes)
			}
			return
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatalf("never reached end of combat")
}

// TestCombatDamageTriggersBeforeEndOfCombat: CR 510.3 -- after combat
// damage the active player receives priority in the damage step, so a
// combat-damage trigger resolves there, before end of combat and its "at
// end of combat" trigger.
func TestCombatDamageTriggersBeforeEndOfCombat(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	drawer := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Drawer", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDamageDone, Flags: kop.TfSelf | kop.TfCombat | kop.TfToPlayer},
			Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}})
	eoc := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test End of Combat", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigPhase, Step: kop.StepEndCombat, Whose: kop.WhoseYou},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	a := sc.perm(drawer, 0)
	sc.perm(eoc, 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, uint8(a), 0xff)
	sc.do(t, AkDone, 0xff, 0xff)
	hand0, life0 := sc.s.pl[0].handN, sc.s.pl[0].life
	damageRound := false
	drewIn, gainedIn := kop.NumSteps, kop.NumSteps // the step each trigger had resolved by
	for i := 0; i < 40 && sc.s.step != kop.StepMain2; i++ {
		d := sc.s.Step()
		if sc.s.pl[0].handN > hand0 && drewIn == kop.NumSteps {
			drewIn = sc.s.step
		}
		if sc.s.pl[0].life > life0 && gainedIn == kop.NumSteps {
			gainedIn = sc.s.step
		}
		if d.Kind != DkPriority {
			sc.s.Apply(d.Options[0])
			continue
		}
		if sc.s.step == kop.StepDamage {
			damageRound = true
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if gainedIn != kop.StepEndCombat {
		t.Fatalf("the end-of-combat trigger resolved by step %d", gainedIn)
	}
	if !damageRound || drewIn != kop.StepDamage {
		t.Fatalf("priority round in the damage step %v, damage trigger resolved by step %d", damageRound, drewIn)
	}
}
