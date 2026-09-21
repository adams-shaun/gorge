// Triggers granted by a static rather than printed on the face.
//
// Conspire, Dethrone, Afflict and Ward reach the stack through a granting static,
// so they are checked here rather than through the per-mode matchers.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// checkGrantedConspireTriggers synthesizes Conspire's copy trigger (CR
// 702.78a's second ability) for a spell that currently HAS the keyword but
// does not print it: a layer-6 grant (Wort, the Raidmother's "each red or
// green instant or sorcery spell you cast has conspire", Raiding Schemes'
// noncreature arm) gives the spell the same rules text as a printed keyword,
// and the printed K:Conspire expansion (cards/keywords.go) only covers
// printed lines. Without this walk the granted spell's cast flow still
// offers the conspired cast, still asks the tap and still pays it (the
// derived-keyword read the offer uses), but nothing copies -- the player pays
// an unrecoverable cost for nothing. The synthesized trigger reuses the
// printed expansion's exact trigger/body shape (Mode$ SpellCast, ValidCard$
// Card.Self, TriggerZones$ Stack; DB$ CopySpellAbility over
// Defined$ TriggeredSpellAbility with Amount$ Count$Conspired), so its
// behaviour is byte-identical to the printed path's: it fires on EVERY cast
// of the granted spell (the deferred CR 601.2i walk, so the pay-time
// FlagConspired CastInfo has already folded Object.Conspired) and resolves
// to a no-op when the tap was declined (Amount 0, effCopySpellAbility's
// loop emits nothing). A copy emits StackCopy, not PutOnStack, so the
// synthesis cannot double-fire on its own output. The walk skips a face that
// PRINTS Conspire (the printed expansion already owns the line -- the same
// grant-identical-to-a-printed-line dedup Afflict keeps). Like Dethrone's
// synthesis this is a read-only derived-characteristics check; granting
// stays in the continuous-effect system. It runs on the faceMayTrigger
// early-return path too (a granted keyword is independent of printed
// triggers, the same shape Dethrone/Afflict are), and its gate is ordered
// cheap-first -- event kind, then object identity, one integer compare each
// on the hot per-event walk -- before the derived keyword scan allocates.
func (e *Engine) checkGrantedConspireTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.PutOnStack || id != ev.Obj {
		return
	}
	if !e.HasKeyword(id, "Conspire") || f.HasKeyword("Conspire") {
		return
	}
	t := cards.Trigger{Mode: "SpellCast", Params: map[string]string{
		"Mode": "SpellCast", "ValidCard": "Card.Self", "TriggerZones": "Stack", "TriggerDescription": "Conspire",
	}, Effect: &cards.SA{Kind: "DB", API: "CopySpellAbility", Params: map[string]string{
		"Defined": "TriggeredSpellAbility", "Amount": "Count$Conspired", "MayChooseTarget": "True",
	}}}
	if observer.triggerMatches(t, id, ev, objLKI) {
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] < maxTriggerFires {
			e.triggerFireCount[key]++
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				// The Ward shape: the body rides the push's __kwConspire
				// payload for events.Apply to rebuild structurally -- a raw
				// SA cannot cross the log, and the TriggerPush -1 index
				// sentinel is Dethrone's own. The cast spell rides
				// Remembered because Defined$ TriggeredSpellAbility reads
				// the triggering spell off it.
				Conspire: true,
				Ctx: effects.Ctx{
					Source:         id,
					Controller:     o.Controller,
					Remembered:     triggerRemembered(ev, id),
					LKI:            objLKI,
					TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
				},
			})
		}
	}
}

