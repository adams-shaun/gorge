package effects

import "github.com/adams-shaun/gorge/engine/state"

// departedSourceLKI reports whether c carries the last-known characteristics
// of a source that has left the battlefield since the ability was created
// (rules' bindSourceLKI, CR 113.7a / 608.2h). A source still on the
// battlefield as the object that put the ability on the stack is read live; one
// that left and came back is a new object (CR 400.7), so the ability still
// reads the old one's last known information (an undying Gleeful Arsonist's
// "damage equal to its power" trigger, the Arsonist killed and returned in
// response).
func (c *Ctx) departedSourceLKI(g *state.Game) bool {
	if !c.TriggerContext.SourceLKIValid {
		return false
	}
	o := g.Obj(c.Source)
	if o == nil || o.Zone != state.ZBattlefield {
		return true
	}
	return c.sourceReturnedSincePush(g, o)
}

// sourceReturnedSincePush reports whether o, the ability's source now on the
// battlefield, is a later incarnation than the one the resolving ability was
// put on the stack with.
func (c *Ctx) sourceReturnedSincePush(g *state.Game, o *state.Object) bool {
	if c.ResolvingObj == 0 {
		return false
	}
	ro := g.Obj(c.ResolvingObj)
	if ro == nil || ro.Source != o.ID || !ro.PushSourceStamped || !ro.PushSourceOnBattlefield {
		return false
	}
	return o.Incarnation != ro.PushSourceIncarnation
}
