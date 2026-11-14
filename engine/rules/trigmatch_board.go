package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
)

// trigBoard is the engine as trigmatch.Board, the read-only view the trigger
// matchers in engine/rules/trigmatch receive. It is a defined type over
// Engine rather than a set of Engine methods so the adapter adds nothing to
// Engine's method set and cannot collide with an Engine method of the same
// name; boardOf(e) is a free pointer conversion (a function, not an Engine
// method, for the same reason), and converting the *trigBoard to the
// interface does not allocate.
//
// Every method is a query that forwards to the engine read it names. None
// emits or writes game state.
type trigBoard Engine

var _ trigmatch.Board = (*trigBoard)(nil)

// boardOf is e as the trigger matchers' read-only view.
func boardOf(e *Engine) *trigBoard { return (*trigBoard)(e) }

func (b *trigBoard) eng() *Engine { return (*Engine)(b) }

func (b *trigBoard) Game() *state.Game { return b.G }
func (b *trigBoard) Log() *events.Log  { return b.L }

func (b *trigBoard) EvalCount(source state.ObjID, controller state.PlayerID, svars map[string]string, expr string) (int32, bool) {
	return effects.EvalCountOK(b.eng(), effects.NewCtxPtr(source, controller, effects.CtxInit{SVars: svars}), expr)
}

func (b *trigBoard) Facts() trigmatch.Facts {
	return trigmatch.Facts{
		Damaging:               b.damaging,
		CombatDamaging:         b.combatDamaging,
		DmgSrcOverride:         b.dmgSrcOverride,
		DeclaredAttackers:      b.declaredAttackers,
		TapObj:                 b.tapObj,
		TapPlayer:              b.tapPlayer,
		TapEntering:            b.tapEntering,
		TappingForMana:         b.tappingForMana,
		TappingManaProduced:    b.tappingManaProduced,
		GoadProbe:              b.goadProbe,
		LifeLossBatch:          b.lifeLossBatch,
		FinishingLifeLossBatch: b.finishingLifeLossBatch,
		TappedTurn:             b.tappedTurn,
		PlayingLand:            b.playingLand(),
	}
}

// DepartedSourceController reads the stack top's captured last-known
// controller of src (engine_damage.go captures it when src leaves).
func (b *trigBoard) DepartedSourceController(src state.ObjID) (state.PlayerID, bool) {
	e := b.eng()
	if len(e.G.Stack) == 0 {
		return 0, false
	}
	top := e.G.Stack[len(e.G.Stack)-1]
	if l, ok := e.damageSourceLKI[top][src]; ok {
		return l.Controller, true
	}
	if o := e.G.Obj(top); o != nil && o.Source == src {
		c, ok := e.sourceControllerLKI[top]
		return c, ok
	}
	return 0, false
}

func (b *trigBoard) ControllerOf(id state.ObjID) state.PlayerID { return b.eng().controllerOf(id) }

func (b *trigBoard) EffectMatchControllerFor(id state.ObjID) (state.PlayerID, bool) {
	return b.eng().effectMatchControllerFor(id)
}

func (b *trigBoard) Chars(id state.ObjID) *effects.Chars { return b.eng().Chars(id) }
func (b *trigBoard) Power(id state.ObjID) int32          { return b.eng().Power(id) }
func (b *trigBoard) Toughness(id state.ObjID) int32      { return b.eng().Toughness(id) }

func (b *trigBoard) FaceDownPrintedHides(o *state.Object) bool {
	return b.eng().faceDownPrintedHides(o)
}

func (b *trigBoard) PlayerSpecCtx(source state.ObjID) effects.PlayerSpecCtx {
	return b.eng().playerSpecCtx(source)
}

// MatchesSpec and MatchesObject build the filter context here, where it stays
// on the stack (specCtx inlines and matchesSpec / MatchesObjectCtx do not
// leak it), and apply the matcher's overrides to it.
func (b *trigBoard) MatchesSpec(spec string, id, source state.ObjID, you state.PlayerID, o trigmatch.SpecOpts) bool {
	e := b.eng()
	sc := e.specCtx(source, you)
	sc.DelayedRemembered, sc.ExtraTypes, sc.Layers.StaticGoads = o.DelayedRemembered, o.ExtraTypes, o.StaticGoads
	// A registration's frozen chosen-card set IS the chosen list its body's
	// ChosenCard(Strict) predicate reads (state.DelayedTrigger.ChosenCard,
	// Forge's Effect card), bound like DelayedRemembered is.
	if o.DelayedChosenBound {
		sc.Chosen, sc.ChosenValid = o.DelayedChosen, true
	}
	sc.ExtraTypesOwner = id
	if o.TargetsBound {
		sc.Resolving, sc.ResolutionTargets = true, o.Targets
	}
	// ExtraTypes is the caller's Chars read, still in the scratch: the match
	// can derive other objects (layer5colors.go's table), and they must not
	// rewrite it under the types it is answering from.
	if o.ExtraTypes != nil {
		e.charsWalk.Borrowed++
		defer func() { e.charsWalk.Borrowed-- }()
	}
	// A zone-scoped candidate reads a bare Permanent base as a permanent
	// card; any other spec keeps the derived-bound ordinary match.
	if o.InZone && effects.SpecMentionsPermanent(spec) {
		return effects.MatchesSpecInZone(e.G, spec, id, sc, o.Zone)
	}
	return e.matchesSpec(spec, id, sc)
}

