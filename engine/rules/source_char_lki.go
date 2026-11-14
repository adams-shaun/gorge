package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Source-characteristic last-known information for a target filter.
//
// CR 113.7a: an ability on the stack exists independently of its source, and
// CR 608.2h: when it needs information about a source that has left the
// battlefield, it uses the source's last known information. A target filter
// whose bound is the source's own characteristic -- Alesha, Who Laughs at
// Fate's "creature card with mana value less than or equal to Alesha's
// power" (ValidTgts$ ...cmcLEX with SVar:X:Count$CardPower), Legacy's
// Allure's "power less than or equal to the number of treasure counters on
// it" (Count$CardCounters.TREASURE, the source sacrificed as the cost) -- is
// re-judged at resolution (CR 608.2b, legalTargets) through
// targetSpecContext's Resolve. Before this, that Resolve read the LIVE source:
// a graveyard card with its printed power and no counters, so a target the
// offer had certified fizzled when the source was destroyed in response.
//
// The snapshot is the source's layer-derived power and toughness and its
// counters at the last moment it was on the battlefield, captured at the same
// departure boundary as the own-source lifelink/controller LKI
// (captureSourceLifelinkLKI): for every waiting ability object of that source
// on the stack, and for every pending trigger of it (moved onto the stack
// object at TriggerPush). An activation that sacrifices its own source as a
// cost is seeded before the cost is paid (payCast). A zone-change trigger of
// the source itself (it died) already carries its LKI in triggerLKI, which
// sourceCharLKIFor reads when no departure snapshot exists.
//
// Engine-only and replay-derived like the other LKI maps: a replay re-runs
// the same intents, so the same departures repopulate it identically.

// sourceCharSnapshot is one source's characteristics as it last existed on
// the battlefield. counters is an owned copy that is never written after
// capture, so a clone may share it.
type sourceCharSnapshot struct {
	power, toughness int32
	counters         []state.Counter
	// chosenType and chosenColor are the source's recorded "as this enters"
	// choices; the live object loses them on leaving the battlefield.
	chosenType, chosenColor string
	// chosen is the source's recorded card/player choice (Object.Chosen, the
	// Choose "chosen" fold): Dauntless Bodyguard's as-enters creature, read
	// by its sacrifice-cost Pump's Card.ChosenCardStrict. Owned copy, never
	// written after capture.
	chosen []state.Target
	// manaColorSpent is the cast-time spend ("if {W}{W} was spent to cast
	// it"); the live object loses it on leaving the battlefield (CR 400.7).
	manaColorSpent state.Mana
	// tapped is the source's battlefield status, read by an intervening-if
	// whose PresentDefined$ Self spec asks Card.tapped/Card.untapped (House
	// Cartographer's Survival, its trigger killed by Bear Trap in response;
	// CR 608.2h reads the departed source's last known status).
	tapped bool
}

// sourceCharSnapshotOf reads src's characteristics now; src must still be on
// the battlefield for the power/toughness to be layer-derived.
func (e *Engine) sourceCharSnapshotOf(src state.ObjID) sourceCharSnapshot {
	o := e.G.Obj(src)
	if o == nil {
		return sourceCharSnapshot{}
	}
	return sourceCharSnapshot{power: e.Power(src), toughness: e.Toughness(src),
		counters:   append([]state.Counter(nil), o.Counters...),
		chosenType: o.ChosenType, chosenColor: o.ChosenColor, chosen: append([]state.Target(nil), o.Chosen...),
		manaColorSpent: o.ManaColorSpent, tapped: o.Tapped}
}

// bindSourceLKI binds the departed source's snapshot for the stack object id
// into tc: the chosen type/colour, the cast-time mana spend and the power,
// toughness and counters (CR 113.7a / 608.2h). It does nothing for an ability whose source never left, so those
// read live as before.
func (e *Engine) bindSourceLKI(tc *effects.TriggerContext, id, source state.ObjID) {
	snap, ok := e.sourceCharLKI[id]
	if !ok || source == 0 {
		return
	}
	tc.SourceLKIValid = true
	tc.SourceChosenType, tc.SourceChosenColor = snap.chosenType, snap.chosenColor
	tc.SourceChosen = snap.chosen
	tc.SourceManaColorSpent = snap.manaColorSpent
	tc.SourcePower, tc.SourceToughness, tc.SourceCounters = snap.power, snap.toughness, snap.counters
	tc.SourceTapped = snap.tapped
}

