package kgen

import "strings"

func init() { triggerMode("Mutates", trigMutates) }

// trigMutates lowers Mode$ Mutates (CR 702.140f; gorge
// trigmatch.mutatesMatches, kernel dirac/trig_mutates.go). The event's object
// is the surviving mutated permanent, so ValidCard$ is matched against the
// pile, not the trigger's source: Card.Self is the pile being the scanning
// source (build turns it into TfSelf), and Creature.YouCtrl is the lord shape
// (Essence Symbiote), whose `you` reads the listener's controller. Any other
// ValidCard$ has no matching kernel read and refuses by name.
func trigMutates(lc *lowerCtx, a *trigArm) error {
	tp, tr := a.tp, a.tr
	switch v := strings.TrimSpace(tp.str("ValidCard")); v {
	case "Card.Self", "Creature.YouCtrl":
	default:
		return refuse("Mutates ValidCard$ %q", v)
	}
	f, err := a.readF("ValidCard")
	if err != nil {
		return err
	}
	tr.F = f
	tr.Event = "Mutates"
	return nil
}
