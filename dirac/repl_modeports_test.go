package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The T17c replacement-mode ports on hand tables (the lockstep pools over
// the lowered cards cover the card behaviour end to end; these drive the
// kernel ops directly and flip each op's own mutation hook, so every
// assertion is shown to fail with the mode out of the table).

func TestReplModePortsPhaseSkip(t *testing.T) {
	t.Run("skip applies", func(t *testing.T) {
		tb := newSPTable()
		upkeep := tb.host("Test Eon", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep)})
		draw := tb.host("Test Draw", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepDraw)})
		sc := tb.scen(t)
		sc.perm(upkeep, 0)
		sc.perm(draw, 0)
		if sc.k.gen == nil || sc.k.gen.anyRepl2&rb2BeginPhase == 0 {
			t.Fatal("precondition: the phase-skip table did not set the kernel's gate word")
		}
		if got := sc.s.phaseSkipNext(sc.k, kop.StepUntap); got != kop.StepUntap {
			t.Fatalf("phaseSkipNext(untap) = %d, want untap (no line skips the untap step)", got)
		}
		// the upkeep skip rewrites the proposed step to the draw step, which
		// the draw skip rewrites again (gorge re-matches each landing
		// StepChange; phaseSkipLanded is Main1 on turn 2)
		if got := sc.s.phaseSkipNext(sc.k, kop.StepUpkeep); got != kop.StepMain1 {
			t.Fatalf("phaseSkipNext(upkeep) = %d, want main1 (the chained skip)", got)
		}
		if got := sc.s.phaseSkipNext(sc.k, kop.StepDraw); got != kop.StepMain1 {
			t.Fatalf("phaseSkipNext(draw) = %d, want main1", got)
		}
	})
	t.Run("ValidPlayer$ You scopes to the controller's turns", func(t *testing.T) {
		tb := newSPTable()
		you := tb.host("Test You", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep), C: kop.ReplPhYou})
		sc := tb.scen(t)
		sc.perm(you, 1)
		if got := sc.s.phaseSkipNext(sc.k, kop.StepUpkeep); got != kop.StepUpkeep {
			t.Fatalf("phaseSkipNext(upkeep) = %d, want upkeep (the active seat does not control the line)", got)
		}
	})
	t.Run("the turn walk skips the upkeep and the draw step", func(t *testing.T) {
		tb := newSPTable()
		upkeep := tb.host("Test Eon", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep)})
		draw := tb.host("Test Draw", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepDraw)})
		sc := tb.scen(t)
		sc.perm(upkeep, 0)
		sc.perm(draw, 0)
		sc.walkTo(t, 3, kop.StepMain1) // the chain lands in main1, no upkeep, no draw
	})
	t.Run("mutation: the skip dropped", func(t *testing.T) {
		defer func(p bool) { DebugBeginPhaseOff = p }(DebugBeginPhaseOff)
		DebugBeginPhaseOff = true
		tb := newSPTable()
		upkeep := tb.host("Test Eon", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep)})
		draw := tb.host("Test Draw", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepDraw)})
		sc := tb.scen(t)
		sc.perm(upkeep, 0)
		sc.perm(draw, 0)
		if got := sc.s.phaseSkipNext(sc.k, kop.StepUpkeep); got != kop.StepUpkeep {
			t.Fatalf("phaseSkipNext(upkeep) = %d, want upkeep (the skip is off)", got)
		}
		sc.walkTo(t, 3, kop.StepUpkeep) // the phases run
	})
}

// walkTo passes priority from a scen's start until turn and step are live.
func (sc *scen) walkTo(t *testing.T, turn int, st kop.Step) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if sc.s.turn == uint16(turn) && sc.s.step == st {
			return
		}
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	t.Fatalf("walk never reached turn %d step %d (at turn %d step %d)", turn, st, sc.s.turn, sc.s.step)
}

