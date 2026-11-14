package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnblockedTrigger: a Tf2Unblocked line (gorge Mode$ AttackerUnblocked)
// fires once per unblocked attacker matching its filter as the blocks
// complete, and never for a blocked one.
func TestUnblockedTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	crea := kop.FilterID(len(tbl.Filters) - 1)
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	hound, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Hound", Types: kop.TCreature, Power: 1, Toughness: 4, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Flags: kop.TfSelf, Flags2: kop.Tf2Unblocked}, Prog: gain},
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigBecomesBlocked, Filter: crea, Flags2: kop.Tf2Unblocked}, Prog: gain}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 4})
	for _, blocked := range []bool{false, true} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if !sc.k.gen.anyUnblocked {
			t.Fatal("genPrep.anyUnblocked not set")
		}
		h, a := sc.perm(hound, 0), sc.perm(bear, 0)
		b := sc.perm(bear, 1)
		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkAttack, TCode(h), scAny)
		sc.do(t, AkAttack, TCode(a), scAny)
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepBlockers)
		if blocked {
			sc.do(t, AkBlock, TCode(b), TCode(h))
		}
		sc.do(t, AkDone, scAny, scAny)
		// unblocked: the self line once, the creature line twice (hound,
		// bear); the hound blocked: the creature line once (the bear)
		want := 3
		if blocked {
			want = 1
		}
		if got := int(sc.s.npend) + int(sc.s.sp); got != want {
			t.Fatalf("blocked=%v: %d triggers, want %d", blocked, got, want)
		}
	}
}