// checkGrantedDethroneTriggers synthesizes Dethrone's ordinary attack trigger
// (CR 702.105) for a creature that currently HAS the keyword but does not
// print it: a keyword granted in layer 6 has the same rules text as a printed
// keyword, and keyword expansion only adds triggers for printed K: lines.
// This is deliberately a read-only derived-characteristics check; granting
// and removing the keyword remains entirely in the existing continuous-effect
// system. It must run on the faceMayTrigger early-return path too (a granted
// keyword is independent of printed triggers, the same shape Granted Ward
// is), so it is called once per object before that gate.
func (e *Engine) checkGrantedDethroneTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.DeclareAttackers || e.HasKeyword(id, "Dethrone") && f.HasKeyword("Dethrone") {
		return
	}
	if !e.HasKeyword(id, "Dethrone") || f.HasKeyword("Dethrone") {
		return
	}
	t := cards.Trigger{Mode: "Attacks", Params: map[string]string{
		"Mode": "Attacks", "ValidCard": "Card.Self", "Dethrone": "True",
	}, Effect: &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
		"Defined": "Self", "CounterType": "P1P1", "CounterNum": "1",
	}}}
	if observer.triggerMatches(t, id, ev, objLKI) {
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] < maxTriggerFires {
			e.triggerFireCount[key]++
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{Source: id, Controller: o.Controller, Idx: -1, SA: t.Effect,
				Ctx: effects.Ctx{Source: id, Controller: o.Controller, Remembered: triggerRemembered(ev, id), LKI: objLKI,
					TriggerContext: observer.triggerReferents(t, id, ev, objLKI)}})
		}
	}
}

// checkGrantedAfflictTriggers synthesizes Afflict's become-blocked trigger
// (CR 702.130) for a creature that currently HAS the keyword but does not
// print it: a keyword granted in layer 6 (Lost Monarch of Ifnir's
// "Other Zombies you control have afflict 3") has the same rules text as a
// printed keyword, and the printed K:Afflict expansion (cards/keywords.go)
// only covers printed lines. The synthesized trigger reuses the ordinary
// AttackerBlocked machinery -- triggerModeEvents' DeclareBlockers entry,
// zoneGate/phaseGate, attackerBlockedCandidates' dedup and fire-count bound
// -- so its behaviour is byte-identical to the printed path's for
// ValidCard$ Card.Self: one instance per distinct blocking declaration of
// the granted creature itself, the defender captured as
// TriggerContext.DefendingPlayer for the body's
// Defined$ TriggeredDefendingPlayer. The granted amount is the FIRST
// keyword granted in a layer never matches. The walk does NOT skip a card
// that also prints Afflict: the granted amounts are every Afflict entry in
// the derived list whose exact text no printed line already carries, so a
// card printing K:Afflict:1 that is granted "Afflict 3" fires BOTH (printed
// expansion for 1, this walk for 3), while a grant identical to a printed
// line is skipped -- the printed expansion already owns it and the two are
// indistinguishable as strings (a Monarch carrying another Monarch's
// identical grant keeps its one printed instance; the conservative choice).
// Every distinct grant queues its own instance, so two Monarchs on the
// battlefield stack two afflict-3 instances on the same Zombie. Like
// Dethrone's synthesis this is a read-only derived-characteristics check;
// granting stays in the continuous-effect system.
func (e *Engine) checkGrantedAfflictTriggers(id state.ObjID, o *state.Object, f *cards.Face, ev events.Event) {
	if ev.Kind != events.DeclareBlockers {
		return
	}
	if !e.HasKeyword(id, "Afflict") {
		return
	}
	printed := map[string]bool{}
	for _, k := range f.Keywords {
		printed[strings.ToLower(k)] = true
	}
	var amounts []int
	for _, k := range e.Derived(id).Keywords {
		if !strings.EqualFold(cards.KeywordHead(k), "Afflict") || printed[strings.ToLower(k)] {
			continue
		}
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		amount, err := strconv.Atoi(param)
		if err != nil || amount <= 0 {
			continue // a malformed or non-positive grant delivers no trigger.
		}
		amounts = append(amounts, amount)
	}
	if len(amounts) == 0 {
		return
	}
	t := cards.Trigger{Mode: "AttackerBlocked", Params: map[string]string{
		"Mode": "AttackerBlocked", "ValidCard": "Card.Self",
	}}
	if !e.zoneGate(t, id, ev) || !e.phaseGate(t) {
		return
	}
	key := triggerKey{Source: id, Idx: -1}
	if e.triggerFireCount == nil {
		e.triggerFireCount = map[triggerKey]int32{}
	}
	if e.triggerFireCount[key] >= maxTriggerFires {
		return // cascade bound: see maxTriggerFires.
	}
	pt := func(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }
	for _, aid := range e.attackerBlockedCandidates(t, id, ev) {
		// ValidCard$ Card.Self admits only the granted creature itself;
		// attackerBlockedCandidates still returns other attackers this event
		// blocked, so skip anything that is not the source.
		if aid != id {
			continue
		}
		defender := pt(0)
		if ao := e.G.Obj(aid); ao != nil {
			defender = pt(ao.Attacking)
		}
		for _, amount := range amounts {
			e.triggerFireCount[key]++
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				// The Ward shape: the body rides the push's __kwAfflict:
				// payload for events.Apply to rebuild structurally -- a raw
				// SA cannot cross the log, and the TriggerPush -1 index
				// sentinel is Dethrone's own.
				Afflict: strconv.Itoa(amount),
				Ctx: effects.Ctx{
					Source:     id,
					Controller: o.Controller,
					Remembered: []state.Target{{Obj: aid}},
					Captured:   []state.Target{{Obj: aid}},
					TriggerContext: effects.TriggerContext{
						TriggerCard:     aid,
						TriggerSource:   aid,
						AttackingPlayer: pt(e.controllerOf(aid)),
						DefendingPlayer: defender,
					},
				},
			})
		}
	}
}

