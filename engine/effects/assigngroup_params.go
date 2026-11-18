package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// AssignGroupParams is one api:AssignGroup ability's parameters, parsed once
// per resolution here: effAssignGroup reads this struct and never the raw
// Params map.
type AssignGroupParams struct {
	// Choices are the Choices$ SVar names, in script order: the groups a
	// player may be assigned to, and the order their bodies run in.
	Choices []string
	// Chooser is Chooser$ ("" = the resolving controller, which is every
	// corpus carrier).
	Chooser string
	// FriendOrFoe is AILogic$ FriendOrFoe (the Battlebond friend-or-foe
	// cycle): the groups are labelled Friend and Foe in that order.
	FriendOrFoe bool
}

// compileAssignGroup reads an AssignGroup ability's own parameters.
// Defined$ (the player set) is read by the shared Defined-reference tier.
func compileAssignGroup(sa *cards.SA) *AssignGroupParams {
	p := &AssignGroupParams{
		Chooser:     strings.TrimSpace(sa.ParamStr(cards.PKChooser)),
		FriendOrFoe: strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKAILogic)), "FriendOrFoe"),
	}
	for name := range strings.SplitSeq(sa.ParamStr(cards.PKChoices), ",") {
		if name = strings.TrimSpace(name); name != "" {
			p.Choices = append(p.Choices, name)
		}
	}
	return p
}
