package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.14 (Fight), cr-g18. 701.14a: each creature deals damage equal to
// its power to the other -- the two hits are dealt simultaneously and each
// hit's source is the creature. 701.14b: a creature no longer on the
// battlefield or no longer a creature means neither deals damage. 701.14c:
// a creature that fights itself deals damage to itself equal to twice its
// power.

// cr70114DamageSince returns the Damage events logged after mark.
func cr70114DamageSince(e *Engine, mark int) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Damage {
			out = append(out, ev)
		}
	}
	return out
}

// cr70114TargetFightSA is the targeted fight shape: the resolving source
// (Defined$ Self) fights the one chosen target.
func cr70114TargetFightSA() *cards.SA {
	return &cards.SA{Kind: "DB", API: "Fight", Params: map[string]string{
		"Defined": "Self", "ValidTgts": "Creature",
		"TargetMin": "0", "TargetMax": "1", "TgtPrompt": "x"}}
}

// cr70114AttackTable is a two-seat table parked at seat 0's declare-attackers
// step, for the attack-triggered fight carriers.
func cr70114AttackTable(t *testing.T) *Engine {
	t.Helper()
	e := New(Config{Seed: 70114, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e
}

// cr70114Drain answers every ask the attack-trigger fights pose (the
// optional-trigger and Dig "put it onto the battlefield" yes/no, both offered
// "yes" first) and passes priority until the stack is empty.
func cr70114Drain(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		switch d.Kind {
		case decision.KTriggerOptional, decision.KChoose:
			if len(d.Options) == 0 || d.Options[0].Kind != "yes" {
				t.Fatalf("unexpected ask %+v while draining", d)
			}
			submitChoices(t, e, d.Options[0].Index)
		case decision.KPriority:
			if len(e.G.Stack) == 0 {
				return
			}
			passPriorityOnce(t, e)
		default:
			t.Fatalf("unexpected decision %+v while draining", d)
		}
	}
	t.Fatal("the stack did not drain")
}

// TestCR701_14aHansErikssonFightsTheRevealedCreature: Hans Eriksson (1/4)
// attacks, reveals a 2/2 on top of the library and puts it onto the
// battlefield; the reflexive `DB$ Fight | Defined$ Remembered & Self` (no
// ValidTgts$) has the 2/2 and Hans fight each other: Hans takes 2, the 2/2
// takes 1.
func TestCR701_14aHansErikssonFightsTheRevealedCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := cr70114AttackTable(t)
	hans := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Hans Eriksson"))
	e.G.Obj(hans).SummonSick = false
	ox := putTopOfLibrary(t, e, card(t, "Name:Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:2/2\nOracle:x\n"), 0)
	mark := len(e.L.Events)
	e.askAttackers()
	submitAttackersOnly(t, e, hans)
	cr70114Drain(t, e)
	if o := e.G.Obj(ox); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the revealed Ox should be on the battlefield, got %+v", o)
	}
	var toHans, toOx []events.Event
	for _, ev := range cr70114DamageSince(e, mark) {
		switch ev.Obj {
		case hans:
			toHans = append(toHans, ev)
		case ox:
			toOx = append(toOx, ev)
		}
	}
	if len(toHans) != 1 || toHans[0].Amount != 2 || len(toOx) != 1 || toOx[0].Amount != 1 {
		t.Fatalf("Hans/Ox fight damage = Hans %+v, Ox %+v; want Hans 2 and Ox 1: CR 701.14a, the two Defined$ creatures fight each other", toHans, toOx)
	}
}

// TestFightTargetlessDefinedMisresolutionIsLoud: Grothama's granted trigger
// is `DB$ Fight | Defined$ TriggeredAttackerLKICopy & OriginalHost`. This
// build resolves OriginalHost of a static-granted trigger to the attacker
// itself, so both members name the same creature. The fight must not happen
// (no self-fight from a mis-resolved selector) and must say so in the log --
// never a silent no-op that counts the card as supported.
func TestFightTargetlessDefinedMisresolutionIsLoud(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := cr70114AttackTable(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grothama, All-Devouring"))
	giant := onBoardReady(t, e, 0, "Name:Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:3/3\nOracle:x\n")
	mark := len(e.L.Events)
	e.askAttackers()
	submitAttackersOnly(t, e, giant)
	cr70114Drain(t, e)
	if got := cr70114DamageSince(e, mark); len(got) != 0 {
		t.Fatalf("Grothama fight dealt %+v, want nothing (both Defined$ members are the attacker)", got)
	}
	loud := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && strings.HasPrefix(ev.Text, "Fight Defined$ named the same creature twice") {
			loud = true
		}
	}
	if !loud {
		t.Fatal("the unresolvable targetless Fight emitted no Note: a fight that cannot happen must not be silent")
	}
}

