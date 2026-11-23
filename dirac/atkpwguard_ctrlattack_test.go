package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// atkPWScen builds a duel with the Mage Slayer shape of the
// TriggeredDefender.Opponent guard: seat 1's Test Sentry triggers on any
// creature's attack and, behind OpAtkPWGuard, damages the defending player.
// The trigger's controller (seat 1) is not the attacker's controller, so an
// attack on seat 1 is an attack on the trigger's own controller.
//
// The board runs on a pool table, so the attack declaration goes through
// fireAttacksGen, where the bound attack is stamped on the pending trigger
// (combat.go); a hand-written vanilla copy would fire the trigger off
// fireObj and never reach it.
func atkPWScen(t *testing.T) (sc *scen, giant, blade Obj) {
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
	sentry, attacker, bladeID := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Sentry", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered,
				Trig: kop.Trigger{Event: kop.TrigAttacks, Filter: fC},
				Prog: []kop.Instr{
					{Op: kop.OpAtkPWGuard},
					{Op: kop.OpDealDamage, Obj: kop.RefDefendingPlayer, A: kop.Const(3)},
				}}}},
		kop.CardDef{Name: "Test Attacker", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Doom Blade", Cost: kop.ManaCost{Generic: 1}, Types: kop.TInstant, Timing: kop.TimingInstant,
			Abilities: []kop.Ability{{Kind: kop.AbSpell,
				Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fC}}, NTargets: 1,
				Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Plains"], 0)
	sc.perm(sentry, 1)
	giant = sc.perm(attacker, 0)
	blade = sc.hand(bladeID, 0)
	return sc, giant, blade
}

// atkPWPass drives the game with passes until it ends, so the callers assert
// on the stall reason.
func atkPWPass(sc *scen) {
	for i := 0; i < 60; i++ {
		d := sc.s.Step()
		if sc.s.Stalled() || d.Kind == DkNone || sc.s.over {
			return
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
}

func TestAtkPWGuardCtrlAttackGoneStalls(t *testing.T) {
	sc, giant, blade := atkPWScen(t)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(giant), scAny)
	sc.do(t, AkDone, scAny, scAny)
	// the attacker leaves before the trigger resolves
	sc.do(t, AkCast, TCode(blade), scAny)
	sc.do(t, AkTarget, scAny, TCode(giant))
	atkPWPass(sc)
	if r := sc.s.StallReason(); r != StallDefenderPW {
		t.Fatalf("stall %d (%s) with seat 0 at %d life; want the StallDefenderPW stall (the attack was on the trigger's controller)", r, StallName(r), sc.s.pl[0].life)
	}
}

func TestAtkPWGuardCtrlAttackAliveStalls(t *testing.T) {
	sc, giant, _ := atkPWScen(t)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(giant), scAny)
	sc.do(t, AkDone, scAny, scAny)
	atkPWPass(sc)
	if r := sc.s.StallReason(); r != StallDefenderPW {
		t.Fatalf("stall %d (%s); the guard with the attacker alive must stall", r, StallName(r))
	}
}
