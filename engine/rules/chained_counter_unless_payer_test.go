package rules

// Repulsive Mutation's second half is a chained Counter with its own target
// and "unless its controller pays". CR 118.12's "its controller" is the
// controller of the spell that link targeted, not the controller of the head
// spell (nor its first half's target). The link's own target must therefore
// be bound before the shared UnlessCost$ gate resolves its UnlessPayer$.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// chainedCounterOppSpellSrc is the spell the chained Counter link targets. It
// is put directly on the stack (the same fixture shape Mana Leak's kernel
// test uses), so the only decisions are the cast's own.
const chainedCounterOppSpellSrc = "Name:Chained Opp Spell\nManaCost:1 U\nTypes:Creature Merfolk\nPT:1/1\nOracle:x\n"

func TestChainedCounterUnlessPayerIsCounteredController(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		pay  bool
		want state.Zone
	}{
		{"pay", true, state.ZBattlefield},
		{"decline", false, state.ZGraveyard},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := kr2Engine(t, 2)
			bear := kr2Put(t, e, 0,
				kr2Src(t, "Name:Chained Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
				state.ZBattlefield, false)
			oppSpell := kr2Put(t, e, 1, kr2Src(t, chainedCounterOppSpellSrc), state.ZStack, false)
			mutation := kr2Put(t, e, 0, kr2Corpus(t, "Repulsive Mutation"), state.ZHand, false)
			// The countered spell's controller must be able to pay the toll,
			// or the pay branch is not offered at all.
			kr2Fund(e, 1)

			d := kr2Cast(t, e, 0, mutation)
			var unless *decision.Decision
			sawX, sawRoot, sawLink := false, false, false
			for i := 0; d != nil && i < 40; i++ {
				if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" {
					unless = d
					break
				}
				switch {
				case d.Kind == decision.KChoose && chainedXOption(d) >= 0:
					// X = 1: the first half puts a counter on the Bear, and
					// the unless cost reads the greatest power (3).
					sawX = true
					d = kr2Answer(t, e, d, chainedXOption(d))
				case d.Kind == decision.KTarget && chainedOfferedObj(d, bear) >= 0:
					sawRoot = true
					d = kr2Answer(t, e, d, chainedOfferedObj(d, bear))
				case d.Kind == decision.KTarget && chainedOfferedObj(d, oppSpell) >= 0:
					sawLink = true
					d = kr2Answer(t, e, d, chainedOfferedObj(d, oppSpell))
				default:
					t.Fatalf("unexpected decision while casting Repulsive Mutation: kind=%s resume=%q player=%d options=%+v",
						d.Kind, d.ResumeKind, d.Player, d.Options)
				}
			}
			if !sawX || !sawRoot || !sawLink {
				t.Fatalf("precondition: cast asks X=%v root target=%v chained target=%v, want all true",
					sawX, sawRoot, sawLink)
			}
			if unless == nil {
				t.Fatal("no unless_pay ask for the chained Counter link")
			}
			// The assertion this test exists for: the countered spell's
			// controller (seat 1) pays, never the resolving spell's
			// controller or the first half's target's controller (seat 0).
			if unless.Player != 1 {
				t.Fatalf("unless_pay payer = seat %d, want the countered spell's controller (seat 1), not seat %d",
					unless.Player, unless.Player)
			}
			// The chained counter has not applied yet: the target spell is
			// still on the stack when the payer is asked.
			if z := e.G.Obj(oppSpell).Zone; z != state.ZStack {
				t.Fatalf("target spell zone at the ask = %s, want Stack", z)
			}
			want := decision.ModeUnlessDecline
			if tc.pay {
				want = decision.ModeUnlessPay
			}
			pick := -1
			for _, o := range unless.Options {
				if o.Mode == want {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("unless ask offers no %q option: %+v", want, unless.Options)
			}
			d = kr2Answer(t, e, unless, pick)
			// A mana-only toll with a funded pool charges without opening the
			// CR 601.2g window; tolerate the window if one is posed anyway.
			for i := 0; d != nil && d.ResumeKind == unlessManaKind && i < 10; i++ {
				done := len(d.Options) - 1
				for _, o := range d.Options {
					if o.Kind == "done" {
						done = o.Index
					}
				}
				d = kr2Answer(t, e, d, done)
			}
			passUntilStackEmpty(t, e, 20)

			if z := e.G.Obj(oppSpell).Zone; z != tc.want {
				t.Fatalf("%s: countered spell zone = %s, want %s", tc.name, z, tc.want)
			}
			// The head half really resolved: the Bear got the counter.
			if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
				t.Fatalf("head half: Bear +1/+1 counters = %d, want 1", got)
			}
		})
	}
}

// chainedXOption returns the index of d's X=1 option, or -1.
func chainedXOption(d *decision.Decision) int {
	if d.Kind != decision.KChoose {
		return -1
	}
	for _, o := range d.Options {
		if o.Kind == "x" && o.Amount == 1 {
			return o.Index
		}
	}
	return -1
}

// chainedOfferedObj returns the index of d's option naming obj, or -1.
func chainedOfferedObj(d *decision.Decision, obj state.ObjID) int {
	for _, o := range d.Options {
		if o.Obj == obj {
			return o.Index
		}
	}
	return -1
}