func TestReplModePortsLifeFloor(t *testing.T) {
	for _, off := range []bool{false, true} {
		name := "the floor clamps a damage loss"
		if off {
			name = "mutation: the floor dropped"
		}
		t.Run(name, func(t *testing.T) {
			defer func(p bool) { DebugLifeFloorOff = p }(DebugLifeFloorOff)
			DebugLifeFloorOff = off
			tb := newSPTable()
			ali := tb.host("Test Ali", kop.Repl{Kind: kop.ReplLifeReduced, A: 1})
			sc := tb.scen(t)
			sc.perm(ali, 0)
			if sc.k.gen == nil || sc.k.gen.anyRepl2&rb2LifeFloor == 0 {
				t.Fatal("precondition: the life-floor table did not set the kernel's gate word")
			}
			floor, plain := int16(1), int16(-2)
			if off {
				floor, plain = -2, -2
			}
			// a damage-caused loss from 3 to below 1 lands on the floor
			sc.s.pl[0].life = 3
			sc.s.loseLifeDmg(sc.k, 0, 5)
			if got := sc.s.pl[0].life; got != floor {
				t.Fatalf("life after a 5 damage loss from 3 = %d, want %d", got, floor)
			}
			// a life cost is paid, never floored (CR 119.4)
			sc.s.pl[0].life = 3
			sc.s.loseLife(sc.k, 0, 5)
			if got := sc.s.pl[0].life; got != plain {
				t.Fatalf("life after a 5 life cost from 3 = %d, want %d", got, plain)
			}
			// a loss that never reaches the floor is untouched
			sc.s.pl[0].life = 10
			if got := sc.s.lifeFloorTake(sc.k, 0, 5); got != 5 {
				t.Fatalf("lifeFloorTake from 10 by 5 = %d, want 5", got)
			}
			// the floored amount is what the loss commits
			sc.s.pl[0].life = 3
			if got := sc.s.lifeFloorTake(sc.k, 0, 5); got != 2 && !off {
				t.Fatalf("lifeFloorTake from 3 by 5 = %d, want 2", got)
			}
			// the opponent's floor line does not seat 0's loss
			tb2 := newSPTable()
			theirs := tb2.host("Test Ali", kop.Repl{Kind: kop.ReplLifeReduced, A: 1})
			sc2 := tb2.scen(t)
			sc2.perm(theirs, 1)
			sc2.s.pl[0].life = 3
			sc2.s.loseLifeDmg(sc2.k, 0, 5)
			if got := sc2.s.pl[0].life; got != -2 {
				t.Fatalf("life after a 5 damage loss with the floor line across the table = %d, want -2", got)
			}
		})
	}
}

func TestReplModePortsDamageMinus(t *testing.T) {
	for _, off := range []bool{false, true} {
		name := "the minus rewrites the event's amount"
		if off {
			name = "mutation: the minus dropped"
		}
		t.Run(name, func(t *testing.T) {
			defer func(p bool) { DebugDmgMinusOff = p }(DebugDmgMinusOff)
			DebugDmgMinusOff = off
			tb := newSPTable()
			f := tb.filter(kop.Filter{Types: kop.TCreature})
			uni := tb.host("Test Unicorn", kop.Repl{Kind: kop.ReplDamageMinus, A: 1, C: kop.ReplDmgNonCombat, Filter: f})
			sc := tb.scen(t)
			sc.perm(uni, 0)
			if sc.k.gen == nil || sc.k.gen.anyRepl2&rb2DmgMinus == 0 {
				t.Fatal("precondition: the damage-minus table did not set the kernel's gate word")
			}
			bear := sc.perm(v.GrizzlyBears, 0)
			giant := sc.perm(v.HillGiant, 1)
			wantNoncombat, wantSingle := 2, 0
			if off {
				wantNoncombat, wantSingle = 3, 1
			}
			if got := spDeal(sc, 1, giant, TCode(bear), 3); got != wantNoncombat {
				t.Fatalf("3 noncombat damage to a creature = %d, want %d", got, wantNoncombat)
			}
			if got := spDeal(sc, 1, giant, TCode(bear), 1); got != wantSingle {
				t.Fatalf("1 noncombat damage to a creature = %d, want %d (floored at none)", got, wantSingle)
			}
			if got := sc.s.replDamage(sc.k, 1, sc.slot(giant), v.HillGiant, TCode(bear), true, 3); got != 3 {
				t.Fatalf("3 combat damage to a creature = %d, want 3 (IsCombat$ False)", got)
			}
			if got := spDeal(sc, 1, giant, TCode(PlayerTarget1), 3); got != 3 {
				t.Fatalf("3 damage to a player = %d, want 3 (the filter names creatures only)", got)
			}
		})
	}
}

func TestReplModePortsDamageSet(t *testing.T) {
	for _, off := range []bool{false, true} {
		name := "the set rewrites damage at the gate and up"
		if off {
			name = "mutation: the set dropped"
		}
		t.Run(name, func(t *testing.T) {
			defer func(p bool) { DebugDmgSetOff = p }(DebugDmgSetOff)
			DebugDmgSetOff = off
			tb := newSPTable()
			f := tb.filter(kop.Filter{Types: kop.TCreature})
			pres := tb.host("Test Presence", kop.Repl{Kind: kop.ReplDamageSet, A: 3, C: int16(kop.ReplDmgSetGE(4)) | kop.ReplDmgPerm, Filter: f})
			sc := tb.scen(t)
			sc.perm(pres, 0)
			if sc.k.gen == nil || sc.k.gen.anyRepl2&rb2DmgSet == 0 {
				t.Fatal("precondition: the damage-set table did not set the kernel's gate word")
			}
			bear := sc.perm(v.GrizzlyBears, 0)
			giant := sc.perm(v.HillGiant, 1)
			wantOver, wantGate := 3, 3
			if off {
				wantOver, wantGate = 5, 4
			}
			if got := spDeal(sc, 1, giant, TCode(bear), 5); got != wantOver {
				t.Fatalf("5 damage to a creature = %d, want %d", got, wantOver)
			}
			if got := spDeal(sc, 1, giant, TCode(bear), 4); got != wantGate {
				t.Fatalf("4 damage to a creature = %d, want %d (at the gate)", got, wantGate)
			}
			if got := spDeal(sc, 1, giant, TCode(bear), 3); got != 3 {
				t.Fatalf("3 damage to a creature = %d, want 3 (under the gate is untouched)", got)
			}
			if got := spDeal(sc, 1, giant, TCode(PlayerTarget1), 5); got != 5 {
				t.Fatalf("5 damage to a player = %d, want 5 (the filter names creatures only)", got)
			}
		})
	}
}

