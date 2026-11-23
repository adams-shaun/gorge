package effects

import "github.com/adams-shaun/gorge/engine/state"

// captureRememberedLKIPT records the derived battlefield power and toughness
// of an object a RememberLKI$ ChangeZone is about to move off the battlefield
// (CR 608.2h), before the move removes the continuous effects that shaped them.
// The reflexive "when you do" half of Yannik, Scavenging Sentinel reads
// "the exiled creature's power" (TriggerRemembered$CardPower) from it: the
// exiled card itself only has its printed power, and an Aura it wore has
// already fallen off. An entry already held is kept (a re-entry or an earlier
// capture is the older, truer one), as captureTargetPT does.
func captureRememberedLKIPT(h Host, c *Ctx, o *state.Object) {
	if c == nil || o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		return
	}
	if _, held := c.Snap.TargetPT[o.ID]; held {
		return
	}
	if c.Snap.TargetPT == nil {
		c.Snap.TargetPT = make(map[state.ObjID]TargetPT)
	}
	c.Snap.TargetPT[o.ID] = TargetPT{Power: h.Power(o.ID), Toughness: h.Toughness(o.ID),
		Tapped: o.Tapped, Attacking: o.IsAttacking}
}
