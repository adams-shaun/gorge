package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// timeLordSrc is a freely-authored Time Lord creature standing in for the
// intervening-if condition TARDIS's own script gates on
// (IsPresent$ Card.Time Lord+YouCtrl). The carrier is TARDIS; the
// creature that satisfies the gate is a fixture.
const timeLordSrc = "Name:Time Lord Fixture\nManaCost:1 U\nTypes:Legendary Creature Time Lord\nPT:1/1\nOracle:x\n"

// TestPlaneswalkProbeAskResolvesChain is the synthetic complement to the
// carrier pin: the same Optional$ Planeswalk shape on an authored probe,
// cast through the ordinary cast path so the mid-resolution election and the
// chained SubAbility after it are exercised independently of TARDIS's
// unsupported Crew/attack mechanics.
func TestPlaneswalkProbeAskResolvesChain(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	probe := card(t, "Name:Planeswalk Probe\nManaCost:G\nTypes:Sorcery\n"+
		"A:SP$ Planeswalk | Optional$ True | SubAbility$ Next\n"+
		"SVar:Next:DB$ GainLife | LifeAmount$ 1\nOracle:x\n")

	for _, want := range []string{"yes", "no"} {
		t.Run(want, func(t *testing.T) {
			e, cfg := corpusEngineCfg(t, reg, []*cards.Card{probe}, nil)
			probeID := moveByName(t, e, 0, "Planeswalk Probe", state.ZHand)
			if e.G.Obj(probeID).Zone != state.ZHand {
				t.Fatalf("probe is in %s, want hand", e.G.Obj(probeID).Zone)
			}
			life := e.G.Players[0].Life
			addMana(t, e, 0, "G")
			d := castFixture(t, e, probeID, -1)
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "planeswalk_optional" {
				t.Fatalf("planeswalk decision = %+v", d)
			}
			idx := optionIndexForKind(d, want)
			if idx < 0 {
				t.Fatalf("no %q option in %+v", want, d.Options)
			}
			submitChoices(t, e, idx)
			passUntilStackEmpty(t, e, 40)
			if e.G.Obj(probeID).Zone != state.ZGraveyard {
				t.Fatalf("probe is in %s, want graveyard", e.G.Obj(probeID).Zone)
			}
			if e.G.Players[0].Life != life+1 {
				t.Fatalf("chained GainLife did not resolve: life %d, want %d", e.G.Players[0].Life, life+1)
			}
			replayCheck(t, e, cfg)
		})
	}
}
