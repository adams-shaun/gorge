package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// trigTokencreatedQueued counts object o's triggered abilities pending or on the stack.
func trigTokencreatedQueued(sc *scen, o Obj) int {
	n := 0
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].obj == o {
			n++
		}
	}
	for i := 0; i < int(sc.s.sp); i++ {
		if e := &sc.s.stack[i]; e.kind == seTrigger && e.obj == o {
			n++
		}
	}
	return n
}

// TestTokenCreatedOnce: Staff of the Storyteller's "whenever you create
// one or more creature tokens" (gorge once per token batch, one
// resolution) queues once for a resolution creating three tokens over two
// instructions, again for a second resolution the same turn, and not for
// the opponent's tokens (ValidToken$ Creature.YouOwn).
func TestTokenCreatedOnce(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	n := CardID(len(tbl.Cards))
	tok, staff, make3, opp := n, n+1, n+2, n+3
	mk := func(a int) kop.Instr {
		return kop.Instr{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(a), C: int16(tok)}
	}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Token", Types: kop.TCreature | kop.TToken, Power: 1, Toughness: 1},
		kop.CardDef{Name: "Test Staff", Types: kop.TArtifact, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTokenCreated, Filter: yours, Flags2: kop.Tf2Batch},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Three Tokens", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{mk(2), mk(1)}}}},
		kop.CardDef{Name: "Test Opponent Token", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpCreateToken, Obj: kop.RefOpponent, A: kop.Const(1), C: int16(tok)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyTokCr {
		t.Fatal("anyTokCr unset")
	}
	st := sc.perm(staff, 0)
	sc.actProliferateCast(t, make3)
	if q := trigTokencreatedQueued(sc, st); q != 1 {
		t.Fatalf("one resolution creating three tokens queued %d Staff triggers, want 1", q)
	}
	sc.s.npend, sc.s.sp = 0, 0
	sc.actProliferateCast(t, make3)
	if q := trigTokencreatedQueued(sc, st); q != 1 {
		t.Fatalf("a second resolution queued %d Staff triggers, want 1", q)
	}
	sc.s.npend, sc.s.sp = 0, 0
	sc.actProliferateCast(t, opp)
	if q := trigTokencreatedQueued(sc, st); q != 0 {
		t.Fatalf("the opponent's token queued %d Staff triggers, want 0", q)
	}
}

// TestCopyTokenFiresTokenCreated: a copy effect's token is created (gorge
// CR 111.1): Staff of the Storyteller's TokenCreated line fires for
// an OpCopyAsTokenEx token as it does for an OpCreateToken one.
func TestCopyTokenFiresTokenCreated(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	n := CardID(len(tbl.Cards))
	staff, mirror := n, n+1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Staff", Types: kop.TArtifact, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTokenCreated, Filter: yours, Flags2: kop.Tf2Batch},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Mirror", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{{Kind: kop.AbActivated,
			Prog: []kop.Instr{{Op: kop.OpCopyAsTokenEx, Obj: kop.RefSelf, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	st := sc.perm(staff, 0)
	m := sc.perm(mirror, 0)
	sc.do(t, AkActivate, TCode(m), scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	if q := trigTokencreatedQueued(sc, st); q != 1 {
		t.Fatalf("a copy token queued %d Staff triggers, want 1", q)
	}
}
