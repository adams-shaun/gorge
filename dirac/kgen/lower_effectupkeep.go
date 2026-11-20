package kgen

// pumpUntilUpkeepObject lowers a battlefield object's Pump Duration$
// UntilYourNextUpkeep landwalk grant (Erhnam Djinn's "target non-Wall creature
// an opponent controls gains forestwalk until your next upkeep"): the
// resolving controller and the land filter ride one ovNT override record
// (kop.Override.Landwalk), and kop.Override.UntilUpkeep marks that the end
// point is the next upkeep step rather than the cleanup before the next turn.
// Only the Djinn shape lowers: a P/T pump, a player subject or a non-battlefield
// subject, and any keyword list with a non-landwalk term left over, refuse as
// before.
func (lc *lowerCtx) pumpUntilUpkeepObject(obj lRef, zone int, na, nd bool, kw string, hasKW bool) error {
	if zone != zBattlefield || !lc.pumpObjectRef(obj) || na || nd || !hasKW {
		return refuse("Pump Duration %q", "UntilYourNextUpkeep")
	}
	rest, landwalk, err := lc.splitLandwalkPump(kw) // lower_landwalkgrant.go
	if err != nil {
		return err
	}
	if landwalk == nil || rest != "" {
		return refuse("Pump Duration %q", "UntilYourNextUpkeep")
	}
	lc.emitOverride(obj, &lOverride{Landwalk: landwalk, UntilUpkeep: true}, "UntilYourNextUpkeep")
	return nil
}
