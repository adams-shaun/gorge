package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dmgSrcDefendScen is Mage Slayer's DamageSource$ form of the
// TriggeredDefender recipient: an Attacks trigger whose program is the
// OpAtkPWGuard the Defined$ emits (guardC) followed by OpDealDamageFrom's
// defending player mode (CS "defending": B.N 1 over RefDefendingPlayer, the
// trigger object as the source). Seat 1 controls a Test Walker the attacker
// attacks; a Test Doom Blade removes the attacker before the trigger
// resolves. The board runs on a pool table, so the attack declaration goes
// through fireAttacksGen, where the trigger's defSeat is stamped and the
// attacked object interned (combat.go, trig_atkobj.go).
func dmgSrcDefendScen(t *testing.T, guardC int16) (sc *scen, giant, blade, walker Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	fC := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	sentry, attacker, bladeID, walkerID := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2), CardID(len(tbl.Cards)+3)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Sentry", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigAttacks, Filter: fC},
				Prog: []kop.Instr{
					{Op: kop.OpAtkPWGuard, C: guardC},
					{Op: kop.OpDealDamageFrom, Obj: kop.RefTriggerObj, A: kop.Const(2), B: kop.Const(1), C: int16(kop.RefDefendingPlayer)},
				}}}},
		kop.CardDef{Name: "Test Attacker", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Doom Blade", Cost: kop.ManaCost{Generic: 1}, Types: kop.TInstant, Timing: kop.TimingInstant,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fC}}, NTargets: 1,
				Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}},
		kop.CardDef{Name: "Test Walker", Types: kop.TPlaneswalker, Loyalty: 5})
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Plains"], 0)
	sc.perm(sentry, 0)
	giant = sc.perm(attacker, 0)
	walker = sc.perm(walkerID, 1)
	blade = sc.hand(bladeID, 0)
	return sc, giant, blade, walker
}

// TestMageSlayerDamageSourceHitsTheAttackedObject: Mage Slayer's carrier
// attacks a planeswalker and its DealDamageFrom resolves with the attacker
// alive and, in the gone arm, after the attacker left (CR 603.3c, 608.2h).
// gorge damages the DefenderObj the Defined$'s Valid
// Planeswalker.TriggeredDefender arm names; the kernel's defending mode must
// neither stall (StallDefenderPW, the plain guard the lowering used to add)
// nor damage the defending player there, and must spare the walker when the
// attack was on the player. Without C AtkPWHit the guard keeps stalling, the
// object being no recipient.
func TestMageSlayerDamageSourceHitsTheAttackedObject(t *testing.T) {
	for _, tc := range []struct {
		name     string
		guardC   int16
		gone     bool
		onWalker bool
		stalls   bool
	}{
		{"alive, walker", kop.AtkPWHit, false, true, false},
		{"gone, walker", kop.AtkPWHit, true, true, false},
		{"gone, walker, plain guard", 0, true, true, true},
		{"alive, player", kop.AtkPWHit, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, giant, blade, walker := dmgSrcDefendScen(t, tc.guardC)
			// Precondition: the atkObj table is kept exactly where the
			// program asks for it (trig_atkobj.go useAtkPWHit).
			if got := len(sc.s.atkObjCold()) != 0; got != (tc.guardC&kop.AtkPWHit != 0) {
				t.Fatalf("atkObj table kept = %v, want %v", got, tc.guardC&kop.AtkPWHit != 0)
			}
			sc.passUntil(t, kop.StepAttackers)
			if tc.onWalker {
				sc.do(t, AkAttack, TCode(giant), TCode(walker))
			} else {
				sc.do(t, AkAttack, TCode(giant), PlayerCode(1))
			}
			sc.do(t, AkDone, scAny, scAny)
			if tc.gone {
				sc.do(t, AkCast, TCode(blade), scAny)
				sc.do(t, AkTarget, scAny, TCode(giant))
			}
			if a := sc.s.perms[sc.slot(giant)].atk; !tc.gone && tc.onWalker && a != TCode(walker) {
				t.Fatalf("attacker atk %d, want the walker %d", a, walker)
			}
			// resolve the stack (the attack trigger, the removal) within the step
			for i := 0; i < 20 && (sc.s.sp > 0 || sc.s.npend > 0) && !sc.s.Stalled(); i++ {
				d := sc.s.Step()
				if d.Kind != DkPriority {
					break
				}
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			}
			if r := sc.s.StallReason(); (r == StallDefenderPW) != tc.stalls || r != 0 && !tc.stalls {
				t.Fatalf("stall %d (%s), want stall=%v", r, StallName(r), tc.stalls)
			}
			if tc.gone && sc.s.objs[giant].zone == kop.ZoneBattlefield {
				t.Fatal("the attacker is still on the battlefield: the case is not the gone arm")
			}
			if tc.onWalker {
				if hit := sc.s.g.dmgTurn.has(walker); hit == tc.stalls {
					t.Fatalf("walker damaged = %v with stall=%v", hit, tc.stalls)
				}
				if !tc.stalls && sc.s.pl[1].life != 20 {
					t.Fatalf("the defending player lost life (%d): the attack was on the walker", sc.s.pl[1].life)
				}
			} else {
				if hit := sc.s.g.dmgTurn.has(walker); hit {
					t.Fatal("the walker took the player attack's damage")
				}
				if d := 20 - sc.s.pl[1].life; d != 2 {
					t.Fatalf("the defending player took %d damage, want 2", d)
				}
			}
		})
	}
}