func (e *Engine) checkGrantedWardTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool) {
	if e.finishingLifeLossBatch || e.lifeLossBatchDepth > 0 {
		return
	}
	// Every synthesized Ward uses BecomesTarget + Card.Self. Apply that
	// matcher's cheap gates before deriving characteristics for every object
	// in every zone. Keep the caller's visitation/queue order unchanged.
	if ev.Kind != events.TargetsChosen || !slices.Contains(ev.IDs, id) {
		return
	}
	printed := map[string]bool{}
	if !e.faceDownPrintedHides(o) {
		// While the object is face down its printed face does not exist
		// (CR 708.8): a cloaked card whose real face prints Ward must not
		// suppress its own cloak-ward -- the derived list is the only
		// keyword source on this path.
		for _, k := range f.Keywords {
			printed[strings.ToLower(k)] = true
		}
	}
	for _, k := range observer.Derived(id).Keywords {
		if printed[strings.ToLower(k)] {
			continue
		}
		param := ""
		if head := cards.KeywordHead(k); !strings.EqualFold(head, "Ward") {
			continue
		} else if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		if param == "" {
			continue
		}
		t := cards.Trigger{Mode: "BecomesTarget",
			Params: map[string]string{"ValidTarget": "Card.Self", "Ward": "True", "TriggerDescription": "Ward"},
			Effect: &cards.SA{Kind: "DB", API: "Ward",
				Params: map[string]string{"UnlessCost": param, "TriggerDescription": "Ward"}}}
		if !observer.triggerMatches(t, id, ev, objLKI) {
			continue
		}
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     id,
			Controller: o.Controller,
			Ward:       param,
			Ctx: effects.Ctx{
				Source:         id,
				Controller:     o.Controller,
				Remembered:     triggerRemembered(ev, id),
				Captured:       triggerRemembered(ev, id),
				LKI:            objLKI,
				LKIPower:       lkiPower,
				LKIToughness:   lkiToughness,
				LKIPTValid:     objLKI != nil && lkiPTValid,
				TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
			},
		})
	}
}