// captureSourceCharLKI records src's snapshot, immediately before it leaves
// the battlefield, for every ability object of src on the stack and every
// pending trigger of src that has none yet. The FIRST departure wins: an
// ability whose source already left, came back as a new object (CR 400.7)
// and left again still refers to the object that created it. The snapshot is
// taken only when some waiting ability needs it.
func (e *Engine) captureSourceCharLKI(src state.ObjID) {
	var snap sourceCharSnapshot
	have := false
	take := func() sourceCharSnapshot {
		if !have {
			snap, have = e.sourceCharSnapshotOf(src), true
		}
		return snap
	}
	for _, id := range e.G.Stack {
		o := e.G.Obj(id)
		if o == nil || o.Ability == nil || o.Source != src {
			continue
		}
		if _, ok := e.sourceCharLKI[id]; ok {
			continue
		}
		if e.sourceCharLKI == nil {
			e.sourceCharLKI = make(map[state.ObjID]sourceCharSnapshot)
		}
		e.sourceCharLKI[id] = take()
	}
	for i := range e.pendingTriggers {
		pt := &e.pendingTriggers[i]
		if pt.Source == src && !pt.sourceCharLKIValid {
			pt.sourceCharLKI, pt.sourceCharLKIValid = take(), true
		}
	}
	for i := range e.G.Delayed {
		if dt := &e.G.Delayed[i]; dt.EventMode == "SpellCast" && dt.Source == src {
			if e.delayedSourceCharLKI == nil {
				e.delayedSourceCharLKI = make(map[state.ObjID]sourceCharSnapshot)
			}
			e.delayedSourceCharLKI[src] = take() // the last departure wins
			break
		}
	}
}

// delayedSourceCharLKIFor returns the departure snapshot of a delayed
// registration's source, when the source is no longer on the battlefield. A
// source back on the battlefield reads live.
func (e *Engine) delayedSourceCharLKIFor(source state.ObjID) (sourceCharSnapshot, bool) {
	snap, ok := e.delayedSourceCharLKI[source]
	if !ok {
		return sourceCharSnapshot{}, false
	}
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZBattlefield {
		return sourceCharSnapshot{}, false
	}
	return snap, true
}

// delayedSpellMatchSpec is the filter context of a delayed SpellCast
// registration's ValidCard$ match. A source that has left the battlefield
// answers its characteristic heads (Count$CardPower, CardToughness,
// CardCounters.<KIND>) from its departure snapshot (CR 608.2h: Loki
// Laufeyson's "mana value less than or equal to Loki's power"); every other
// body, and a source still on the battlefield, resolves live as specCtx does.
func (e *Engine) delayedSpellMatchSpec(dt *state.DelayedTrigger) effects.SpecContext {
	sc := e.specCtx(dt.Source, dt.Controller)
	snap, ok := e.delayedSourceCharLKIFor(dt.Source)
	if !ok {
		return sc
	}
	sc.SourceLKIValid = true
	sc.SourcePower, sc.SourceToughness, sc.SourceCounters = snap.power, snap.toughness, snap.counters
	sc.SourceChosenType, sc.SourceChosenColor = snap.chosenType, snap.chosenColor
	sc.SourceChosen = snap.chosen
	live := sc.Resolve
	sc.Resolve = func(name string) (int32, bool) {
		if o := e.G.Obj(dt.Source); o != nil {
			if f := o.Face(); f != nil {
				if body, ok := f.SVars[name]; ok {
					if n, ok := snap.count(body); ok {
						return n, true
					}
				}
			}
		}
		return live(name)
	}
	return sc
}

