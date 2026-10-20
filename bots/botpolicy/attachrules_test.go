package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// attachBoard builds the A5 fixture: the given creatures, the rules armed
// (zero = OFF), and source 900 either an Aura spell on the stack or an
// Equipment on the battlefield.
func attachBoard(r AttachRules, equipment bool, fs ...fact) Board {
	b := boardOf(fs...)
	b.Attach = r
	b.Cards = TableOf(map[state.ObjID]Card{})
	if equipment {
		b.Cards = TableOf(map[state.ObjID]Card{900: {OnBattlefield: true}})
	} else {
		b.Stack = []StackEntry{{ID: 900, Controller: 0, IsSpell: true}}
	}
	return b
}

// attachPick answers an Attach KTarget ask (source 900, hint logic) over the
// given option tuples and returns the chosen object.
func attachPick(b Board, logic string, options ...tgt) state.ObjID {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Source: 900,
		TargetEffect: &decision.TargetEffect{API: "Attach", AttachLogic: logic}}
	for _, o := range options {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: o.kind, Obj: o.obj, Player: o.pl, Controller: o.pl})
	}
	in := Decide(b, &d, rng(1))
	if len(in.Choices) != 1 {
		return 0
	}
	return objAt(&d, in.Choices[0])
}

var allAttach = AttachRules{AuraPolarity: true, Carrier: true, HoldOwnAura: true}

func TestAttachPumpAuraAimsOwn(t *testing.T) {
	fs := []fact{atk(1, 2, 2), def(1, 4, 4)}
	opts := []tgt{opp(201), mine(101)}
	if got := attachPick(attachBoard(AttachRules{AuraPolarity: true}, false, fs...), "Pump", opts...); got != 101 {
		t.Errorf("ON picked %d, want own 101", got)
	}
	if got := attachPick(attachBoard(AttachRules{}, false, fs...), "Pump", opts...); got != 201 {
		t.Errorf("OFF picked %d, want 201 (the fixture must discriminate)", got)
	}
}

func TestAttachPumpAuraNoOwnPicksWeakestOpponent(t *testing.T) {
	fs := []fact{def(1, 4, 4), def(2, 1, 1)}
	opts := []tgt{opp(201), opp(202)}
	if got := attachPick(attachBoard(AttachRules{AuraPolarity: true}, false, fs...), "Pump", opts...); got != 202 {
		t.Errorf("ON picked %d, want the weakest opponent 202", got)
	}
	if got := attachPick(attachBoard(AttachRules{}, false, fs...), "Pump", opts...); got != 201 {
		t.Errorf("OFF picked %d, want 201", got)
	}
}

func TestAttachCurseForcedOwnPicksWeakest(t *testing.T) {
	on := attachBoard(AttachRules{AuraPolarity: true}, false, atk(1, 4, 4), atk(2, 1, 1))
	off := attachBoard(AttachRules{}, false, atk(1, 4, 4), atk(2, 1, 1))
	opts := []tgt{mine(101), mine(102)}
	if got := attachPick(on, "Curse", opts...); got != 102 {
		t.Errorf("ON picked %d, want the weakest own 102", got)
	}
	if got := attachPick(off, "Curse", opts...); got != 101 {
		t.Errorf("OFF picked %d, want 101", got)
	}
	opts = []tgt{mine(101), opp(201), mine(102)}
	on = attachBoard(AttachRules{AuraPolarity: true}, false, atk(1, 4, 4), atk(2, 1, 1), def(1, 3, 3))
	off = attachBoard(AttachRules{}, false, atk(1, 4, 4), atk(2, 1, 1), def(1, 3, 3))
	if got := attachPick(on, "Curse", opts...); got != 201 {
		t.Errorf("ON with a foreign option picked %d, want 201", got)
	}
	if got := attachPick(off, "Curse", opts...); got != 201 {
		t.Errorf("OFF with a foreign option picked %d, want 201", got)
	}
}

func TestAttachUnknownLogicUnchanged(t *testing.T) {
	fs := []fact{atk(1, 2, 2), def(1, 4, 4)}
	opts := []tgt{opp(201), mine(101)}
	on := attachPick(attachBoard(allAttach, false, fs...), "", opts...)
	off := attachPick(attachBoard(AttachRules{}, false, fs...), "", opts...)
	if on != off || on != 201 {
		t.Errorf("ON %d, OFF %d, want both 201", on, off)
	}
}

func TestEquipCarrierDemotesDefender(t *testing.T) {
	fs := []fact{atk(1, 0, 4, "Defender"), atk(2, 1, 1)}
	opts := []tgt{mine(101), mine(102)}
	if got := attachPick(attachBoard(AttachRules{Carrier: true}, true, fs...), "", opts...); got != 102 {
		t.Errorf("ON picked %d, want the attacker 102", got)
	}
	if got := attachPick(attachBoard(AttachRules{}, true, fs...), "", opts...); got != 101 {
		t.Errorf("OFF picked %d, want the Defender 101 (the fixture must discriminate)", got)
	}
}

func TestEquipCarrierIgnoresTapped(t *testing.T) {
	fs := []fact{
		{101, Creature{Power: 3, Toughness: 3, Tapped: true, Controller: 0}},
		{102, Creature{Power: 2, Toughness: 2, Controller: 0}},
	}
	opts := []tgt{mine(102), mine(101)}
	if got := attachPick(attachBoard(AttachRules{Carrier: true}, true, fs...), "", opts...); got != 101 {
		t.Errorf("ON picked %d, want the bigger tapped 101", got)
	}
	if got := attachPick(attachBoard(AttachRules{}, true, fs...), "", opts...); got != 102 {
		t.Errorf("OFF picked %d, want the untapped 102", got)
	}
}

func TestEquipDeclinedWithOnlyDefenders(t *testing.T) {
	opts := func() []decision.Option {
		return []decision.Option{
			{Index: 0, Kind: "ability", Obj: 900, Attach: true, Label: "Bonesplitter: Equip 1"},
			{Index: 1, Kind: "pass"},
		}
	}
	run := func(r AttachRules, fs ...fact) (int, bool) {
		b := boardOf(fs...)
		b.Attach = r
		b.IsMain, b.MyTurn = true, true
		b.Cards = TableOf(map[state.ObjID]Card{900: {OnBattlefield: true, AttachedTo: 0}})
		d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: opts()}
		in := Decide(b, &d, rng(1))
		return in.Choices[0], b.AbilityWorthTaking(d.Options[0], 0)
	}
	on := AttachRules{Carrier: true}
	if c, worth := run(AttachRules{}, atk(1, 0, 4, "Defender")); c != 0 || !worth {
		t.Errorf("OFF chose %d worth=%v, want the equip (0, true)", c, worth)
	}
	if c, worth := run(on, atk(1, 0, 4, "Defender")); c != 1 || worth {
		t.Errorf("ON chose %d worth=%v, want pass (1, false)", c, worth)
	}
	if c, worth := run(on, atk(1, 0, 4, "Defender"), atk(2, 1, 1)); c != 0 || !worth {
		t.Errorf("ON with an attacker chose %d worth=%v, want the equip (0, true)", c, worth)
	}
}
