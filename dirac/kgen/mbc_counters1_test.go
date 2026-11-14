package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// mbcCounters1Build lowers and builds one inline card, returning its def, the
// pool's expressions and its refusals.
func mbcCounters1Build(t *testing.T, name, src string) (kop.CardDef, []kop.Expr, []refusal) {
	t.Helper()
	card, diags := cards.ParseBytes("mbc_counters1_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, pt.exprs, lw.cards[0].Refusals
}

func mbcCounters1Instrs(d kop.CardDef) []kop.Instr {
	var out []kop.Instr
	for i := range d.Abilities {
		out = append(out, d.Abilities[i].Prog...)
	}
	return out
}

// TestMBCCounters1Lowerings pins the encoding of the mbc-counters-1 spellings:
// the Experience player counter (OpGainEnergy C = PlCtrExperience) and its
// Count$YourCountersExperience read (ExEnergyOf N = PlCtrExperience), the
// COMPONENT and DESCENT counter kinds, and a literal-N DividedAsYouChoose$
// over -2/-1 counters (ExtDividedCounters B.N = CtrM2M1).
func TestMBCCounters1Lowerings(t *testing.T) {
	t.Run("experience", func(t *testing.T) {
		const src = "Name:Exp Probe\nManaCost:1\nTypes:Artifact\n" +
			"A:AB$ PutCounter | Cost$ T | Defined$ You | CounterType$ Experience | CounterNum$ 1 | SpellDescription$ You get an experience counter.\n" +
			"A:AB$ GainLife | Cost$ 1 | LifeAmount$ ExpRead | SpellDescription$ Gain life.\n" +
			"SVar:ExpRead:Count$YourCountersExperience\nOracle:x\n"
		d, exprs, rs := mbcCounters1Build(t, "Exp Probe", src)
		if len(rs) != 0 {
			t.Fatalf("refused: %v", rs)
		}
		gain := false
		for _, in := range mbcCounters1Instrs(d) {
			if in.Op == kop.OpGainEnergy && in.C == kop.PlCtrExperience {
				gain = true
			}
		}
		read := false
		for _, x := range exprs {
			if x.Kind == kop.ExEnergyOf && x.N == kop.PlCtrExperience {
				read = true
			}
		}
		if !gain || !read {
			t.Fatalf("experience gain %v read %v in %+v / %+v", gain, read, d.Abilities, exprs)
		}
	})
	for _, c := range []struct{ kind, name string }{{"COMPONENT", "component"}, {"DESCENT", "descent"}} {
		t.Run(c.name, func(t *testing.T) {
			src := "Name:Ctr Probe\nManaCost:1\nTypes:Artifact\n" +
				"A:AB$ PutCounter | Cost$ T | Defined$ Self | CounterType$ " + c.kind + " | CounterNum$ 2 | SpellDescription$ Put counters.\nOracle:x\n"
			d, _, rs := mbcCounters1Build(t, "Ctr Probe", src)
			if len(rs) != 0 {
				t.Fatalf("refused: %v", rs)
			}
			want, err := counterKind(c.kind)
			if err != nil {
				t.Fatal(err)
			}
			for _, in := range mbcCounters1Instrs(d) {
				if in.Op == kop.OpPutCounter && kop.CounterKind(in.C) == want {
					return
				}
			}
			t.Fatalf("no PutCounter of %s in %+v", c.kind, d.Abilities)
		})
	}
	t.Run("divided-m2m1", func(t *testing.T) {
		const src = "Name:Div Probe\nManaCost:3 B B\nTypes:Instant\n" +
			"A:SP$ PutCounter | ValidTgts$ Creature | TgtPrompt$ Select target creature | CounterType$ M2M1 | CounterNum$ 2 | TargetMin$ 1 | TargetMax$ 2 | DividedAsYouChoose$ 2 | SpellDescription$ Distribute two -2/-1 counters.\nOracle:x\n"
		d, _, rs := mbcCounters1Build(t, "Div Probe", src)
		if len(rs) != 0 {
			t.Fatalf("refused: %v", rs)
		}
		for _, in := range mbcCounters1Instrs(d) {
			if in.Op == kop.OpExt && in.Ext() == kop.ExtDividedCounters {
				if kop.CounterKind(in.B.N) != kop.CtrM2M1 || in.B.K != kop.VConst {
					t.Fatalf("DividedCounters B = %+v, want the M2M1 kind", in.B)
				}
				return
			}
		}
		t.Fatalf("no ExtDividedCounters in %+v", d.Abilities)
	})
	t.Run("divided-unknown-kind", func(t *testing.T) {
		// a kind the kernel does not keep still refuses, naming it
		const src = "Name:Div Probe\nManaCost:3 B B\nTypes:Instant\n" +
			"A:SP$ PutCounter | ValidTgts$ Creature | TgtPrompt$ Select target creature | CounterType$ NOSUCHKIND | CounterNum$ 2 | TargetMin$ 1 | TargetMax$ 2 | DividedAsYouChoose$ 2 | SpellDescription$ Distribute.\nOracle:x\n"
		_, _, rs := mbcCounters1Build(t, "Div Probe", src)
		if len(rs) != 1 || !strings.Contains(rs[0].Reason, `counter kind "NOSUCHKIND"`) {
			t.Fatalf("refusals = %v, want the counter-kind reason", rs)
		}
	})
}