func TestReplModePortsUncounter(t *testing.T) {
	t.Run("one line: the counter is stopped", func(t *testing.T) {
		tb := newSPTable()
		f := tb.filter(kop.Filter{Types: kop.TCreature})
		host := tb.host("Test Sliver", kop.Repl{Kind: kop.ReplSpellsCantBeCountered, C: kop.ReplUncAnyCtrl, Filter: f})
		sc := tb.scen(t)
		sc.perm(host, 1) // the line names every controller's spells
		if !sc.k.uncAnyCtrl {
			t.Fatal("precondition: the widened scan is off")
		}
		sc.castCounter(t)
		if got := sc.s.objs[bearsObj].zone; got != kop.ZoneBattlefield {
			t.Fatalf("bears zone %d, want battlefield (the widened line stopped the counter)", got)
		}
		if got := sc.s.objs[cancelObj].zone; got != kop.ZoneGraveyard {
			t.Fatalf("cancel zone %d, want graveyard", got)
		}
	})
	t.Run("two lines: the CR 616.1 ask", func(t *testing.T) {
		tb := newSPTable()
		f := tb.filter(kop.Filter{Types: kop.TCreature})
		h0 := tb.host("Test Sliver 0", kop.Repl{Kind: kop.ReplSpellsCantBeCountered, C: kop.ReplUncAnyCtrl, Filter: f})
		h1 := tb.host("Test Sliver 1", kop.Repl{Kind: kop.ReplSpellsCantBeCountered, C: kop.ReplUncAnyCtrl, Filter: f})
		sc := tb.scen(t)
		h0obj = sc.perm(h0, 0)
		sc.perm(h1, 1)
		sc.castCounter(t)
		// gorge poses the order ask to the countered spell's controller; the
		// candidates are the seats' battlefields in entry order (uncOrderPose)
		sc.do(t, AkChoose, TCode(h0obj), scAny)
		sc.drainStack(t)
		if got := sc.s.objs[bearsObj].zone; got != kop.ZoneBattlefield {
			t.Fatalf("bears zone %d, want battlefield (every candidate only stops the event)", got)
		}
	})
	t.Run("mutation: the scan dropped", func(t *testing.T) {
		defer func(p bool) { DebugUncounterOff = p }(DebugUncounterOff)
		DebugUncounterOff = true
		tb := newSPTable()
		f := tb.filter(kop.Filter{Types: kop.TCreature})
		host := tb.host("Test Sliver", kop.Repl{Kind: kop.ReplSpellsCantBeCountered, C: kop.ReplUncAnyCtrl, Filter: f})
		sc := tb.scen(t)
		sc.perm(host, 1)
		sc.castCounter(t)
		if got := sc.s.objs[bearsObj].zone; got != kop.ZoneGraveyard {
			t.Fatalf("bears zone %d, want graveyard (the scan is off, the counter went through)", got)
		}
	})
}

// castCounter is the spell-cast-and-counter exchange a Test Sliver table's
// scenarios share: seat 0 casts Grizzly Bears with the two forests it needs
// and seat 1 counters it with Cancel; the objects are in bearsObj/cancelObj
// and h0obj (the seat-0 host, for the two-line ask).
var (
	bearsObj, cancelObj, h0obj Obj
)

func (sc *scen) castCounter(t *testing.T) {
	t.Helper()
	sc.lands(0, v.Forest, 2)
	sc.lands(1, v.Island, 3)
	bears := sc.hand(v.GrizzlyBears, 0)
	cancel := sc.hand(v.Cancel, 1)
	bearsObj, cancelObj = bears, cancel
	sc.do(t, AkCast, TCode(bears), scAny)
	sc.do(t, AkPass, scAny, scAny) // seat 0 passes with bears on the stack
	sc.do(t, AkCast, TCode(cancel), scAny)
	sc.do(t, AkTarget, scAny, TCode(bears))
	for i := 0; i < 20; i++ {
		if sc.s.g.ro.ac&uncOrdTag == uncOrdTag || sc.s.sp == 0 {
			break
		}
		d := sc.s.Step()
		if d.Kind != DkPriority {
			break // the order ask (or another decision) is up
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
}

// drainStack passes priority until the stack is empty.
func (sc *scen) drainStack(t *testing.T) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if sc.s.sp == 0 {
			return
		}
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision %d while the stack drains", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("stack never drained")
}
