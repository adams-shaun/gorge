package rules

import "testing"

// TestImproviseShapesStayWithTheManualWindow: a plain Improvise cast is
// planned (TestImproviseCastPaysColouredPipFromLand); an Improvise cast with
// an {X} cost or a sacrifice additional cost is not priced by planImprovise
// and still answers shape:contribution (payment_plan.go).
func TestImproviseShapesStayWithTheManualWindow(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		name, face, mana, detail string
	}{
		{"plain", "A:SP$ Draw | NumCards$ 1", "1 U", ""},
		{"x", "A:SP$ Draw | NumCards$ 1", "X U", "shape:contribution"},
		{"sacrifice", "A:SP$ Draw | Cost$ 1 U Sac<1/Creature> | NumCards$ 1", "1 U", "shape:contribution"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "Name:Carve Out\nManaCost:" + tc.mana + "\nTypes:Sorcery\nK:Improvise\n" + tc.face + "\nOracle:x\n"
			e, _, spell := newFixtureDeck(t, uint64(9700+i), src)
			toMain1(t, e)
			onBoard(t, e, 0, plainArtifactSrc)
			island := onBoard(t, e, 0, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
			e.G.Obj(island).SummonSick = false
			onBoard(t, e, 0, "Name:Victim\nManaCost:1\nTypes:Creature Test\nPT:1/1\nOracle:test\n")
			e.pending = nil
			e.askPriority(0)
			if !e.hasCastImprovise(spell) {
				t.Fatal("fixture spell lost Improvise")
			}
			got := e.PlanCastPayment(0, paymentCast(spell))
			if tc.detail == "" {
				if got.Plan == nil {
					t.Fatalf("plain improvise cast declined: %+v", got)
				}
				return
			}
			if got.Plan != nil || got.Reason != "unsupported" || got.Detail != tc.detail {
				t.Fatalf("plan outcome = %+v, want unsupported detail %q", got, tc.detail)
			}
		})
	}
}
