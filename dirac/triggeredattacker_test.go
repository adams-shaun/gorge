package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTriggeredAttackerSingle: gorge's Defined$ TriggeredAttacker (and
// TriggeredAttackerLKICopy) is the one attacker that raised the event, not
// the whole declare-attackers batch. The kernel fires an
// Attacks trigger once per attacker with that attacker as the trigger
// object, so a body that puts a counter on TriggeredAttacker must put it on
// the triggering attacker only. The filter is a SUBTYPE filter so exactly one
// of the two attackers matches: gorge binds TriggerCard only when exactly one
// declared attacker matched, and with two matches it falls back to the whole
// Remembered batch (the pre-a2df098cc read). Bone Sabres
// (Creature.EquippedBy + Defined$ TriggeredAttackerLKICopy) is the accepted
// corpus carrier: its filter names one attacker by construction, unlike the
// broad Kragma Warcaller Minotaur.YouCtrl shape kgen refuses.
// TestTriggeredAttackerSingleLockstep guards the pin.
func TestTriggeredAttackerSingle(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sub := len(tbl.Subtypes)
	tbl.Subtypes = append(append([]string(nil), tbl.Subtypes...), "Cat")
	var sm kop.SubMask
	sm[sub>>6] |= 1 << (sub & 63)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Sub: uint8(sub + 1)})
	f := kop.FilterID(len(tbl.Filters) - 1)
	cat := CardID(len(tbl.Cards))
	bear := CardID(len(tbl.Cards) + 1)
	watcher := CardID(len(tbl.Cards) + 2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Cat", Types: kop.TCreature, Subtypes: sm, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Ordeal", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigAttacks, Filter: f},
				Prog: []kop.Instr{{Op: kop.OpPutCounter, Obj: kop.RefTriggerObj, A: kop.Const(1), C: int16(kop.CtrP1P1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(watcher, 0)
	a1, a2 := sc.perm(cat, 0), sc.perm(bear, 0)
	sc.combat(t, []Obj{a1, a2}, nil)
	// The Cat's Attacks trigger is ordered first (the helper stops at that
	// decision); answer it, then let the stack drain.
	for i := 0; i < 40 && (sc.s.npend > 0 || sc.s.sp > 0); i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		if d.N == 0 {
			t.Fatalf("stalled with %d pending, stack %d", sc.s.npend, sc.s.sp)
		}
		sc.s.Apply(d.Options[0])
	}
	ctr := func(o Obj) int { return sc.s.perms[sc.slot(o)].ctrLow(int(kop.CtrP1P1)) }
	if n1, n2 := ctr(a1), ctr(a2); n1 != 1 || n2 != 0 {
		t.Fatalf("counters after two attackers: %d / %d, want 1 / 0 (only the Cat matched TriggeredAttacker)", n1, n2)
	}
}
