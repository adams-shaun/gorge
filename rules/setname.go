package rules

import (
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// This file is the ONE bridge between rules' layer walk and the effects tier's
// name comparisons (Forge's named<X>, notnamed<X>, sameName and NamedCard).
//
// A layer-3 SetName$ effect (CR 613.1d) replaces a permanent's name, and only
// the layer walk can say which effect applies and which of two competing
// effects wins on timestamp. The effects tier cannot import rules and must not
// re-derive any of that from a battlefield scan. So rules computes the renames
// itself and hands them down as DATA on the SpecContext the filter call already
// carries -- the same shape SpecContext.ExtraTypes uses for layer-4 types, and
// deliberately not a callable resolver field (which poisons escape analysis) or
// a back-pointer from state.Game into a live engine (which would make an
// authoritative Game depend on something outside its event fold, and would
// survive a Game.Clone aimed at a different board).
//
// A filter call that rules did not build -- effects.MatchesSpecFrom from inside
// a resolving effect, a unit test, a bare *state.Game -- carries no names and
// reads the printed face. That is the same reach ExtraTypes has (see the
// "Layer-4 type grants reach only the layer walk" row in AGENTS.md) and it is
// pinned by TestBareGameNameFilterReadsThePrintedName.

// effectiveNames returns the layer-3 renames in force on the battlefield, as
// the immutable value slice a SpecContext carries. It is nil on every board
// with no SetName$ effect active, which is almost every board: the active()
// scan below is a cheap pass over an already-cached, already-sorted list.
//
// The result is cached under active()'s own key -- the log head plus the
// continuous-effect version -- because the rename set is a pure function of
// exactly those two inputs. Clone copies none of these fields, so a cloned
// engine rebuilds its own from its own game.
func (e *Engine) effectiveNames() []effects.ObjectName {
	if e.renameBuilding {
		// Re-entry: a Derived() call below reached a filter that asked for the
		// names it is in the middle of computing. The half-built answer must
		// never escape, so the inner read falls back to printed names -- the
		// same narrowing the layer walk's own Affected$ filters already have.
		return nil
	}
	if e.renameEpoch == len(e.L.Events) && e.renameVersion == e.continuousVersion {
		return e.renameBuf
	}
	e.renameEpoch, e.renameVersion = len(e.L.Events), e.continuousVersion
	buf := e.renameBuf[:0]
	if !e.anySetNameActive() {
		e.renameBuf = buf
		return nil
	}
	e.renameBuilding = true
	defer func() { e.renameBuilding = false }()
	// e.G.Objs is append-ordered, so this walk is deterministic; only the
	// battlefield is scanned because a layer effect applies nowhere else.
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield {
			continue
		}
		f := o.Face()
		if f == nil {
			continue
		}
		name := e.Derived(o.ID).Name
		if name == "" || name == f.Name {
			continue
		}
		buf = append(buf, effects.ObjectName{ID: o.ID, Name: name})
	}
	e.renameBuf = buf
	return buf
}

// anySetNameActive reports whether any active continuous effect sets a name.
// Without this gate every SpecContext construction would pay for a full
// battlefield Derived() walk.
func (e *Engine) anySetNameActive() bool {
	for _, ce := range e.active() {
		if ce.Layer == LText && ce.SetName != "" {
			return true
		}
	}
	return false
}

// withNames binds the current renames on a SpecContext. Every rules-built
// SpecContext goes through it, so a name filter rules evaluates agrees with
// the layer walk rules and view render from.
func (e *Engine) withNames(sc effects.SpecContext) effects.SpecContext {
	sc.EffectiveNames = e.effectiveNames()
	return sc
}
