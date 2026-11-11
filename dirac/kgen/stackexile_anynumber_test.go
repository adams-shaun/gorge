package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStackExileAnyNumber: a Mindbreak Trap-shaped instant, "exile any number
// of target spells" (ChangeZone Origin$ Stack to exile, TargetMin$ 0 up to
// the stack count) with a free alternative cost gated on an opponent having
// cast three or more spells this turn, lowers to an up-to slot over the stack
// whose program moves each target to exile (OpCounter CtrDestExile |
// CtrDestMove) and marks the card two-seat for the opponent read. Any other
// bound or condition head still refuses by name.
func TestStackExileAnyNumber(t *testing.T) {
	const head = "Name:Trap Probe\nManaCost:2 U U\nTypes:Instant Trap\n"
	src := func(cond, maxSV, dest string) string {
		return head +
			"S:Mode$ AlternativeCost | ValidSA$ Spell.Self | EffectZone$ All | Cost$ 0 | CheckSVar$ OppCast | Description$ free if an opponent cast enough.\n" +
			"A:SP$ ChangeZone | TargetType$ Spell | ValidTgts$ Card | TgtZone$ Stack | Origin$ Stack | Destination$ " + dest +
			" | TargetMin$ 0 | TargetMax$ MaxT | SpellDescription$ Move any number of target spells.\n" +
			"SVar:OppCast:" + cond + "\nSVar:MaxT:" + maxSV + "\n"
	}
	d, rs := kwardpowBuild(t, "Trap Probe", src("PlayerCountOpponents$ConditionGE3 SpellsCastThisTurn", "Count$ValidStack Card", "Exile"))
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	if !d.TwoSeat {
		t.Error("the opponent's spell count did not mark the card two-seat")
	}
	var ctr *kop.Instr
	var slot *kop.TargetSpec
	for ai := range d.Abilities {
		a := &d.Abilities[ai]
		for i := range a.Prog {
			if a.Prog[i].Op == kop.OpCounter {
				ctr, slot = &a.Prog[i], &a.Targets[0]
			}
		}
	}
	if ctr == nil {
		t.Fatal("no OpCounter in the spell's program")
	}
	if ctr.C != kop.CtrDestExile|kop.CtrDestMove {
		t.Errorf("Counter C %d, want exile | move %d", ctr.C, kop.CtrDestExile|kop.CtrDestMove)
	}
	if slot.Kind != kop.TgtSpell || !slot.UpTo || slot.Min != 0 || slot.Max != 254 {
		t.Errorf("slot kind %d upto %v %d..%d, want a stack up-to 0..254 slot", slot.Kind, slot.UpTo, slot.Min, slot.Max)
	}

	for _, p := range []struct{ name, src, want string }{
		{"other bound", src("PlayerCountOpponents$ConditionGE3 SpellsCastThisTurn", "Count$Valid Creature", "Exile"), "only 0 up to Count$ValidStack Card"},
		{"other operator", src("PlayerCountOpponents$ConditionLT3 SpellsCastThisTurn", "Count$ValidStack Card", "Exile"), "only ConditionGE<n> SpellsCastThisTurn"},
		{"other property", src("PlayerCountOpponents$ConditionGE3 CardsDrawn", "Count$ValidStack Card", "Exile"), "only ConditionGE<n> SpellsCastThisTurn"},
		{"graveyard", src("PlayerCountOpponents$ConditionGE3 SpellsCastThisTurn", "Count$ValidStack Card", "Graveyard"), "ChangeZone Origin$ Stack shape"},
	} {
		_, rs := kwardpowBuild(t, "Trap Probe", p.src)
		if all := kpaylifexReasons(rs); !strings.Contains(all, p.want) {
			t.Errorf("%s: refusals %q, want %q", p.name, all, p.want)
		}
	}

	// A one-target stack exile (no TargetMin$/TargetMax$) is not this shape.
	// Like Gale's Redirection it carries RememberChanged, and the earlier
	// ChangeZone refusal must stay ahead of any later value head.
	const single = head +
		"A:SP$ ChangeZone | TargetType$ Spell | ValidTgts$ Card | TgtZone$ Stack | Origin$ Stack | Destination$ Exile | RememberChanged$ True | SpellDescription$ Exile target spell.\n"
	if _, rs := kwardpowBuild(t, "Trap Probe", single); !strings.Contains(kpaylifexReasons(rs), "ChangeZone Origin$ Stack shape") {
		t.Errorf("one-target exile: refusals %q, want the ChangeZone shape", kpaylifexReasons(rs))
	}
}
