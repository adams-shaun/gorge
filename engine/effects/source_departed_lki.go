package effects

import "github.com/adams-shaun/gorge/engine/state"

// departedSourceLKI reports whether c carries the last-known characteristics
// of a source that has left the battlefield since the ability was created
// (rules' bindSourceLKI, CR 113.7a / 608.2h). A source still, or again, on
// the battlefield is read live.
func (c *Ctx) departedSourceLKI(g *state.Game) bool {
	if !c.TriggerContext.SourceLKIValid {
		return false
	}
	o := g.Obj(c.Source)
	return o == nil || o.Zone != state.ZBattlefield
}