// TestFightArenaDefinedTargetedGuard is a guard (passes today): Arena's
// `DB$ Fight | Defined$ Targeted` (no ValidTgts$ of its own) fights the two
// creatures its Tap chain targeted -- seat 0's Craw Wurm (6) and the Hill
// Giant (3) seat 1 chose, one hit each way.
func TestFightArenaDefinedTargetedGuard(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70115,
		map[string]state.Zone{"Arena": state.ZBattlefield, "Craw Wurm": state.ZBattlefield},
		map[string]state.Zone{"Hill Giant": state.ZBattlefield})
	arena, wurm, giant := mine["Arena"], mine["Craw Wurm"], theirs["Hill Giant"]
	addMana(t, e, 0, "CCC")
	d := e.Pending()
	act := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == arena {
			act = o.Index
		}
	}
	if act < 0 {
		t.Fatalf("precondition: Arena's ability not offered: %+v", d.Options)
	}
	submitChoices(t, e, act)
	mark := len(e.L.Events)
	for i := 0; i < 40 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving Arena")
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passPriorityOnce(t, e)
			continue
		}
		if d.Kind != decision.KTarget || len(d.Options) != 1 {
			t.Fatalf("unexpected Arena ask %+v", d)
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	// The two hits are simultaneous (CR 701.14a): their log order is not
	// part of the contract, only who took what.
	taken := map[state.ObjID]int32{}
	got := cr70114DamageSince(e, mark)
	for _, ev := range got {
		taken[ev.Obj] += ev.Amount
	}
	if len(got) != 2 || taken[giant] != 6 || taken[wurm] != 3 {
		t.Fatalf("Arena fight damage = %+v, want one hit each: Hill Giant 6, Craw Wurm 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestCR701_14cSelfFightDealsTwicePower: a 3/7 instructed to fight itself
// (Defined$ Self, the chosen target is the same creature) deals 6 damage to
// itself, as one hit.
func TestCR701_14cSelfFightDealsTwicePower(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	ox := onBoard(t, e, 0, "Name:Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:3/7\nOracle:x\n")
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: ox, Controller: 0,
		Targets: []state.Target{{Obj: ox}}, TargetsOffered: true}, cr70114TargetFightSA())
	got := cr70114DamageSince(e, mark)
	if len(got) != 1 || got[0].Obj != ox || got[0].Amount != 6 {
		t.Fatalf("self-fight damage = %+v, want one 6-damage hit on the Ox: CR 701.14c, twice its power", got)
	}
}

// TestCR701_14bNoncreatureFighterDealsNothing: the fighter is a
// noncreature permanent at resolution, so neither it nor the target creature
// deals damage.
func TestCR701_14bNoncreatureFighterDealsNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	relic := onBoard(t, e, 0, "Name:Relic\nManaCost:1\nTypes:Artifact\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if e.IsCreature(relic) {
		t.Fatal("precondition: the Relic must not be a creature")
	}
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: relic, Controller: 0,
		Targets: []state.Target{{Obj: bear}}, TargetsOffered: true}, cr70114TargetFightSA())
	if got := cr70114DamageSince(e, mark); len(got) != 0 {
		t.Fatalf("fight with a noncreature fighter dealt %+v, want nothing: CR 701.14b", got)
	}
}

// TestCR701_14aBloodFeudPairGuard is a guard (passes today): the
// no-Defined$ two-target shape (`SP$ Fight | ValidTgts$ Creature |
// TargetMin$ 2 | TargetMax$ 2`) pairs its two targets against each other
// once -- never a self-fight per target.
func TestCR701_14aBloodFeudPairGuard(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	ox := onBoard(t, e, 0, "Name:Ox\nManaCost:1 G\nTypes:Creature Ox\nPT:3/7\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/7\nOracle:x\n")
	sa := &cards.SA{Kind: "SP", API: "Fight", Params: map[string]string{
		"ValidTgts": "Creature", "TargetMin": "2", "TargetMax": "2", "TgtPrompt": "x"}}
	mark := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Source: ox, Controller: 0,
		Targets: []state.Target{{Obj: ox}, {Obj: bear}}, TargetsOffered: true}, sa)
	got := cr70114DamageSince(e, mark)
	if len(got) != 2 || got[0].Obj != bear || got[0].Amount != 3 || got[1].Obj != ox || got[1].Amount != 2 {
		t.Fatalf("two-target fight = %+v, want bear 3 then ox 2 (one mutual pair)", got)
	}
}
