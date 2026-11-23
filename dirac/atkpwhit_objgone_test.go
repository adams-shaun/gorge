package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// atkPWHitScen is atkPWScen's shape for the Raid Bombardment / Fathom Fleet
// Swordjack guard: seat 0's Test Sentry triggers on any creature's attack and,
// behind OpAtkPWGuard C=guardC, deals 3 to the defending player or, with
// AtkPWHit, the planeswalker the attacker attacks (DmgAtkedObj). Seat 1
// controls a Test Walker the attacker attacks.
func atkPWHitScen(t *testing.T, guardC int16) (sc *scen, giant, blade, walker Obj) {
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
					{Op: kop.OpDealDamage, Obj: kop.RefDefendingPlayer, A: kop.Const(3), B: kop.Const(kop.DmgAtkedObj)},
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

// TestAtkPWHitGoneAttackerDamagesTheBoundWalker: the attacker attacks a
// planeswalker and leaves before its attack trigger resolves. gorge damages
// the DefenderObj bound as the trigger was queued (CR 603.3c, 608.2h); the
// kernel keeps that object in the atkObj table through the trigger's defSeat
// and must neither stall (StallDefenderPW) nor spare the walker. Without
// C AtkPWHit the guard keeps stalling, the object being no recipient.
func TestAtkPWHitGoneAttackerDamagesTheBoundWalker(t *testing.T) {
	for _, tc := range []struct {
		name   string
		guardC int16
		gone   bool
		stalls bool
	}{
		{"alive, AtkPWHit", kop.AtkPWHit, false, false},
		{"gone, AtkPWHit", kop.AtkPWHit, true, false},
		{"gone, plain guard", 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, giant, blade, walker := atkPWHitScen(t, tc.guardC)
			// Precondition: the table is kept exactly where the program asks.
			if got := len(sc.s.atkObjCold()) != 0; got != (tc.guardC&kop.AtkPWHit != 0) {
				t.Fatalf("atkObj table kept = %v, want %v", got, tc.guardC&kop.AtkPWHit != 0)
			}
			sc.passUntil(t, kop.StepAttackers)
			sc.do(t, AkAttack, TCode(giant), TCode(walker))
			sc.do(t, AkDone, scAny, scAny)
			if tc.gone {
				sc.do(t, AkCast, TCode(blade), scAny)
				sc.do(t, AkTarget, scAny, TCode(giant))
			}
			if a := sc.s.perms[sc.slot(giant)].atk; tc.gone == false && a != TCode(walker) {
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
			if hit := sc.s.g.dmgTurn.has(walker); hit == tc.stalls {
				t.Fatalf("walker damaged = %v with stall=%v", hit, tc.stalls)
			}
		})
	}
}
