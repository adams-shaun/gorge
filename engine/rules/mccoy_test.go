package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMcCoyGorge: the corpus Dr. McCoy, Surly Surgeon replaces destruction of
// another creature its controller controls that has a +1/+1 counter on it:
// remove all +1/+1 counters and heal all damage instead (CR 614.1). It covers
// each destruction source and the edges the replacement does not catch:
// indestructible (never destroyed), McCoy herself (not "other"), and a
// creature whose toughness is 0 after the counters leave (CR 704.5f, not a
// destruction, so it still dies).
func TestMcCoyGorge(t *testing.T) {
	const indestructibleSrc = "Name:Line Rock\nTypes:Creature Wall\nPT:0/3\nK:Indestructible\nOracle:x\n"
	const zeroToughnessSrc = "Name:Frail Wisp\nTypes:Creature Spirit\nPT:0/0\nOracle:x\n"
	reg := testutil.CorpusRegistry(t)
	mccoy := mustCorpusCard(t, reg, "Dr. McCoy, Surly Surgeon")
	murder := mustCorpusCard(t, reg, "Murder")

	t.Run("destroy spell", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 7, mccoy, murder)
		spell := moveSeededCard(t, e, 0, murder, state.ZHand)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
		e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
		addMana(t, e, 0, "BBB")
		castDestroyAt(t, e, spell, bear)
		assertSpared(t, e, bear)
		replayCheck(t, e, cfg)
	})

	t.Run("lethal damage", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 11, mccoy)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
		e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 4})
		e.checkStateBased()
		assertSpared(t, e, bear)
		replayCheck(t, e, cfg)
	})

	t.Run("deathtouch", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 13, mccoy)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
		e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
		e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "Deathtouched", Amount: 1})
		e.checkStateBased()
		assertSpared(t, e, bear)
		if got := e.G.Obj(bear).Counter("Deathtouched"); got != 0 {
			t.Fatalf("%d deathtouch marks left; want the replacement to clear them", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("indestructible", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 17, mccoy, murder)
		spell := moveSeededCard(t, e, 0, murder, state.ZHand)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		rock := putToken(t, e, 0, indestructibleSrc, state.ZBattlefield)
		placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: rock, Counter: "P1P1", Amount: 1})
		e.emit(events.Event{Kind: events.Damage, Obj: rock, Amount: 4})
		e.checkStateBased()
		addMana(t, e, 0, "BBB")
		castDestroyAt(t, e, spell, rock)
		o := e.G.Obj(rock)
		if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 || o.Damage != 4 {
			t.Fatalf("indestructible rock zone %s, %d counters, %d damage; want untouched by the replacement",
				o.Zone, o.Counter("P1P1"), o.Damage)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("not other", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 19, mccoy, murder)
		spell := moveSeededCard(t, e, 0, murder, state.ZHand)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		medic := placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.CounterChange, Obj: medic, Counter: "P1P1", Amount: 1})
		addMana(t, e, 0, "BBB")
		castDestroyAt(t, e, spell, medic)
		if z := e.G.Obj(medic).Zone; z != state.ZGraveyard {
			t.Fatalf("McCoy zone %s; the replacement names another creature, so McCoy must die", z)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("zero toughness after counters leave", func(t *testing.T) {
		e, cfg := tokenReplGame(t, 23, mccoy)
		plain := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
		wisp := putToken(t, e, 0, zeroToughnessSrc, state.ZBattlefield)
		e.emit(events.Event{Kind: events.CounterChange, Obj: wisp, Counter: "P1P1", Amount: 1})
		placeMcCoy(t, e, mccoy, plain)
		e.emit(events.Event{Kind: events.Damage, Obj: wisp, Amount: 1})
		// The replacement runs inside the destruction attempt, which the SBA
		// attempt memory records; the 0-toughness sweep sees the new board on
		// the next check (sba.go's tried.objs residue note).
		e.checkStateBased()
		e.checkStateBased()
		if z := e.G.Obj(wisp).Zone; z != state.ZCeased {
			t.Fatalf("0/0 wisp zone %s; with its counter gone the token dies by CR 704.5f", z)
		}
		replayCheck(t, e, cfg)
	})
}

// placeMcCoy moves McCoy to the battlefield and answers its enters trigger,
// which asks for another creature its controller controls.
func placeMcCoy(t *testing.T, e *Engine, mccoy *cards.Card, target state.ObjID) state.ObjID {
	t.Helper()
	medic := moveSeededCard(t, e, 0, mccoy, state.ZBattlefield)
	addMana(t, e, 0, "")
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		idx := indexOfObjOption(d, target)
		if idx < 0 {
			t.Fatalf("McCoy's enters trigger does not offer %d: %+v", target, d.Options)
		}
		submitChoices(t, e, idx)
	}
	passUntilStackEmpty(t, e, 20)
	return medic
}

func assertSpared(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 || o.Damage != 0 {
		t.Fatalf("creature zone %s, %d counters, %d damage; want spared with none of either",
			o.Zone, o.Counter("P1P1"), o.Damage)
	}
}