// checkGrantedStaticTriggers queues the triggered abilities a live static
// grant (AddTrigger$ on a Mode$ Continuous static, e.g. Hearthhull's
// "STATION 8+ Whenever you sacrifice a land") gives this object: the
// checkGrantedWard/checkGrantedDethrone precedent -- a granted triggered
// ability has the same rules text a printed one would, and the face walk
// above only ever scans printed T: lines. The grants live on the memoised
// static scan (active()); each Affected$-matched grant is matched against
// the event exactly as a printed trigger would be (triggerMatches, which
// resolves the granted body's own TriggerZones$/ValidPlayer$/Phase$ clauses)
// and queued with its SVar-resolved SA -- the pendingTrigger shape the
// granted-keyword paths use, pushed through events.GrantTriggerPush.
//
// The queue gate is the live==replay contract: the stack object is minted
// inside events.Apply, which resolves the Execute$ body from the GRANTOR's
// own SVar table (the object carrying the printed static, threaded to the
// event as Amount; 0 = the self-grant shape, where grantor == recipient), so
// the walk links the effect from that same table (the resolveSVarAcrossFaces
// walk mirrored in grantedTriggerExecute) and a grant whose body it cannot
// produce never queues -- the conservative direction, matching the
// replayable-log invariant rather than minting an ability a replay cannot
// rebuild. A self-grant (Hearthhull) trivially satisfies it, and so does a
// cross-object grant (an Aura granting its enchanted creature a trigger): the
// Execute$ SVar lives on the GRANTOR's face, which is ce.Source.
//
// Fire-count: like Ward and Dethrone, every granted trigger shares the
// granted slot's triggerKey (Source, Idx -1) -- the cascade bound only, not
// once-per-turn memory, and maxTriggerFires (256) is generous enough that
// the sharing cannot starve a legitimate fire. Like those two the walk is
// deliberately a read over active()'s sorted slice, never a map: the queue
// order stays the scan's deterministic order.
func (e *Engine) checkGrantedStaticTriggersUsing(observer *Engine, statics []ContinuousEffect, id state.ObjID, o *state.Object, ev events.Event, objLKI *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool) {
	if e.finishingLifeLossBatch || e.lifeLossBatchDepth > 0 {
		return
	}
	for i := range statics {
		ce := &statics[i]
		if ce.AddTrigger == nil {
			continue
		}
		if !effects.MatchesSpecFrom(observer.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		t := *ce.AddTrigger
		// The live==replay gate: link the Execute$ body exactly the way
		// events.Apply will (the GRANTOR's own table -- ce.Source carries the
		// printed static; a self-grant degenerates to the affected object); a
		// body it cannot resolve never queues, and a same-named body it CAN
		// resolve is by construction the same body a replay would resolve.
		grantor := observer.G.Obj(ce.Source)
		if grantor == nil || grantor.Face() == nil {
			continue
		}
		if t.Effect = grantedTriggerExecute(grantor, t.Params["Execute"]); t.Effect == nil {
			continue
		}
		// CR 603.8's outstanding-instance latch, mirrored from the face walk
		// (a state trigger already queued or on the stack does not re-fire).
		if t.Mode == "Always" && e.stateTriggerOutstanding(id, -1) {
			continue
		}
		// LifeLostAll is evaluated once at the end of a simultaneous
		// life-loss batch, exactly as the face walk scopes it.
		if t.Mode == "LifeLostAll" && e.lifeLossBatchDepth > 0 && !e.finishingLifeLossBatch {
			continue
		}
		if !observer.triggerMatches(t, id, ev, objLKI) {
			continue
		}
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue // cascade bound: see maxTriggerFires.
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     id,
			Controller: o.Controller,
			Idx:        -1,
			SA:         t.Effect,
			Granted:    true,
			Grantor:    ce.Source,
			Execute:    t.Params["Execute"],
			Ctx: effects.Ctx{
				Source:         id,
				Controller:     o.Controller,
				Remembered:     triggerRemembered(ev, id),
				LKI:            objLKI,
				LKIPower:       lkiPower,
				LKIToughness:   lkiToughness,
				LKIPTValid:     objLKI != nil && lkiPTValid,
				TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
			},
		})
	}
}

// grantedTriggerExecute mirrors events.Apply's GrantTriggerPush resolution
// (the resolveSVarAcrossFaces walk): the granted body's Execute$ name is
// resolved against the GRANTOR's own SVar table -- current face first, then
// every other face -- so the live queue links exactly the body a replayed
// log will. (Apply resolves from the object Amount names when set; for the
// self-grant shape grantor == recipient, so passing the grantor covers both
// arms.) nil when no face resolves it.
func grantedTriggerExecute(o *state.Object, execute string) *cards.SA {
	if execute == "" {
		return nil
	}
	if f := o.Face(); f != nil {
		if sa := cards.ResolveSVar(f.SVars, execute); sa != nil {
			return sa
		}
	}
	if o.Card == nil {
		return nil
	}
	for _, cf := range o.Card.Faces {
		if sa := cards.ResolveSVar(cf.SVars, execute); sa != nil {
			return sa
		}
	}
	return nil
}

// triggerCastAlt is one surviving ValidCard$ alternative of a Mode$
// SpellCast trigger after the trigger-side self-cast exclusion was applied:
// spec is the token-stripped filter text, exclSelf whether the alternative
// carried the bare !CastSaSource token — its ActivatorThisTurnCastEach$
// tally must also skip the trigger source's own printed-name casts.
type triggerCastAlt struct {
	spec     string
	exclSelf bool
}
