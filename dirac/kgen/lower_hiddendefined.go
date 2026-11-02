package kgen

import "strings"

// hiddenDefinedInert: Hidden$ True on a ChangeZone whose objects a
// Defined$ names is inert in gorge (zone_change.go:418-447, the hidden
// pick needs cz.Defined == ""); consume it. Any other Hidden$ refuses.
func hiddenDefinedInert(ps *params) bool {
	if strings.TrimSpace(ps.m["Hidden"]) != "True" || strings.TrimSpace(ps.m["Defined"]) == "" {
		return false
	}
	ps.consume("Hidden")
	return true
}

// gainControlTrue: a ChangeZone GainControl$ enters the object under the
// resolving controller's control only when it is True (kop.MvUnderYourControl).
// Any other value (ChosenPlayer: Zuko, Conflicted returns under an
// opponent's control) names a controller the kernel's move cannot give, and
// ps.is would have marked it read, so it refuses here.
func gainControlTrue(v string) error {
	if strings.TrimSpace(v) != "True" {
		return refuse("ChangeZone GainControl$ %q", strings.TrimSpace(v))
	}
	return nil
}

// gainControlChosen: GainControl$ ChosenPlayer / Player.Chosen names the one
// opponent while this ability's one-opponent ChoosePlayer is armed
// (chosenPlayerDefined). It lowers only on a Defined$ move to the battlefield
// (Zuko, Conflicted's MoveToPlay): no ChangeType$ pick, no hand/graveyard
// choice, no Defined$ the other helpers (TopOfLibrary, ExiledWith) read. The
// final Return then carries kop.RetUnderOpponent; every other shape refuses.
func (lc *lowerCtx) gainControlChosen(ps *params, gc string, dest int, choice bool) bool {
	if _, ok := lc.chosenPlayerDefined(strings.TrimSpace(gc)); !ok {
		return false
	}
	d := strings.TrimSpace(ps.m["Defined"])
	return dest == zBattlefield && !choice && d != "" && d != "TopOfLibrary" && d != "ExiledWith" &&
		ps.m["ChangeType"] == "" && ps.m["ThisDefinedAndTgts"] == ""
}

// underOpponentShape: the Return that carries kop.RetUnderOpponent acts on
// the resolution's remembered objects and nothing else the kernel's move
// leaves unverified (entry counters share B.N with the flag).
func underOpponentShape(obj lRef, self bool, counterKind string, override *lOverride, transformed bool, cs string) error {
	switch {
	case obj.Kind != rRemembered:
		return refuse("GainControl$ ChosenPlayer of %s", refNames[obj.Kind])
	case self || counterKind != "" || override != nil || transformed:
		return refuse("GainControl$ ChosenPlayer with counters, a static effect, Transformed$ or ThisDefinedAndTgts$")
	case strings.Contains(cs, "Attacking") || strings.Contains(cs, "UnderYourControl"):
		return refuse("GainControl$ ChosenPlayer with Attacking$")
	}
	return nil
}
