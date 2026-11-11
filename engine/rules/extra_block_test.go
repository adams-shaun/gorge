// CR 509.1a / 510.1d. CanBlockAmount$ / CanBlockAny$ statics let one blocker
// be declared against several attackers (Ghastbark Twins, Wall of Glare), and
// the blocker's controller divides its combat damage among them. Authored
// fixtures: no corpus card is loaded.
package rules

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	twinsSrc = "Name:Ghastbark Twins\nManaCost:5 G G\nTypes:Creature Treefolk\nPT:7/7\nK:Trample\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ 1 | Description$ CARDNAME can block an additional creature each combat.\nOracle:x\n"
	plainSevenSrc = "Name:Plain Seven\nManaCost:5 G G\nTypes:Creature Treefolk\nPT:7/7\nOracle:x\n"
	atk22Src      = "Name:Atk Two\nManaCost:1 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n"
	atk33Src      = "Name:Atk Three\nManaCost:2 R\nTypes:Creature Goblin\nPT:3/3\nOracle:x\n"
	atk44Src      = "Name:Atk Four\nManaCost:3 R\nTypes:Creature Goblin\nPT:4/4\nOracle:x\n"
	teamBlockSrc  = "Name:Team Sentinel\nManaCost:2 W\nTypes:Creature Human\nPT:1/1\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | CanBlockAmount$ 1 | Description$ Each creature you control can block an additional creature each combat.\nOracle:x\n"
	anyBlockSrc = "Name:Wall of Glare\nManaCost:1 W\nTypes:Creature Wall\nPT:0/4\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAny$ True | Description$ CARDNAME can block any number of creatures.\nOracle:x\n"
	xBlockSrc = "Name:X Blocker\nManaCost:2 W\nTypes:Creature Human\nPT:2/2\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ X | Description$ CARDNAME can block X additional creatures.\nSVar:X:Count$Valid Creature\nOracle:x\n"
	gatedBlockSrc = "Name:Gated Blocker\nManaCost:2 W\nTypes:Creature Human\nPT:2/2\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ 1 | Condition$ Blessing | Description$ x\nOracle:x\n"
)

// extraBlockTable seats two attackers (sources given) on seat 0 and the
// blocker on seat 1, attackers declared and the block offer pending.
func extraBlockTable(t *testing.T, blockerSrc string, atkSrcs ...string) (*Engine, []state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	e := combatEngine(t)
	var atks []state.ObjID
	for _, s := range atkSrcs {
		atks = append(atks, onBoardReady(t, e, 0, s))
	}
	blk := onBoard(t, e, 1, blockerSrc)
	e.askAttackers()
	submitAttackersOnly(t, e, atks...)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("no blockers decision: %+v", d)
	}
	return e, atks, blk, d
}

func blockerOptions(d *decision.Decision, blk state.ObjID) (group string, n int) {
	for _, o := range d.Options {
		if o.Obj == blk {
			group = o.Group
			n++
		}
	}
	return group, n
}

func TestCR509_1aGhastbarkTwinsBlocksTwoAttackers(t *testing.T) {
	t.Parallel()
	e, atks, twins, d := extraBlockTable(t, twinsSrc, atk22Src, atk22Src)
	group, n := blockerOptions(d, twins)
	if n != 2 || group == "" {
		t.Fatalf("precondition: Twins has %d options in group %q, want 2", n, group)
	}
	if got := d.GroupLimits[group]; got != 2 {
		t.Fatalf("GroupLimits[%q] = %d, want 2 (%v)", group, got, d.GroupLimits)
	}
	submitBlockerPairsOnly(t, e, [2]state.ObjID{atks[0], twins}, [2]state.ObjID{atks[1], twins})
	for _, a := range atks {
		if bb := e.G.Obj(a).BlockedBy; len(bb) != 1 || bb[0] != twins {
			t.Fatalf("attacker %d BlockedBy = %v, want [%d]", a, bb, twins)
		}
	}

	// Control: an ordinary 7/7 may not be declared against both.
	e2, atks2, plain, d2 := extraBlockTable(t, plainSevenSrc, atk22Src, atk22Src)
	if g2, _ := blockerOptions(d2, plain); d2.GroupLimits[g2] != 0 {
		t.Fatalf("control offer raised a limit: %v", d2.GroupLimits)
	}
	var choices []int
	for _, a := range atks2 {
		for _, o := range d2.Options {
			if o.Obj == plain && o.Attacker == a {
				choices = append(choices, o.Index)
			}
		}
	}
	if len(choices) != 2 {
		t.Fatalf("control precondition: %d pair options", len(choices))
	}
	if err := e2.Submit(decision.Intent{Seq: d2.Seq, Player: d2.Player, Choices: choices}); err == nil {
		t.Fatal("control: an ordinary blocker was accepted against two attackers")
	}
}

