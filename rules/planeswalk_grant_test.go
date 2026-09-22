package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestTardisPlaneswalkAskResolvesChain exercises the live Engine resume path
// with the same Optional$ Planeswalk shape used by the real-corpus TARDIS.
// TARDIS is included in the corpus-backed fixture as the carrier being pinned;
// the small probe makes the planeswalk body independently castable without
// depending on the unrelated Crew and attack mechanics.
func TestTardisPlaneswalkAskResolvesChain(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	tardis := lookup(t, reg, "TARDIS")
	realPlaneswalk := cards.ResolveSVar(tardis.Faces[0].SVars, "DBPlaneswalk")
	if realPlaneswalk == nil || realPlaneswalk.API != "Planeswalk" || realPlaneswalk.Params["Optional"] != "True" {
		t.Fatalf("TARDIS DBPlaneswalk = %+v, want Optional$ True Planeswalk", realPlaneswalk)
	}
	probe := card(t, "Name:Planeswalk Probe\nManaCost:G\nTypes:Sorcery\n"+
		"A:SP$ Planeswalk | Optional$ True | SubAbility$ Next\n"+
		"SVar:Next:DB$ GainLife | LifeAmount$ 1\nOracle:x\n")

	for _, want := range []string{"yes", "no"} {
		t.Run(want, func(t *testing.T) {
			e, cfg := corpusEngineCfg(t, reg, []*cards.Card{probe, tardis}, nil)
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
			idx := -1
			for _, option := range d.Options {
				if option.Kind == want {
					idx = option.Index
				}
			}
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
			var elected, noDeck bool
			for _, event := range e.L.Events {
				if event.Kind != events.Note {
					continue
				}
				if event.Text == "planeswalk election: "+want {
					elected = true
				}
				if event.Text == "planeswalk (no planar deck)" {
					noDeck = true
				}
				if strings.HasPrefix(event.Text, "unimplemented API") {
					t.Fatalf("Planeswalk reached generic fallback: %q", event.Text)
				}
			}
			if !elected || (want == "yes" && !noDeck) {
				t.Fatalf("notes election=%v noDeck=%v", elected, noDeck)
			}
			replayCheck(t, e, cfg)
		})
	}
}
