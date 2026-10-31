package rules

// RepeatOptional$: the do/while election's iteration count, with a
// body that itself asks, and a stop answer that must not leak into the next
// Repeat on the chain.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRepeatOptionalResumeStatesAreDistinct is the Forbidden Ritual shape
// end to end: a RepeatOptional$ whose body asks (Scry 2, then gain 1). The
// first body runs, then the election for the second iteration is posed --
// exactly once per completed body, never a second body before it -- and a
// yes runs one more body, a no stops. Two bodies run, so two arranges, two
// elections and exactly +2 life.
func TestRepeatOptionalResumeStatesAreDistinct(t *testing.T) {
	t.Parallel()
	spell := "Name:Repeat Ritual\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ Repeat | RepeatSubAbility$ Body | RepeatOptional$ True\n" +
		"SVar:Body:DB$ Scry | Defined$ You | ScryNum$ 2 | SubAbility$ Gain\n" +
		"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg := fixtureGame(t, 81, cardsOfSrcs(t, spell), nil)
	moveNamedTo(t, e, 0, "Repeat Ritual", state.ZHand)
	life := e.G.Players[0].Life
	arranges, elections := 0, 0
	d := castNamedSpell(t, e, "Repeat Ritual", "U", -1)
	for d != nil {
		switch {
		case d.Kind == decision.KArrange:
			arranges++
			if arranges != elections+1 {
				t.Fatalf("body %d ran after %d elections: a body ran before its election", arranges, elections)
			}
			d = answerChoices(t, e, tapePick(d)...)
		case d.Kind == decision.KChoose && d.ResumeKind == "repeat_optional":
			elections++
			if elections != arranges {
				t.Fatalf("election %d posed after %d bodies", elections, arranges)
			}
			if got := e.G.Players[0].Life; got != life+int32(arranges) {
				t.Fatalf("life at election %d = %d, want %d (each completed body gains 1 first)", elections, got, life+int32(arranges))
			}
			ans := optionKindOrFail(t, d, "yes")
			if elections == 2 {
				ans = optionKindOrFail(t, d, "no")
			}
			d = answerChoices(t, e, ans)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	if arranges != 2 || elections != 2 {
		t.Fatalf("bodies = %d, elections = %d, want 2/2", arranges, elections)
	}
	if got := e.G.Players[0].Life; got != life+2 {
		t.Fatalf("life = %d, want %d (exactly two bodies)", got, life+2)
	}
	replayCheck(t, e, cfg)
}

// TestRepeatOptionalContinuationIsConsumedByItsRepeat: a RepeatOptional$
// answered "no" stops only its own Repeat. The chained RepeatNum$ 2 Repeat
// after it still runs both of its iterations.
func TestRepeatOptionalContinuationIsConsumedByItsRepeat(t *testing.T) {
	t.Parallel()
	spell := "Name:Repeat Chain\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ Repeat | RepeatSubAbility$ First | RepeatOptional$ True | SubAbility$ DBSecond\n" +
		"SVar:DBSecond:DB$ Repeat | RepeatSubAbility$ Second | RepeatNum$ 2\n" +
		"SVar:First:DB$ GainLife | Defined$ You | LifeAmount$ 1\n" +
		"SVar:Second:DB$ GainLife | Defined$ You | LifeAmount$ 10\nOracle:x\n"
	e, cfg := fixtureGame(t, 82, cardsOfSrcs(t, spell), nil)
	moveNamedTo(t, e, 0, "Repeat Chain", state.ZHand)
	life := e.G.Players[0].Life
	d := castNamedSpell(t, e, "Repeat Chain", "U", -1)
	if d == nil || d.ResumeKind != "repeat_optional" {
		t.Fatalf("decision = %+v, want the repeat election after the first body", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "no")); d != nil {
		t.Fatalf("the stopped repeat posed %+v", d)
	}
	if got := e.G.Players[0].Life; got != life+21 {
		t.Fatalf("life = %d, want %d (one optional body, then BOTH counted iterations)", got, life+21)
	}
	replayCheck(t, e, cfg)
}
