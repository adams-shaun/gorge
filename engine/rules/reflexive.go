package rules

import (
	"maps"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Reflexive triggered abilities (CR 603.12): "Create two tokens. When you do,
// tap target creature an opponent controls." The "when you do" half is a
// triggered ability that triggers during the resolution that spawned it and
// goes on the stack -- with its own targets, respondable, firing ward and
// "becomes the target" triggers -- the next time a player would receive
// priority.
//
// The spawning primitive is effects' ImmediateTrigger. It used to resolve its
// Execute$ body inline, inside the spawning resolution, which made the target
// an untargeted resolution-time choice and gave nobody a window between the
// two halves. It now queues one pendingTrigger per instance here.
//
// No new event kind and no registration: the entry is the delayed-shape
// pendingTrigger with DelayedID -1, the shape a Saga chapter, a dungeon room
// and a Room's unlock trigger already queue (engine/rules/saga.go). pushTrigger mints
// it through a DelayedPush whose Counter is the Execute$ SVar name, which
// events.Apply resolves from the source object's own SVar tables, and poses
// the ordinary CR 603.3c mode/target placement asks for it. The instance's
// remembered set rides the event's IDs.

// QueueReflexiveTrigger implements effects.Host.
//
// It reports false -- the caller then resolves the body inline, exactly as
// before -- unless a log-only replay can mint the SAME body: events.Apply
// resolves the Execute$ name from the source object's faces, so the resolving
// chain's SVar table must be the source's own current-face table (a gained or
// granted body compiled on a foreign face is not; the chain's table is a
// defensive copy, effects.SetSVars, so contents are compared, not identity)
// and the name must resolve there to the body being queued.
func (e *Engine) QueueReflexiveTrigger(c *effects.Ctx, execute string, body *cards.SA, remembered []state.Target) bool {
	if c == nil || body == nil || execute == "" {
		return false
	}
	src := e.G.Obj(c.Source)
	if src == nil || src.Face() == nil {
		return false
	}
	// A body a static granted lives on the granting object's table, not the
	// source's (Sunfire Torch's "when you do, this creature deals 2 damage"):
	// the reflexive ability is minted the way the granted trigger itself was,
	// from the grantor (CR 603.12, 613.1f).
	tableOwner, granted := src, false
	if g := e.G.Obj(c.Grantor); c.Grantor != 0 && g != nil && g.Face() != nil {
		tableOwner, granted = g, true
	}
	if !reflexiveTableIsSources(c, tableOwner) {
		return false
	}
	sa := events.ResolveSVarAcrossFaces(tableOwner, execute)
	if sa == nil || sa.Line != body.Line {
		return false
	}
	tc := c.TriggerContext
	tc.Reflexive = true
	tc.SpawnerCaptured = append([]state.Target(nil), c.Captured...)
	// The spawning resolution's delayed-registration capture is not this
	// ability's: DelayTriggerRemembered names the instance's own set.
	tc.DelayedRemembered = nil
	tc.OptionalSpec = ""
	// CR 107.3: "you may pay {X}. When you do, ... X ..." -- the reflexive
	// ability's X is the value the spawning ability was paid or announced
	// with (triggerPaidX reads it back for the minted ability).
	if c.X != 0 || c.XAnnounced {
		tc.TriggerPaidX = c.X
	}
	pt := pendingTrigger{
		Source:     c.Source,
		Controller: c.Controller,
		Delayed:    true,
		DelayedID:  ^uint32(0),
		Execute:    execute,
		SA:         sa,
		Ctx:        c.ForTrigger(tc),
	}
	if granted {
		pt.Delayed, pt.DelayedID = false, 0
		pt.Granted, pt.Grantor = true, c.Grantor
	}
	pt.Ctx.Remembered = append([]state.Target(nil), remembered...)
	pt.Ctx.Snap.TargetPT = rememberedPTLKI(e, e.G, pt.Ctx.Remembered)
	e.pendingTriggers = append(e.pendingTriggers, pt)
	return true
}

// bindReflexiveContext carries what the spawning resolution knew onto the
// minted reflexive ability, in the engine maps resolveTop already reads per
// stack object (the same replay-derived maps a face trigger's push fills):
// the spawning trigger's LKI snapshot ("equal to its power" on a dies
// trigger's reflexive half), the damage-source LKI flags, and the lists the
// spawning ability PAID (Sacrificed$/Exiled$/Revealed$ heads).
func (e *Engine) bindReflexiveContext(id state.ObjID, pt *pendingTrigger) {
	if !pt.Ctx.TriggerContext.Reflexive {
		return
	}
	e.keepTriggerLKI(id, pt)
	if pt.Ctx.Snap.TargetPT != nil {
		if e.reflexiveRememberedPT == nil {
			e.reflexiveRememberedPT = make(map[state.ObjID]map[state.ObjID]effects.TargetPT)
		}
		e.reflexiveRememberedPT[id] = effects.CloneTargetPTLKI(pt.Ctx.Snap.TargetPT)
	}
	if pt.Ctx.Snap.SourceLifelinkValid {
		if e.sourceLifelinkLKI == nil {
			e.sourceLifelinkLKI = make(map[state.ObjID]bool)
		}
		e.sourceLifelinkLKI[id] = pt.Ctx.Snap.SourceLifelink
	}
	if pt.Ctx.Snap.SourceControllerValid {
		if e.sourceControllerLKI == nil {
			e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
		}
		e.sourceControllerLKI[id] = pt.Ctx.Snap.SourceController
	}
	if len(pt.Ctx.Sacrificed) > 0 {
		if e.sacrificedLKI == nil {
			e.sacrificedLKI = make(map[state.ObjID][]state.SacrificedInfo)
		}
		e.sacrificedLKI[id] = pt.Ctx.Sacrificed
	}
	if len(pt.Ctx.Paid.Exiled) > 0 {
		if e.castExiled == nil {
			e.castExiled = make(map[state.ObjID][]state.ObjID)
		}
		e.castExiled[id] = pt.Ctx.Paid.Exiled
	}
	if len(pt.Ctx.Paid.Revealed) > 0 {
		if e.castRevealed == nil {
			e.castRevealed = make(map[state.ObjID][]state.ObjID)
		}
		e.castRevealed[id] = pt.Ctx.Paid.Revealed
	}
	if len(pt.Ctx.Paid.Discarded) > 0 {
		if e.castDiscarded == nil {
			e.castDiscarded = make(map[state.ObjID][]state.ObjID)
		}
		e.castDiscarded[id] = pt.Ctx.Paid.Discarded
	}
	if len(pt.Ctx.Paid.CounterPut) > 0 {
		if e.castCounterPut == nil {
			e.castCounterPut = make(map[state.ObjID][]state.ObjID)
		}
		e.castCounterPut[id] = pt.Ctx.Paid.CounterPut
	}
	if len(pt.Ctx.Paid.Tapped) > 0 {
		if e.castTapped == nil {
			e.castTapped = make(map[state.ObjID][]state.ObjID)
		}
		e.castTapped[id] = pt.Ctx.Paid.Tapped
	}
	if len(pt.Ctx.Paid.Returned) > 0 {
		if e.castReturned == nil {
			e.castReturned = make(map[state.ObjID][]state.ObjID)
		}
		e.castReturned[id] = pt.Ctx.Paid.Returned
	}
}

// ptReader is the derived power/toughness read rememberedPTLKI needs.
type ptReader interface {
	Power(state.ObjID) int32
	Toughness(state.ObjID) int32
}

// rememberedPTLKI is the last known power/toughness (CR 608.2h) of each
// remembered object on the battlefield as a reflexive ability is put on the
// stack, in a fresh map (never the spawning Ctx's, whose map headers a
// shallow Ctx copy shares); nil when none is there. Taken at that instant,
// not at a later departure: a pump between the two is not seen (the kernel's
// reflexive capture is the same instant).
func rememberedPTLKI(h ptReader, g *state.Game, remembered []state.Target) map[state.ObjID]effects.TargetPT {
	var out map[state.ObjID]effects.TargetPT
	for _, t := range remembered {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		if out == nil {
			out = make(map[state.ObjID]effects.TargetPT)
		}
		out[t.Obj] = effects.TargetPT{Power: h.Power(t.Obj), Toughness: h.Toughness(t.Obj),
			Tapped: o.Tapped, Attacking: o.IsAttacking}
	}
	return out
}

// reflexiveCaptured rebinds a reflexive ability's resolution Ctx.Captured to
// the spawning ability's capture: resolveTop seeds Captured from the stack
// object's Remembered, which for a reflexive ability is the instance's own
// remembered set (real memory the body reads through Remembered /
// TriggerRemembered, never to be excluded as a capture).
func reflexiveCaptured(ctx *effects.Ctx) {
	if ctx.TriggerContext.Reflexive {
		ctx.Captured = append([]state.Target(nil), ctx.TriggerContext.SpawnerCaptured...)
	}
}

// reflexiveTableIsSources reports whether the resolving chain's SVar table is
// the source's own: its current face's, or -- when the source is a copy of
// another object (Superior Spider-Man's as-enters copy runs its "When you do,
// exile that card" after the copy basis replaced the face) -- one of its own
// printed faces', which events.ResolveSVarAcrossFaces falls back to.
func reflexiveTableIsSources(c *effects.Ctx, src *state.Object) bool {
	if svarTableIsFaces(c, src.Face()) {
		return true
	}
	if src.LeftCopyFace != nil && svarTableIsFaces(c, src.LeftCopyFace) {
		// A copy sacrificed to its own cost: the table is the copied face's
		// (events.ResolveSVarAcrossFaces searches LeftCopyFace on replay).
		return true
	}
	if src.Card == nil {
		return false
	}
	for _, cf := range src.Card.Faces {
		if svarTableIsFaces(c, cf) {
			return true
		}
	}
	return false
}

// svarTableIsFaces reports whether c's table is face's SVar table, allowing the
// resolution's published scratch bindings on top of it: ExcessSVar$ (Rhino's
// Rampage, Bolg) binds an integer under a name the face never declares, and
// that binding must not send the "when you do" half down the inline path
// (where a targetless body would act on the spawner's targets). A script body
// is never a bare integer, so a foreign table's extra SVars still fail.
func svarTableIsFaces(c *effects.Ctx, face *cards.Face) bool {
	if maps.Equal(c.SVars, face.SVars) {
		return true
	}
	for k, v := range face.SVars {
		if tv, ok := c.SVars[k]; !ok || tv != v {
			return false
		}
	}
	for k, v := range c.SVars {
		if _, declared := face.SVars[k]; declared {
			continue
		}
		if v == "" || strings.TrimLeft(v, "0123456789") != "" {
			return false
		}
	}
	return true
}
