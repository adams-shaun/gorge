package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// bareTable is vanilla's table with no generated side tables (no Exprs,
// TypeVocab or Conds): the only thing that can build the kernel's gen is a
// genNeed predicate over the table's replacement kinds.
func bareTable() *spTable {
	t := &spTable{tbl: *v.Table}
	t.tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	t.tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	return t
}

// TestReplDmgRewriteGenNeed: a table whose only generated feature is one of
// the damage-rewrite or phase-skip replacement kinds builds the gen side
// (every rb2 read gates on k.gen != nil).
func TestReplDmgRewriteGenNeed(t *testing.T) {
	for _, c := range []struct {
		name string
		r    kop.Repl
		bit  uint32
	}{
		{"DamageSet", kop.Repl{Kind: kop.ReplDamageSet, A: 3, C: int16(kop.ReplDmgSetGE(4)) | kop.ReplDmgPerm}, rb2DmgSet},
		{"DamageMinus", kop.Repl{Kind: kop.ReplDamageMinus, A: 1, C: kop.ReplDmgPerm}, rb2DmgMinus},
		{"BeginPhase", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep)}, rb2BeginPhase},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := bareTable()
			if len(tb.tbl.Exprs) != 0 || len(tb.tbl.Conds) != 0 {
				t.Fatal("precondition: the bare table carries generated side tables")
			}
			tb.host("Test Host", c.r)
			sc := tb.scen(t)
			if sc.k.gen == nil {
				t.Fatalf("a table whose only generated feature is %s built no gen", c.name)
			}
			if sc.k.gen.anyRepl2&c.bit == 0 {
				t.Fatalf("%s: the gen's gate word %#x lacks %#x", c.name, sc.k.gen.anyRepl2, c.bit)
			}
		})
	}
}

// TestReplDmgRewriteClash: minus/set rewrites that match one event beside a
// non-commuting rewrite cap loudly (gorge asks the CR 616.1 order); so do two
// minus lines, which commute but which gorge asks the order of all the same
// (two Lashknife Barriers: the mbc damage_minus_pair_order_ask recording).
func TestReplDmgRewriteClash(t *testing.T) {
	set := func(a int16, gate int) kop.Repl {
		return kop.Repl{Kind: kop.ReplDamageSet, A: a, C: int16(kop.ReplDmgSetGE(gate)) | kop.ReplDmgPerm}
	}
	minus := kop.Repl{Kind: kop.ReplDamageMinus, A: 1, C: kop.ReplDmgPerm}
	for _, c := range []struct {
		name  string
		repls []kop.Repl
		stall bool
	}{
		{"set beside minus", []kop.Repl{set(3, 4), minus}, true},
		{"two sets", []kop.Repl{set(3, 4), set(2, 3)}, true},
		{"two minus ask the order", []kop.Repl{minus, minus}, true},
		{"one minus alone", []kop.Repl{minus}, false},
		{"one set alone", []kop.Repl{set(3, 4)}, false},
		{"set under its gate beside minus", []kop.Repl{set(3, 9), minus}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := newSPTable()
			var hosts []CardID
			for i, r := range c.repls {
				hosts = append(hosts, tb.host("Test Host "+string(rune('A'+i)), r))
			}
			sc := tb.scen(t)
			for _, h := range hosts {
				sc.perm(h, 0)
			}
			bear := sc.perm(v.GrizzlyBears, 0)
			giant := sc.perm(v.HillGiant, 1)
			spDeal(sc, 1, giant, TCode(bear), 5)
			got := sc.s.StallReason() == StallReplPlus || sc.s.g.stalled == StallReplPlus
			if got != c.stall {
				t.Fatalf("stalled %v (reason %d, g.stalled %d), want %v", got, sc.s.StallReason(), sc.s.g.stalled, c.stall)
			}
		})
	}
}

// TestReplDmgRewriteUntapSkipChain: a pending untap skip enters the upkeep,
// which a printed upkeep skip rewrites in turn (gorge's setStep(Upkeep)
// proposal goes through the replacement pass).
func TestReplDmgRewriteUntapSkipChain(t *testing.T) {
	for _, withLine := range []bool{true, false} {
		name := "a printed upkeep skip chains"
		if !withLine {
			name = "no printed skip: the upkeep stands"
		}
		t.Run(name, func(t *testing.T) {
			tb := newSPTable()
			tb.tbl.Cards = append(tb.tbl.Cards, kop.CardDef{Name: "Test Skipper", Types: kop.TSorcery,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
					{Op: kop.OpExt, A: kop.Const(int(kop.ExtSkipPhase)), C: kop.SkipPhaseUntap}}}}})
			if withLine {
				h := tb.host("Test Eon", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepUpkeep)})
				sc := tb.scen(t)
				sc.perm(h, 0)
				skipUntapChain(t, sc, true)
				return
			}
			tb.host("Test Other", kop.Repl{Kind: kop.ReplBeginPhase, A: int16(kop.StepDraw)})
			sc := tb.scen(t)
			skipUntapChain(t, sc, false)
		})
	}
}

func skipUntapChain(t *testing.T, sc *scen, upkeepSkipped bool) {
	t.Helper()
	r := coldSkipOne.of(sc.s)
	if len(r) == 0 {
		t.Fatal("precondition: the one-shot skip region is not laid out")
	}
	r[sc.s.active] |= skUntap
	sc.s.skipUntapTurn()
	want := kop.StepUpkeep
	if upkeepSkipped {
		want = kop.StepDraw // the upkeep skip lands in the draw step (turn 2: no CR 103.8a skip)
	}
	if sc.s.step != want {
		t.Fatalf("step after a consumed untap skip = %d, want %d", sc.s.step, want)
	}
}
