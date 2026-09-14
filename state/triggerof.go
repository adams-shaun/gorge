package state

import "github.com/adams-shaun/gorge/cards"

// TriggerOf reports the T: trigger line on o's source face that minted the
// ability stack object o. One classifier, two consumers that must agree:
// view/view.go's StackView.Kind "trigger" (an object minted by TriggerPush)
// vs "ability" (any other ability object), and rules' TargetType$ target
// legality, which needs the same triggered/activated split to know whether a
// `TargetType$ Triggered` spec may target the object. An object that is not
// an ability wrapper, has no source, or whose source's current face lists no
// trigger carrying exactly this Effect returns false -- the caller decides
// what a false means (the view renders "ability"; rules' classifier falls
// through to its activated/delayed branches).
func TriggerOf(g *Game, o *Object) (cards.Trigger, bool) {
	if o == nil || o.Ability == nil {
		return cards.Trigger{}, false
	}
	src := g.Obj(o.Source)
	if src == nil {
		return cards.Trigger{}, false
	}
	f := src.Face()
	if f == nil {
		return cards.Trigger{}, false
	}
	for _, t := range f.Triggers {
		if t.Effect == o.Ability {
			return t, true
		}
	}
	return cards.Trigger{}, false
}
