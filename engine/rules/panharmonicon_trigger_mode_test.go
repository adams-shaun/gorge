package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// trigModeEchoSrc doubles what a land entering sets off, but only the abilities
// whose own Mode$ is a zone change.
const trigModeEchoSrc = `Name:Mode Echo
ManaCost:2
Types:Artifact
S:Mode$ Panharmonicon | ValidMode$ ChangesZone,ChangesZoneAll | ValidCard$ Permanent.YouCtrl | ValidCause$ Land | Destination$ Battlefield | Description$ If a land entering sets off an ability of a permanent you control, it happens an extra time.
Oracle:x
`

// trigModeAnyEchoSrc names no ValidMode$: it doubles every ability of the
// permanent, whatever its mode.
const trigModeAnyEchoSrc = `Name:Mode Echo Any
ManaCost:2
Types:Artifact
S:Mode$ Panharmonicon | ValidCard$ Permanent.YouCtrl | Description$ Any ability of a permanent you control happens an extra time.
Oracle:x
`

// trigModeGaugeSrc has a land-play ability and a zone-change ability, each a
// gain of 1 life, so the count of each shows up in the life total.
const trigModeGaugeSrc = `Name:Mode Gauge
ManaCost:1
Types:Artifact
T:Mode$ LandPlayed | ValidCard$ Land.YouCtrl | Execute$ PlayedLife | TriggerZones$ Battlefield
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Land.YouCtrl | Execute$ EnteredLife | TriggerZones$ Battlefield
SVar:PlayedLife:DB$ GainLife | LifeAmount$ 1
SVar:EnteredLife:DB$ GainLife | LifeAmount$ 1
Oracle:x
`

// TestPanharmoniconDoublesOnlyTheListedTriggerModes pins CR 603.2d as Forge
// reads it: a static's ValidMode$ lists trigger modes, and the doubled trigger's
// own Mode$ must be one of them. One land entering from the hand causes both a
// Mode$ LandPlayed ability and a Mode$ ChangesZone ability, but a ChangesZone
// static doubles only the second.
func TestPanharmoniconDoublesOnlyTheListedTriggerModes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	played := func(t *testing.T, echo string) int32 {
		t.Helper()
		extras := []*cards.Card{card(t, trigModeGaugeSrc)}
		if echo != "" {
			extras = append(extras, card(t, echo))
		}
		e := corpusEngine(t, reg, extras, nil)
		if echo != "" {
			onBoardCard(t, e, 0, card(t, echo))
		}
		onBoardCard(t, e, 0, card(t, trigModeGaugeSrc))
		before := e.G.Players[0].Life
		land := playLandFromHand(t, e, "Mountain")
		answerQuiet(t, e, 60)
		if z := e.G.Obj(land).Zone; z != state.ZBattlefield {
			t.Fatalf("precondition: land in %v, want battlefield", z)
		}
		return e.G.Players[0].Life - before
	}

	if got := played(t, ""); got != 2 {
		t.Fatalf("baseline: the land set off abilities worth %d life, want 2 (one LandPlayed, one ChangesZone)", got)
	}
	if got := played(t, trigModeEchoSrc); got != 3 {
		t.Fatalf("ChangesZone static: %d life, want 3 -- only the ChangesZone ability doubles, the LandPlayed one stays single", got)
	}
	if got := played(t, trigModeAnyEchoSrc); got != 4 {
		t.Fatalf("static without ValidMode$: %d life, want 4 -- every ability of the permanent doubles", got)
	}
}