func (b *trigBoard) MatchesObject(spec string, obj *state.Object, source state.ObjID, you state.PlayerID, o trigmatch.SpecOpts) bool {
	e := b.eng()
	sc := e.specCtx(source, you)
	sc.DelayedRemembered, sc.ExtraTypes, sc.Layers.StaticGoads = o.DelayedRemembered, o.ExtraTypes, o.StaticGoads
	if o.DelayedChosenBound {
		sc.Chosen, sc.ChosenValid = o.DelayedChosen, true
	}
	if obj != nil {
		sc.ExtraTypesOwner = obj.ID
		// A permanent that has just left the battlefield is matched by the
		// types it had there (CR 603.10a, 613.1d), animation included: the live
		// table no longer carries them.
		if sc.ExtraTypes == nil && e.lkiTypes.set && e.lkiTypes.id == obj.ID {
			sc.ExtraTypes = e.lkiTypes.types
		}
		// An object last known on the battlefield, on a board that still has
		// it there (the leaves-the-battlefield look-back observer, CR 603.10a),
		// matches by its derived characteristics, as matchesSpec binds them.
		if on := e.G.Obj(obj.ID); on != nil && on.Zone == state.ZBattlefield && obj.Zone == state.ZBattlefield && e.activeDepth == 0 {
			facts := specBindFacts(spec)
			if facts.reads {
				d := e.Derived(obj.ID)
				sc.ExtraKeywords, sc.ExtraKeywordsOwner = d.Keywords, obj.ID
				e.charsWalk.Borrowed++
				defer func() { e.charsWalk.Borrowed-- }()
				sc.DerivedPower, sc.DerivedToughness, sc.HasDerivedPT = d.Power, d.Toughness, true
				sc.BasePower, sc.BaseToughness, sc.HasBasePT = d.BasePower, d.BaseToughness, true
			}
			if facts.colors {
				sc.Layers.DerivedColors = e.derivedColorTable()
			}
		}
	}
	return effects.MatchesObjectCtx(e.G, spec, obj, sc)
}

func (b *trigBoard) BoardLayers() effects.LayerTables { return b.eng().boardLayers() }

func (b *trigBoard) StaticallyGoadedWithLKI(lki *state.Object) map[state.ObjID]bool {
	return b.eng().staticallyGoadedWithLKI(lki)
}

func (b *trigBoard) CastProvenanceAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	return b.eng().castProvenanceAdmits(spec, objID, you)
}

func (b *trigBoard) ActionCause() state.ObjID { return b.eng().actionCause() }

func (b *trigBoard) InResolution() bool { return b.resolvingObj != 0 }

// PayingCostOf reads the open cast flow (pendingCast) for the keyword whose
// cost is being paid: an activated ability's Keyword$ (Craft), or Emerge for
// an emerge cast (CR 702.118a).
func (b *trigBoard) PayingCostOf() (kw string, isSpell bool, emergeSac state.ObjID, ok bool) {
	pc := b.cast
	if pc == nil {
		return "", false, 0, false
	}
	if pc.isAbility() {
		if sa := b.eng().pcAbility(pc); sa != nil {
			if kw := sa.ParamStr(cards.PKKeyword); kw != "" {
				return kw, false, 0, true
			}
		}
		return "", false, 0, false
	}
	if pc.emerge {
		return "Emerge", true, pc.emergeSac, true
	}
	return "", false, 0, false
}

func (b *trigBoard) InFlightCounterAdder() (state.PlayerID, bool) {
	return b.eng().inFlightCounterAdder()
}

func (b *trigBoard) AbilityCastStackObject(perm state.ObjID) state.ObjID {
	return b.eng().abilityCastStackObject(perm)
}

// LiveActivation reads the open cast flow's pendingCast when it is an
// activation of card's ability: its answered targets and the counters its
// paid SubCounter parts removed (an announced part counts the announced X).
func (b *trigBoard) LiveActivation(card state.ObjID) (trigmatch.Activation, bool) {
	pc := b.cast
	if pc == nil || !pc.isAbility() || pc.card != card {
		return trigmatch.Activation{}, false
	}
	var n int32
	for _, part := range pc.cost.SubCounter {
		if part.Announced {
			n += pc.x
		} else {
			n += part.N
		}
	}
	return trigmatch.Activation{Targets: pc.targets, CountersRemoved: n}, true
}

func (b *trigBoard) TokenSnapshot(ev events.Event) *state.Object { return b.eng().tokenSnapshot(ev) }
func (b *trigBoard) ArrivingPlane(ev events.Event) state.ObjID   { return b.eng().arrivingPlane(ev) }

func (b *trigBoard) ProtectionSource(source state.ObjID) state.ObjID {
	return b.eng().protectionSource(source)
}

func (b *trigBoard) EchoGateHolds(source state.ObjID) bool { return b.eng().echoGateHolds(source) }

func (b *trigBoard) SpellsCastThisTurn(p state.PlayerID) int {
	return pay.SpellsCastThisTurn(asPayer(b.eng()), p)
}
func (b *trigBoard) ManaExpendTotal(p state.PlayerID) int32 { return b.eng().manaExpendTotal(p) }

func (b *trigBoard) SpellsCastThisTurnByMatching(activator, you state.PlayerID, spec string) int {
	return len(b.eng().castsThisTurnMatching(you, spec, 0, effects.TriggerContext{}, activator))
}

func (b *trigBoard) IsLoyaltyAbility(ab *cards.SA) bool { return isLoyaltyAbility(ab) }

// playingLand is the land whose land play is in flight (cast_commit.go's
// payCast arms etbLandPlay before the move and finishLandPlay disarms it
// after the move's triggers are checked), or 0.
func (b *trigBoard) playingLand() state.ObjID {
	if !b.etbLandPlay {
		return 0
	}
	return b.etbLandObj
}
