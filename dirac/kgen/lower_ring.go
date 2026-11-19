package kgen

// The Ring (CR 701.54): api RingTemptsYou (api_ring.go) and
// Mode$ RingTemptsYou (gorge trigmatch/actions.go ringTemptsMatches). The
// kernel's half is dirac/act_ring.go.

// ringTemptsTrigger lowers Mode$ RingTemptsYou: a temptation of the trigger's
// controller (ValidPlayer$ You, the corpus's only spelling) whose new
// Ring-bearer matches ValidCard$ (the event's object, the source the Other).
// With no designated bearer a ValidCard$ trigger never fires; the event still
// carries one. TriggerZones$ Battlefield or Graveyard (Ringwraiths) only.
func (lc *lowerCtx) ringTemptsTrigger(tp *params, tr *lTrigger, readF func(string) (*lFilter, error)) error {
	if tr.Zone != zNone && tr.Zone != zBattlefield && tr.Zone != zGraveyard {
		return refuse("RingTemptsYou TriggerZones %q", tp.str("TriggerZones"))
	}
	switch vp := tp.str("ValidPlayer"); vp {
	case "":
		tr.Whose = whoAny
	case "You":
		tr.Whose = whoYou
	default:
		return refuse("RingTemptsYou ValidPlayer %q", vp)
	}
	var err error
	if tr.F, err = readF("ValidCard"); err != nil {
		return err
	}
	tr.Event = "RingTemptsYou"
	return nil
}