// SourceLeftBattlefieldSince reports whether the source of the stack object
// stack left the battlefield after that ability was created -- triggered
// (the snapshot a pending trigger took is moved onto its stack object at
// TriggerPush) or activated (including a self-sacrifice cost, seeded by
// payCast). effects reads it so "this permanent" effects of an ability whose
// source became a new object do nothing (CR 400.7).
//
// A source that left together with the permanent it watches (CR 603.10a:
// Super Shredder and Casey Jones die together) triggers from the shared
// pre-departure board, so its trigger is queued after the source's departure
// and took no snapshot. Such a trigger can only have come from the battlefield,
// so finding its source elsewhere at push is the same fact.
func (e *Engine) SourceLeftBattlefieldSince(stack state.ObjID) bool {
	if _, ok := e.sourceCharLKI[stack]; ok {
		return true
	}
	t, ok := e.triggerForAbilityObject(stack, e.G.Obj(stack))
	return ok && triggerOnlyFromBattlefield(t)
}

// triggerOnlyFromBattlefield reports whether t can only have triggered while
// its source was on the battlefield: it says so (TriggerZones$, or its
// ActiveZones$ spelling, as zoneGate reads them), or it declares neither and
// watches a departure from the battlefield.
func triggerOnlyFromBattlefield(t cards.Trigger) bool {
	spec := t.ParamStr(cards.PKTriggerZones)
	if spec == "" {
		spec = t.ParamStr(cards.PKActiveZones)
	}
	if spec == "" {
		spec = t.ParamStr(cards.PKOrigin)
	}
	if spec == "" {
		// zoneGate's default is the battlefield for every mode but the few it
		// admits from elsewhere: a granted "whenever this creature deals combat
		// damage" (Quintessential Katana) on a creature that dealt it and died
		// in the same step is queued after the departure.
		switch t.Mode {
		case "Discarded", "Cycled", "ChangesZone", "ChangesZoneAll", "SpellCast", "SpellCastOrCopy", "Drawn":
			return false
		}
		return true
	}
	return spec == "Battlefield"
}

// setSourceCharLKI records snap for the stack object id.
func (e *Engine) setSourceCharLKI(id state.ObjID, snap sourceCharSnapshot) {
	if e.sourceCharLKI == nil {
		e.sourceCharLKI = make(map[state.ObjID]sourceCharSnapshot)
	}
	e.sourceCharLKI[id] = snap
}

// sourceCharLKIFor returns the last-known snapshot a target filter of the
// stack object stack must read for its source, and whether one applies: a
// departure snapshot captured while the ability waited, else the trigger's
// own zone-change LKI when that LKI IS the source (a dies trigger reading
// "its power") and the live object is no longer that permanent. A source
// still on the battlefield as the same object reads live, as before.
func (e *Engine) sourceCharLKIFor(stack, source state.ObjID, lki *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool) (sourceCharSnapshot, bool) {
	if snap, ok := e.sourceCharLKI[stack]; ok {
		return snap, true
	}
	if lki == nil || !lkiPTValid || lki.ID != source || lki.Zone != state.ZBattlefield {
		return sourceCharSnapshot{}, false
	}
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZBattlefield && o.Incarnation == lki.Incarnation {
		return sourceCharSnapshot{}, false
	}
	return sourceCharSnapshot{power: lkiPower, toughness: lkiToughness, counters: lki.Counters,
		chosenType: lki.ChosenType, chosenColor: lki.ChosenColor, chosen: lki.Chosen, manaColorSpent: lki.ManaColorSpent,
		tapped: lki.Tapped}, true
}

// count answers a source-characteristic Count$ body from the snapshot:
// Count$CardPower, Count$CardToughness and Count$CardCounters.<KIND> (ALL
// included), each with an optional /Op suffix applied by the count grammar's
// own arithmetic. ok is false for every other body, which the caller then
// evaluates as before.
func (s sourceCharSnapshot) count(body string) (int32, bool) {
	head, ok := strings.CutPrefix(strings.TrimSpace(body), "Count$")
	if !ok {
		return 0, false
	}
	head, op, hasOp := strings.Cut(head, "/")
	var n int32
	switch {
	case head == "CardPower":
		n = s.power
	case head == "CardToughness":
		n = s.toughness
	case strings.HasPrefix(head, "CardCounters."):
		scratch := state.Object{Counters: s.counters}
		n = scratch.Counter(strings.TrimPrefix(head, "CardCounters."))
	default:
		return 0, false
	}
	if hasOp {
		n = effects.ApplyCountOp(n, op)
	}
	return n, true
}
