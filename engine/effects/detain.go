package effects

import (
	"maps"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("Detain", effDetain)
}

// detainStatics is the card text Detain puts on each detained permanent
// (CR 701.35a), as the static bodies an Effect hands to the continuous
// registry. One body per Mode$: effEffect's StaticAbilities$ loop parses one
// mode per name, unlike the printed S: route's compound split.
var detainStatics = [...]struct {
	name, body string
}{
	{"GorgeDetainCantAttack", "Mode$ CantAttack | ValidCard$ Card.IsRemembered"},
	{"GorgeDetainCantBlock", "Mode$ CantBlock | ValidCard$ Card.IsRemembered"},
	{"GorgeDetainCantBeActivated", "Mode$ CantBeActivated | ValidCard$ Card.IsRemembered | AffectedZone$ Battlefield"},
}

// effDetain implements DB$ Detain (CR 701.35): each permanent the ability
// detained gets "until your next turn, it can't attack or block and its
// activated abilities can't be activated."
//
// The restriction is delivered the way Forge's DetainEffect does: as a
// Duration$ UntilYourNextTurn Effect whose RememberObjects$ Targeted
// captures the detained set and whose StaticAbilities$ bodies name it through
// Card.IsRemembered. effEffect registers those bodies in the continuous
// registry the combat and activation gates already consult, so no new state,
// event kind or rules-side reader is needed.
func effDetain(h Host, c *Ctx, sa *cards.SA) {
	var detained []state.Target
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			detained = append(detained, t)
		}
	}
	if len(detained) == 0 {
		return
	}
	svars := maps.Clone(c.SVars)
	if svars == nil {
		svars = make(map[string]string, len(detainStatics))
	}
	names := make([]string, 0, len(detainStatics))
	for _, st := range detainStatics {
		svars[st.name] = st.body
		names = append(names, st.name)
	}
	// The Effect's RememberObjects$ Targeted reads the resolution's target
	// list (effectRemembered's refTargetUnion), so the child carries the
	// filtered detained set rather than the outer ability's raw list.
	inner := c.Child(c.Source, c.Controller)
	inner.SVars = svars
	inner.Targets = detained
	effEffect(h, &inner, &cards.SA{
		Kind: "DB",
		API:  "Effect",
		Params: map[string]string{
			"StaticAbilities": strings.Join(names, " "),
			"RememberObjects": "Targeted",
			"Duration":        "UntilYourNextTurn",
		},
	})
}
