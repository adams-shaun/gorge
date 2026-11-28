package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// lithoformProbeScript is Nahiri's Lithoforming's shape on an {X}{R}{R}
// sorcery: sacrifice X lands, draw one card per land sacrificed, then an
// Effect that freezes X as its chosen number, grants that many additional
// land plays this turn and makes the controller's lands enter tapped this
// turn.
const lithoformProbeScript = "Name:Lithoform Probe\nManaCost:X R R\nTypes:Sorcery\n" +
	"A:SP$ Sacrifice | SacValid$ Land | Amount$ PX | RememberSacrificed$ True | SubAbility$ ProbeDrawN | SpellDescription$ Sacrifice X lands and draw that many; play X more lands this turn, and your lands enter tapped this turn.\n" +
	"SVar:ProbeDrawN:DB$ Draw | NumCards$ PN | SubAbility$ ProbeLandEffect\n" +
	"SVar:ProbeLandEffect:DB$ Effect | SetChosenNumber$ PX | StaticAbilities$ ProbeMoreLands | ReplacementEffects$ ProbeLandsTapped | SubAbility$ ProbeForgetAll\n" +
	"SVar:ProbeMoreLands:Mode$ Continuous | Affected$ You | AdjustLandPlays$ PZ | Description$ Play X more lands this turn.\n" +
	"SVar:ProbeLandsTapped:Event$ Moved | ValidCard$ Land.YouCtrl | Destination$ Battlefield | ReplaceWith$ ProbeTapIt | ReplacementResult$ Updated | Description$ Your lands enter tapped this turn.\n" +
	"SVar:ProbeTapIt:DB$ Tap | ETB$ True | Defined$ ReplacedCard\n" +
	"SVar:ProbeForgetAll:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:PX:Count$xPaid\nSVar:PN:Count$RememberedSize\nSVar:PZ:Count$ChosenNumber\n" +
	"Oracle:Sacrifice X lands and draw that many. You may play X more lands this turn. Your lands enter tapped this turn.\n"

// lithoformTappedAndExtra: over the whole game, after a Lithoform Probe
// resolved, a land entered the battlefield and was tapped as it entered (the
// emblem's replacement), a Stomping Ground that entered so was then asked its
// own pay-life election (gorge taps first and asks after, with no CR 616.1
// order ask), and one player played two lands in that turn (the
// frozen-number extra land play).
func lithoformTappedAndExtra(g *GorgeSide) bool {
	resolved := false // a probe resolved this turn
	played := [2]int{}
	tappedIn := map[state.ObjID]bool{} // lands tapped as they entered after a probe resolved
	tapped, shock, extra := false, false, false
	evs := g.E.L.Events
	for i := range evs {
		e := &evs[i]
		switch e.Kind {
		case events.TurnChange:
			resolved, played = false, [2]int{}
		case events.MoveZone:
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Lithoform Probe" && e.From == state.ZStack {
				resolved = true
				continue
			}
			if !resolved || e.To != state.ZBattlefield || !slices.Contains(o.Face().Types, "Land") {
				continue
			}
			if i+1 < len(evs) && evs[i+1].Kind == events.Tap && evs[i+1].Obj == e.Obj {
				tapped = true
				tappedIn[e.Obj] = true
			}
		case events.ModeChosen:
			if o := g.E.G.Obj(e.Obj); tappedIn[e.Obj] && o != nil && o.Face() != nil && o.Face().Name == "Stomping Ground" {
				shock = true
			}
		case events.LandPlayed:
			if p := int(e.Player); p >= 0 && p < 2 {
				played[p]++
				if resolved && played[p] >= 2 {
					extra = true
				}
			}
		}
	}
	return tapped && shock && extra
}

// TestHostedCmdrLithoformingLockstep plays the frozen-number land plays and
// the emblem lands-enter-tapped replacement decision by decision against
// gorge: seat 0 casts Lithoform Probes over Mountains and Stomping Grounds
// (a shock land's own pay-life election under the emblem), seat 1 attacks
// with Grizzly Bears.
func TestHostedCmdrLithoformingLockstep(t *testing.T) {
	probeDeck := append(deckOf(8, "Lithoform Probe"), deckOf(8, "Stomping Ground")...)
	probeDeck = append(probeDeck, deckOf(24, "Mountain")...)
	oppDeck := append(deckOf(6, "Grizzly Bears"), deckOf(34, "Forest")...)
	runHostedCmdrShape(t, hostedCmdrShape{
		hostedCmdrCase: hostedCmdrCase{
			name:     "lithoform_probe",
			carrier:  "Lithoform Probe",
			seats:    [2]string{"hcs-lithoform-a", "hcs-lithoform-b"},
			decks:    [2][]string{probeDeck, oppDeck},
			fixtures: map[string]string{"Lithoform Probe": lithoformProbeScript},
		},
		real:    []string{"Mountain", "Stomping Ground", "Forest", "Grizzly Bears"},
		reached: lithoformTappedAndExtra,
	})
}
