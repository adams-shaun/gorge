package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// upkeepOvTable returns a scenario table carrying one until-your-next-upkeep
// keyword grant record. The card's program is what the table scan reads to
// set genPrep.uses (useUpkeepOv); the tests apply the record themselves.
func upkeepOvTable(t *testing.T) (kop.Table, int16) {
	t.Helper()
	tbl := *vanilla.Table
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...), kop.CardDef{
		Name:  "Upkeep Grant Test",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpOverride, A: kop.Const(kop.DurUntilYourNextUpkeep),
		}}}},
	})
	tbl.Overrides = append(append([]kop.Override(nil), tbl.Overrides...), kop.Override{
		AddKw:       kop.KwFlying,
		UntilUpkeep: true,
	})
	return tbl, int16(len(tbl.Overrides) - 1)
}

// applyUpkeepGrant applies the table's until-upkeep record to victim for the
// resolving controller ctrl, the way resolution does.
func applyUpkeepGrant(t *testing.T, sc *scen, victim Obj, ov int16, ctrl uint8) {
	t.Helper()
	s := sc.s
	s.res.e.ctrl, s.res.e.obj = ctrl, victim
	s.res.e.card, s.res.e.srcVer = vanilla.SavannahLions, s.objs[victim].ver
	s.exec(sc.k, &kop.Instr{Op: kop.OpOverride, Obj: kop.RefSelf, C: ov, A: kop.Const(kop.DurUntilYourNextUpkeep)})
}

// TestPumpUpkeepDropsOnEnteringUpkeep: a Duration$ UntilYourNextUpkeep
// keyword grant (Erhnam Djinn) is live through its controller's untap step
// and ends as their upkeep begins (gorge dropUntilYourNextUpkeep, called from
// setStep before the StepChange emit).
func TestPumpUpkeepDropsOnEnteringUpkeep(t *testing.T) {
	tbl, ov := upkeepOvTable(t)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.gen.uses.has(useUpkeepOv) {
		t.Fatal("useUpkeepOv off: the table scan did not see DurUntilYourNextUpkeep")
	}
	victim := sc.perm(vanilla.SavannahLions, 0)
	sl := sc.slot(victim)
	applyUpkeepGrant(t, sc, victim, ov, 0)
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("precondition: the grant is not live after resolution")
	}
	s.active, s.step = 0, kop.StepUntap
	s.beginStep(k)
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("the grant ended during the controller's untap step; it must last until the upkeep")
	}
	s.beginStep(k) // the untap case leaves s.step at StepUpkeep
	if s.step != kop.StepUpkeep {
		t.Fatalf("step %d after the untap step, want StepUpkeep", s.step)
	}
	if s.kw(k, sl)&kop.KwFlying != 0 {
		t.Fatal("the grant survived entering its controller's upkeep")
	}
}

// TestPumpUpkeepFollowsResolvingController: the boundary is the controller of
// the RESOLVING effect, not the target's controller (gorge dropUntilYourNextUpkeep
// compares ce.Controller with the active player; Erhnam Djinn's target is an
// opponent's creature). The grant survives the target controller's upkeep and
// ends at the resolving controller's.
func TestPumpUpkeepFollowsResolvingController(t *testing.T) {
	tbl, ov := upkeepOvTable(t)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	victim := sc.perm(vanilla.SavannahLions, 0) // the target is seat 0's
	sl := sc.slot(victim)
	applyUpkeepGrant(t, sc, victim, ov, 1) // the resolving controller is seat 1
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("precondition: the grant is not live after resolution")
	}
	s.active, s.step = 0, kop.StepUpkeep
	s.beginStep(k)
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("the target controller's upkeep ended the grant; it keys on the resolving controller")
	}
	s.active = 1
	s.beginStep(k)
	if s.kw(k, sl)&kop.KwFlying != 0 {
		t.Fatal("the resolving controller's upkeep did not end the grant")
	}
}
