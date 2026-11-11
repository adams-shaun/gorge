package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// controllerPositionChoice reports ChangeZone's top-or-bottom shape with no
// AlternativeDecider$ and no DestAltSVar$: Destination$ Library at the top
// (LibraryPosition$ absent or 0), DestinationAlternative$ Library and
// LibraryPositionAlternative$ -1 (Not Forgotten's "on the top or bottom of
// its owner's library", Arashin Sovereign's death trigger, Dream Cache's
// "both on top of your library or both on the bottom"). Forge's
// ChangeZoneEffect.handleAltDest leaves altDestOpt true there and asks the
// activator (the alternative decider defaults to the activator) whether the
// cards take the primary destination; a decline takes the alternative
// position, the bottom.
func (cz *ChangeZoneParams) controllerPositionChoice(to state.Zone) bool {
	return cz.AlternativeDecider == "" && cz.DestAltSVarText == "" && to == state.ZLibrary &&
		cz.DestinationAltKnown && cz.DestinationAlt == state.ZLibrary && cz.LibraryPositionAltText == "-1" &&
		(cz.LibraryPositionText == "" || cz.LibraryPositionText == "0")
}

// askLibraryPosition poses the two-option top-or-bottom library position
// decision (the AlternativeDecider$ ask's shape: a "primary" option, then
// "bottom"). answered is false on the no-host path.
func askLibraryPosition(h Host, c *Ctx, sa *cards.SA, chooser state.PlayerID, primaryLabel string) (bottom, answered bool) {
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: c.Source, ResumeKind: "changezone_alternative", ResumeSA: sa,
		Prompt: "Choose your library position",
		Options: []decision.Option{
			{Index: 0, Kind: "primary", Label: primaryLabel},
			{Index: 1, Kind: "bottom", Label: "bottom"},
		}}
	ans, ok := AskTape(h, d)
	if !ok {
		return false, false
	}
	return len(ans) > 0 && ans[0].Label == "bottom", true
}

// controllerPositionAsk is effChangeZone's object-path ask for the
// controllerPositionChoice shape: ONE card, still in an Origin$ zone (a card
// that left it does not move, so nothing is asked), whose controller chooses
// the top or the bottom of its owner's library before it moves. engaged is
// false when nothing was asked; the caller then keeps the primary placement.
func controllerPositionAsk(h Host, c *Ctx, sa *cards.SA, cz *ChangeZoneParams, to state.Zone,
	targets []state.Target, originZones []state.Zone, originAll bool) (bottom, engaged bool) {
	if !cz.controllerPositionChoice(to) || len(targets) != 1 || targets[0].IsPlayer {
		return false, false
	}
	o := h.Game().Obj(targets[0].Obj)
	if o == nil || cz.OriginPresent && !originAll && !zoneIn(originZones, o.Zone) {
		return false, false
	}
	return askLibraryPosition(h, c, sa, c.Controller, "top")
}
