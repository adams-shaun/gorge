package cards

import "strings"

// mayask_dredge.go is the text half of the Dredge-after-fill gate
// (MayAskCondDredgeFill): a chain that fills a graveyard and then draws
// (Mental Note's "mill two, draw a card"; a Counter or Destroy link does the
// same through a spell or a creature) meets a Dredge election (CR 702.52a) when
// the card it just put there has Dredge, which the board gate read at the start
// of the resolution cannot see in a graveyard that does not hold it yet. Rules'
// tapeDredgeFillMayAsk settles it from the zones the card comes from.

// graveyardFillAPI reports whether an allowlisted API can put a card into a
// graveyard during the resolution.
func graveyardFillAPI(api string) bool {
	switch api {
	case "Mill", "Destroy", "DestroyAll", "ChangeZone", "Counter":
		return true
	}
	return false
}

// ChainDrawsAfterGraveyardFill reports whether sa's SubAbility$ chain (and a
// Charm's Choices$ bodies) holds a Draw link after a link that can fill a
// graveyard.
func ChainDrawsAfterGraveyardFill(sa *SA, svars map[string]string, depth int) bool {
	if depth > maxMayAskDepth {
		return true
	}
	filled := false
	for s := sa; s != nil; s = s.Sub {
		switch {
		case s.API == "Draw" && filled:
			return true
		case graveyardFillAPI(s.API):
			filled = true
		case s.API == "Charm":
			choices, _ := s.Param(PKChoices)
			for _, n := range strings.Split(choices, ",") {
				if body := ResolveSVar(svars, strings.TrimSpace(n)); body != nil && ChainDrawsAfterGraveyardFill(body, svars, depth+1) {
					return true
				}
			}
		}
	}
	return false
}