func TestCR510_1dGhastbarkTwinsControllerDividesSevenDamage(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, split []int32) (*Engine, []state.ObjID, state.ObjID) {
		e, atks, twins, _ := extraBlockTable(t, twinsSrc, atk33Src, atk44Src)
		submitBlockerPairsOnly(t, e, [2]state.ObjID{atks[0], twins}, [2]state.ObjID{atks[1], twins})
		drainCombatPriority(t, e)
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Source != twins || d.Player != 1 {
			t.Fatalf("pending = %+v, want seat 1's division of Twins' damage", d)
		}
		if len(d.Options) != 8 {
			t.Fatalf("division options = %d, want 8 splits of 7", len(d.Options))
		}
		submitDivision(t, e, split)
		return e, atks, twins
	}
	t.Run("3-4 kills both", func(t *testing.T) {
		e, atks, twins := run(t, []int32{3, 4})
		for _, a := range atks {
			if z := e.G.Obj(a).Zone; z != state.ZGraveyard {
				t.Fatalf("attacker %d zone = %v, want graveyard", a, z)
			}
		}
		if z := e.G.Obj(twins).Zone; z != state.ZGraveyard {
			t.Fatalf("Twins zone = %v, want graveyard (3+4 damage)", z)
		}
	})
	t.Run("7-0 kills only the first", func(t *testing.T) {
		e, atks, twins := run(t, []int32{7, 0})
		if z := e.G.Obj(atks[0]).Zone; z != state.ZGraveyard {
			t.Fatalf("first attacker zone = %v, want graveyard", z)
		}
		if o := e.G.Obj(atks[1]); o.Zone != state.ZBattlefield || o.Damage != 0 {
			t.Fatalf("second attacker zone %v damage %d, want untouched", o.Zone, o.Damage)
		}
		if z := e.G.Obj(twins).Zone; z != state.ZGraveyard {
			t.Fatalf("Twins zone = %v, want graveyard (7 damage)", z)
		}
	})
}

func TestExtraBlockCapacity(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	twins := onBoard(t, e, 1, twinsSrc)
	plain := onBoard(t, e, 1, plainSevenSrc)
	xb := onBoard(t, e, 1, xBlockSrc)
	anyb := onBoard(t, e, 1, anyBlockSrc)
	for _, id := range []state.ObjID{twins, plain, xb, anyb} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: %d not on the battlefield", id)
		}
	}
	cap := func(id state.ObjID) int { return combat.BlockCapacity(asBoard(e), id) }
	if got := cap(plain); got != 1 {
		t.Errorf("plain = %d, want 1", got)
	}
	if got := cap(twins); got != 2 {
		t.Errorf("Twins = %d, want 2", got)
	}
	if got := cap(xb); got != 1 {
		t.Errorf("CanBlockAmount$ X = %d, want 1 (non-literal skipped)", got)
	}
	if got := cap(anyb); got != math.MaxInt32 {
		t.Errorf("CanBlockAny$ = %d, want MaxInt32", got)
	}
	team := onBoard(t, e, 1, teamBlockSrc)
	if o := e.G.Obj(team); o.Zone != state.ZBattlefield {
		t.Fatal("precondition: team static not on the battlefield")
	}
	if got := cap(twins); got != 3 {
		t.Errorf("Twins + team static = %d, want 3", got)
	}
	if got := cap(plain); got != 2 {
		t.Errorf("plain + team static = %d, want 2", got)
	}
}

func TestExtraBlockAnyOfferLimitIsAttackerCount(t *testing.T) {
	t.Parallel()
	_, _, wall, d := extraBlockTable(t, anyBlockSrc, atk22Src, atk33Src, atk44Src)
	group, n := blockerOptions(d, wall)
	if n != 3 {
		t.Fatalf("precondition: Wall has %d options, want 3", n)
	}
	if got := d.GroupLimits[group]; got != 3 {
		t.Fatalf("GroupLimits[%q] = %d, want 3 (the attacker count)", group, got)
	}
}

func TestExtraBlockGateRespected(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	g := onBoard(t, e, 1, gatedBlockSrc)
	if e.G.Obj(g).Zone != state.ZBattlefield {
		t.Fatal("precondition: gated blocker not on the battlefield")
	}
	if e.G.Players[1].Blessing {
		t.Fatal("precondition: blessing already held")
	}
	if got := combat.BlockCapacity(asBoard(e), g); got != 1 {
		t.Fatalf("gate false: capacity = %d, want 1", got)
	}
	e.G.Players[1].Blessing = true
	e.staticEpoch = -1
	if got := combat.BlockCapacity(asBoard(e), g); got != 2 {
		t.Fatalf("gate true: capacity = %d, want 2", got)
	}
}

// TestExtraBlockBotAnswerIsLegal: the policy's answer to a Twins offer passes
// Decision.Validate and the engine, and Clamp of a raw double block keeps the
// legal double block (it does not trim it to one).
func TestExtraBlockBotAnswerIsLegal(t *testing.T) {
	t.Parallel()
	e, atks, twins, d := extraBlockTable(t, twinsSrc, atk22Src, atk22Src)
	var both []int
	for _, a := range atks {
		for _, o := range d.Options {
			if o.Obj == twins && o.Attacker == a {
				both = append(both, o.Index)
			}
		}
	}
	if len(both) != 2 {
		t.Fatalf("precondition: %d pair options", len(both))
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: both}); err != nil {
		t.Fatalf("Validate rejects the double block: %v", err)
	}
	in := newTestBot(7).answer(e, d)
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer fails Validate: %v (%v)", err, in.Choices)
	}
	if err := e.Submit(in); err != nil {
		t.Fatalf("bot answer rejected by the engine: %v (%v)", err, in.Choices)
	}
}
