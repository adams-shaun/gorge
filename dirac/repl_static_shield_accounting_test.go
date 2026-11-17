package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStaticShieldCombatAccountingUsesPickedOrder: CR 616.1e. A combat event
// of 5 is matched by an all-but-one static prevention and a prevent-one
// shield. The two do not commute: in scan order (all-but first) nothing is
// left, while the order walk's picks (the shield first) leave 1. The pass's
// accounting must follow the picked order, so the DamageDone trigger sees the
// event with 1, not the scan order's 0.
func TestStaticShieldCombatAccountingUsesPickedOrder(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	add := func(c kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, c)
		return CardID(len(tbl.Cards) - 1)
	}
	allBut := add(kop.CardDef{Name: "Test All But One", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement,
			Repl: kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPAllBut, A: 1}}}})
	shield := add(kop.CardDef{Name: "Test Prevent One", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement,
			Repl: kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 1}}}})
	watcher := add(kop.CardDef{Name: "Test Damage Watcher", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDamageDone, Flags: kop.TfToPlayer, Flags2: kop.Tf2ToYou},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(5)}}}}})
	attacker := add(kop.CardDef{Name: "Test Five", Types: kop.TCreature, Power: 5, Toughness: 5})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	if !s.k().gen.uses.has(useStaticPrev) {
		t.Fatal("precondition: the table's static preventions are not lowered to the kernel walk")
	}
	// Seat 1 owns the shields. Entry order puts the all-but host first, so
	// replCombatGen's scan order fully prevents the event while picking the
	// shield first leaves 1.
	sc.perm(allBut, 1)
	shieldObj := sc.perm(shield, 1)
	sc.perm(watcher, 1)
	atk := sc.perm(attacker, 0)
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(atk), scAny)
	sc.do(t, AkDone, scAny, scAny)
	asked, picked := false, false
	for i := 0; i < 80; i++ {
		if s.step == kop.StepMain2 && s.sp == 0 {
			break
		}
		d := s.Step()
		switch {
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub == ChGenReplOrder:
			if d.Player != 1 || d.N != 2 {
				t.Fatalf("order ask: player %d, %d options; want player 1 with 2", d.Player, d.N)
			}
			asked = true
			for j := 0; j < int(d.N); j++ {
				if Obj(d.Options[j].Source()) == shieldObj {
					s.Apply(d.Options[j])
					picked = true
					break
				}
			}
			if !picked {
				t.Fatalf("no option names the prevent-one shield: %v", d.Options[:d.N])
			}
		default:
			s.Apply(d.Options[0])
		}
	}
	if !asked || !picked {
		t.Fatalf("the combat order ask was not posed (asked %v picked %v)", asked, picked)
	}
	if s.step != kop.StepMain2 {
		t.Fatalf("step %d, want main 2 after combat", s.step)
	}
	// The picked order leaves 1 (5, less the shield's 1, less the all-but's 3):
	// seat 1 loses 1 and the DamageDone watcher gains 5.
	if got := s.pl[1].life; got != 24 {
		t.Fatalf("seat 1 life %d, want 24 (20 - 1 dealt + 5 from the damage trigger)", got)
	}
}
