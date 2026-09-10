package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// manaLetters is state.Mana's index order (MW, MU, MB, MR, MG, MC) spelled
// out as the WUBRGC symbols events.ManaAdd's Counter field expects.
var manaLetters = [...]string{"W", "U", "B", "R", "G", "C"}

// payMana spends cost from p's pool and reports whether it could. Every
// state mutation goes through events, so payment cannot be a direct field
// write to Players[p].Pool: it emits one ManaAdd event per colour bucket
// actually spent, with a negative Amount. That reuses the existing ManaAdd
// kind rather than adding a new one -- Apply's ManaAdd case is a plain "+=",
// so a negative Amount already subtracts correctly, the same trick Ruling F4
// uses for clearing damage.
//
// Ruling T19b-b: this used to be silent on failure (a no-op the caller could
// not observe), on the theory that legalActions always gates "cast" on
// CanPay first so failure here was unreachable. That theory held for the
// base cost but not for an alternative one: legalActions gates an alt-cost
// option on the ALTERNATIVE cost being payable, so a caller that (as
// castSpell used to) paid the base cost regardless of which option was
// chosen could genuinely hit this path with real, well-formed card data --
// and a silent no-op there is what let the spell go on the stack anyway,
// having paid nothing. Reporting failure explicitly is what lets castSpell
// abort the cast instead.
func (e *Engine) payMana(p state.PlayerID, cost Cost) bool {
	before := e.G.Players[p].Pool
	after, lifeSpent, ok := cost.resolveMana(before, e.G.Players[p].Life)
	if !ok {
		return false
	}
	for i, letter := range manaLetters {
		if spent := before[i] - after[i]; spent != 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -spent})
		}
	}
	// CR 107.4f: a Phyrexian pip paid with life costs two life, deducted
	// through the ordinary LifeChange event so a replay learns it.
	if lifeSpent != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -lifeSpent})
	}
	return true
}

// targetBounds resolves a targeting subject's TargetMin$/TargetMax$ to the
// decision's Min/Max. Missing bounds default to 1 (the M1 single-target
// contract: a spell or ability that targets at all targets one thing).
// TargetMin$ 0 is legal -- requirement N2: a spell that MAY target zero
// things resolves untargeted when no legal target exists -- while a negative
// Min or a Max below Min is clamped back to a sane bound (never below 1, so
// a truncated 0 max still asks for at least one). The rule is clamp, not
// ignore: TargetMax$ is parsed whatever it reads (TargetMax$ 0 and even a
// negative TargetMax$ are both taken seriously) and then clamped so
// max >= 1 and max >= min. A discarded parameter and a clamped one both
// resolve to the same number when used alone, but they differ the moment
// TargetMin$ is also present -- this says which one the engine means.
func targetBounds(sa *cards.SA) (int, int) {
	min, max := 1, 1
	if v, ok := sa.Params["TargetMin"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			min = n
		}
	}
	if v, ok := sa.Params["TargetMax"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			max = n
		}
	}
	if min < 0 {
		min = 1
	}
	if max < 1 {
		max = 1
	}
	if max < min {
		max = min
	}
	return min, max
}

// targetMin is the Min half of targetBounds, inlined for resolveTop's N2 gate.
func targetMin(sa *cards.SA) int {
	min, _ := targetBounds(sa)
	return min
}

// targetZones resolves TgtZone$ (comma-separated) and TargetType$ into the
// zones to search for target options. TgtZone$ is the explicit zone
// declaration; TargetType$ (Forge) names the KIND of thing targeted, and
// when it names a stack object -- Spell, Instant, Sorcery, Activated,
// Triggered, SpellAbility -- the target lives on the stack. Mana Leak is
// scripted `TargetType$ Spell | ValidTgts$ Card` with NO TgtZone$ at all, so
// without reading TargetType$ the search defaulted to the battlefield and
// offered every permanent of every seat for a counterspell -- the bug this
// fixes (every counterspell in the corpus was inert, targeting a permanent
// so effCounter found o.Zone != state.ZStack and did nothing). The default
// remains the battlefield. An unknown TgtZone$ token is dropped (reviewer
// minor 4), but the battlefield default applies only when NEITHER source
// named a zone -- a typo'd TgtZone$ on a TargetType$ Spell card must not
// silently widen a stack target back to the battlefield.
func targetZones(sa *cards.SA) []state.Zone {
	var zones []state.Zone
	for _, z := range strings.Split(sa.Params["TgtZone"], ",") {
		switch strings.TrimSpace(z) {
		case "Battlefield":
			zones = appendUniqueZone(zones, state.ZBattlefield)
		case "Graveyard":
			zones = appendUniqueZone(zones, state.ZGraveyard)
		case "Hand":
			zones = appendUniqueZone(zones, state.ZHand)
		case "Exile":
			zones = appendUniqueZone(zones, state.ZExile)
		case "Stack":
			zones = appendUniqueZone(zones, state.ZStack)
		}
	}
	// A stack-targeting TargetType$ adds the stack even when no TgtZone$ is
	// present (the counterspell shape) and even alongside a TgtZone$
	// Battlefield for a spell-or-permanent effect (TgtZone$ Stack,Battlefield).
	if targetsStackObjects(sa.Params["TargetType"]) {
		zones = appendUniqueZone(zones, state.ZStack)
	}
	if len(zones) == 0 {
		zones = []state.Zone{state.ZBattlefield}
	}
	return zones
}

// appendUniqueZone appends z to zones when it is not already present,
// preserving the deterministic TgtZone$/TargetType$ order both askTarget and
// legalTargets share (never a map, so no map iteration order reaches a wire
// decision).
func appendUniqueZone(zones []state.Zone, z state.Zone) []state.Zone {
	for _, existing := range zones {
		if existing == z {
			return zones
		}
	}
	return append(zones, z)
}

// targetsStackObjects reports whether a Forge TargetType$ value names a
// target that lives on the stack: a spell (Spell/Instant/Sorcery), or an
// activated/triggered/spell-ability object. The base token precedes any "."
// qualifier (Spell.singleTarget, Instant.singleTarget, ...).
func targetsStackObjects(tt string) bool {
	for _, t := range strings.Split(tt, ",") {
		base, _, _ := strings.Cut(strings.TrimSpace(t), ".")
		switch base {
		case "Spell", "Instant", "Sorcery", "Activated", "Triggered", "SpellAbility":
			return true
		}
	}
	return false
}

// targetName is the object's name for a target prompt, tolerating the ability
// stack object (no Face) a triggered ability's own target ask produces by
// falling back to its source permanent's name.
func (e *Engine) targetName(source state.ObjID) string {
	if o := e.G.Obj(source); o != nil {
		if f := o.Face(); f != nil && f.Name != "" {
			return f.Name
		}
		if s := e.G.Obj(o.Source); s != nil {
			if sf := s.Face(); sf != nil && sf.Name != "" {
				return sf.Name
			}
		}
	}
	// Falling back to the literal word "target" (reviewer minor 7) is
	// deliberate: every caller has already given the decision a usable
	// prompt, so this is only reached for an object with no name at all --
	// an unreadable name there is better than a fabricated one.
	return "target"
}

// protectionSource resolves the object whose characteristics decide whether
// "protection from X" filters out a candidate target or a Damage recipient
// (Task 15 fix round 1, Critical C2): for an ability stack object -- the
// top-of-stack wrapper events.Apply mints via AbilityPush/TriggerPush with no
// Face -- that is the Source permanent that carries the ability, because
// effects.ColorsOf and the Face-type reads in sourceHasQuality both return
// nothing for a Face-less object (CR 606.3: targeting checks the spell or
// ability's source, and for an activated or triggered ability that source is
// the permanent that granted it). For everything else -- a spell's own stack
// object, which IS the card and so carries a Face, or a plain permanent -- it
// is the object itself. This is the single definition both askTarget (which
// in the same round discovered it was passing the Face-less ability object)
// and legalTargets consult, so the two sites always agree on what "the
// source" is.
func (e *Engine) protectionSource(source state.ObjID) state.ObjID {
	if o := e.G.Obj(source); o != nil && o.Ability != nil && o.Source != 0 {
		return o.Source
	}
	return source
}

// describeTargetEffect is intentionally independent of game state and host.
// Only literal damage is known: evaluating Num here would collapse unresolved
// SVars to zero and would mistake a current X/count for a resolution forecast.
func describeTargetEffect(sa *cards.SA) *decision.TargetEffect {
	if sa == nil {
		return nil
	}
	out := &decision.TargetEffect{API: sa.API}
	switch sa.API {
	case "DealDamage", "DamageAll":
		out.Damage = &decision.DamageEffect{}
		// Fixed-width parsing is architecture-independent and safely representable
		// by the wire's JavaScript number. Negative/overflow/missing stay unknown.
		if n, err := strconv.ParseInt(sa.Params["NumDmg"], 10, 32); err == nil && n >= 0 {
			amount := int(n)
			out.Damage.Amount = &amount
		}
	}
	return out
}

type targetCandidate struct {
	kind   string
	obj    state.ObjID
	player state.PlayerID
}

// legalTargetCandidates is the pure target census shared by cast-option
// enumeration and the post-announcement target ask. It reads state in the
// same deterministic order as the old askTarget loops and emits no events.
//
// source is the object the spec's Self/Other predicates and the protection
// test are resolved against (for an ability, the Source permanent).
// excludeSelf is the object a prospective target may not equal -- the CR
// 115.5 self-targeting rule. It is separate from source because during a
// cast/activation proposal the two diverge: a spell on the stack may not
// target itself (excludeSelf == the card), while an activated ability CAN
// target its own Source permanent (excludeSelf == 0, since the Face-less
// ability object is not on the stack yet). Callers set excludeSelf == 0 to
// disable the rule.
func (e *Engine) legalTargetCandidates(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA) []targetCandidate {
	spec := sa.Params["ValidTgts"]
	sc := e.targetSpecContext(source, excludeSelf, p)
	zones := targetZones(sa)
	var out []targetCandidate
	// Players are offered only alongside the default battlefield search and
	// only when the spec can name one. A spec that routes elsewhere
	// (TgtZone$ Graveyard/Hand/Exile) targets objects only -- never a player.
	if len(zones) == 1 && zones[0] == state.ZBattlefield && targetsPlayers(spec) {
		for _, q := range e.G.AliveFrom(0) {
			out = append(out, targetCandidate{kind: "player", player: q})
		}
	}
	// Resolve the source ONCE for the whole census -- for an ability this is
	// the Source permanent, not the Face-less stack object. Every protection
	// test below is guarded on the candidate's zone, because a permanent's
	// static ability functions only on the battlefield (CR 604.3), so a
	// printed protection does not withhold a target sitting in the
	// Graveyard/Hand/Exile that a TgtZone$ spec is asking about.
	protSrc := e.protectionSource(source)
	for _, z := range zones {
		if z == state.ZStack {
			// The stack is a single, shared sequence, not a per-seat zone, so
			// its objects are enumerated ONCE each -- labelled with the
			// object's own controller -- rather than once per alive seat,
			// which would offer the same spell N times in an N-seat game and
			// drift the option list. Face-less ability objects remain outside
			// the targetable population (see the TargetType$ row in AGENTS.md).
			for _, oid := range e.G.Zone(state.ZStack, 0) {
				o := e.G.Obj(oid)
				// CR 115.5: a spell or ability on the stack is an illegal
				// target for itself. This census runs after PutOnStack or
				// AbilityPush, so source is already that object atop the
				// stack -- its own id must never be offered, or a
				// counterspell would counter itself. Only the source OBJECT
				// is excluded, never a different copy of the same card.
				if o == nil || o.Face() == nil || (excludeSelf != 0 && oid == excludeSelf) {
					continue
				}
				if effects.MatchesSpecCtx(e.G, spec, oid, sc) &&
					!(o.Zone == state.ZBattlefield && e.protectedFrom(oid, protSrc)) {
					out = append(out, targetCandidate{kind: "permanent", obj: oid, player: o.Controller})
				}
			}
			continue
		}
		// Hand is the chooser's own hand only (CR 701.15a); the other
		// non-battlefield zones are public, so every seat's slice is offered.
		players := e.G.AliveFrom(0)
		if z == state.ZHand {
			players = []state.PlayerID{p}
		}
		for _, q := range players {
			for _, oid := range e.G.Zone(z, q) {
				o := e.G.Obj(oid)
				// CR 702.16c withholds a permanent protected from the
				// targeting source's qualities; a CantTarget restriction
				// (Vines of Vastwood) withholds one from the spoke player.
				// Both function only on the battlefield (CR 604.3), the same
				// gate as protection above. CR 115.5 excludes the source.
				if o != nil && o.Face() != nil && (excludeSelf == 0 || oid != excludeSelf) &&
					effects.MatchesSpecCtx(e.G, spec, oid, sc) &&
					!(o.Zone == state.ZBattlefield && e.protectedFrom(oid, protSrc)) &&
					!(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(oid, p)) {
					out = append(out, targetCandidate{kind: "permanent", obj: oid, player: q})
				}
			}
		}
	}
	return out
}

// askTarget offers every legal target for a spell or ability. It deliberately
// retains the post-push insufficient-target backstop: modal and dynamic target
// counts are not rejected by the earlier cast-offer census.
func (e *Engine) askTarget(p state.PlayerID, source state.ObjID, sa *cards.SA) {
	min, max := targetBounds(sa)
	d := &decision.Decision{Player: p, Kind: decision.KTarget, Min: min, Max: max,
		Prompt: "Choose a target for " + e.targetName(source),
		Source: source, TargetEffect: describeTargetEffect(sa)}
	for _, candidate := range e.legalTargetCandidates(p, source, source, sa) {
		label := e.G.Players[candidate.player].Name
		if candidate.obj != 0 {
			label = e.G.Obj(candidate.obj).Face().Name + " (" + label + ")"
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: label, Obj: candidate.obj, Player: candidate.player})
	}
	if min == 0 {
		// Requirement N2 / totality: a target-hungry subject whose minimum
		// is zero resolves untargeted when NO legal target exists. When at
		// least one exists it is still offered (Min 0 lets the chooser take
		// none); a zero-option decision would strand the cast, never asked.
		if len(d.Options) == 0 {
			return
		}
	} else if len(d.Options) < min {
		// A target-hungry subject with fewer legal targets than Min uses CR
		// 608.2b's existing counter/fizzle exit: an immediate move to its
		// normal resting place (exile instead of the graveyard for a
		// Flashback cast, and for a triggered ability object -- which has no
		// graveyard -- exile per CR 608.2m, same as every ability fizzle in
		// resolveTop).
		rest := spellRestZone(e.G.Obj(source))
		if o := e.G.Obj(source); o != nil && o.Ability != nil {
			rest = state.ZExile
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: source,
			From: state.ZStack, To: rest, Text: "countered: no legal targets"})
		e.ensureLeftTheStack(source, rest, "a replacement fully discarded this "+
			"spell's 'countered: no legal targets' move without relocating it anywhere; sent "+
			"to its resting zone instead of re-resolving forever")
		// Ruling T14-e: p, the casting player, not e.G.Active -- CR 117.3c,
		// the caster keeps priority even when it fizzles. Only a SPELL keeps
		// priority this way: an ability fizzling is parked in exile (ceases
		// to exist) mid-drain, and handing out priority from inside the
		// trigger drain would double-emit against the drain's own tail.
		if o := e.G.Obj(source); o == nil || o.Ability == nil {
			e.emit(events.Event{Kind: events.Priority, Player: p, Amount: 0})
		}
		return
	}
	e.ask(d)
}

// handleTarget records the chosen target(s) via TargetsChosen events (Ruling
// T14-b) rather than writing state.Object.Targets directly: apply.go clears
// Targets on a zone change but nothing else ever set it, so a direct write
// here would leave a replayed game with no targets while the live game had
// them. N targets record as TargetMin$ asks for: the first option replaces
// with targets (TargetsChosen shape 0 for an object, 1 for a player), each
// further option appends one (shape 2 object, 3 player) -- see apply.go's
// TargetsChosen case and its test TestTargetsChosenAppendShapes.
func (e *Engine) handleTarget(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	// A cast-flow target decision (CR 601.2c, asked by targetAsk after the
	// object was pushed by pushCast but BEFORE any cost is paid): completing
	// it means recording the chosen targets onto the stack object and then
	// committing the transaction -- paying the costs and firing the cast
	// trigger -- via payCast. For a spell the stack object is the card
	// itself, already on the stack (pushCast), so targets are recorded before
	// payment (601.2c before 601.2h); for an activated ability the stack
	// object is minted by payCast's AbilityPush, so targets are recorded
	// AFTER it. Targets are never written directly (they go through
	// TargetsChosen events) and always after the push, because a zone change
	// clears them.
	if e.cast != nil {
		pc := e.cast
		if pc.ability < 0 {
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen)
			}
			e.payCast()
		} else {
			e.payCast()
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen)
			}
		}
		if e.drainAwaitsTarget {
			e.drainAwaitsTarget = false
			e.resumeTriggerDrain()
		} else if e.pending == nil {
			// CR 117.3c: the caster keeps priority after a completed cast.
			// payCast can pose a MID-CAST ask (CR 601.2g's mana window, a
			// cost choice) which leaves the engine parked on that question;
			// the casting player does not keep priority until the cast has
			// actually paid every cost and fired its cast trigger, so the
			// "caster keeps priority" marker is emitted only once no further
			// announcement decision is outstanding. Emitting it while parked
			// is the same class of log lie as the pass-branch emit this task
			// removed: the log would assert the caster held priority at a
			// moment the engine is waiting on an unanswered question.
			e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
		}
		return
	}
	e.recordChosenTargets(d.Source, chosen)
	// A target decision asked by a trigger drain (putTriggersOnStack's
	// pushTrigger, immediately after the trigger's TriggerPush -- Task 20's
	// checkTriggers never asked targets, so only a spell's cast-time ask
	// reached here before) does not hand out priority: the drain's own tail
	// does, through the SAME continuation handleTriggerOrder uses
	// (resumeTriggerDrain, turn.go), so a second, later trigger is still
	// placed before any player acts. A spell's cast-time target decision
	// keeps its caster's priority (CR 117.3c) via the emit below.
	if e.drainAwaitsTarget {
		e.drainAwaitsTarget = false
		e.resumeTriggerDrain()
		return
	}
	// Ruling T14-e: the submitting player, not e.G.Active -- CR 117.3c, the
	// player who chose the target (the caster) keeps priority.
	e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
}

// recordChosenTargets emits the TargetsChosen events for a set of chosen
// target options onto the given object. Amount discriminates the target shape
// (Ruling T14-b, extended by Task 4): 0 replace-with-object, 1
// replace-with-player, 2 append-object, 3 append-player. It is the shared
// recording path for both a cast-flow target decision (onto the stack object,
// after the push) and a triggered ability's own post-TriggerPush ask.
func (e *Engine) recordChosenTargets(targetObj state.ObjID, chosen []decision.Option) {
	for i, opt := range chosen {
		ev := events.Event{Kind: events.TargetsChosen, Obj: targetObj}
		if opt.Kind == "player" {
			// shape 1 replace / shape 3 append a single player target.
			ev.Amount = 1
			if i > 0 {
				ev.Amount = 3
			}
			ev.Player = opt.Player
		} else {
			// shape 0 replace / shape 2 append one object target.
			if i > 0 {
				ev.Amount = 2
			}
			ev.IDs = []state.ObjID{opt.Obj}
		}
		e.emit(ev)
	}
}

// resolveTop resolves the object on top of the stack and moves it to
// wherever it goes next.
//
// CR 608.2b: before anything else runs, every target recorded when this
// spell or ability was put on the stack is rechecked against legalTargets
// below -- a target legal when chosen can stop being legal by the time its
// spell reaches the top of the stack, most commonly a creature that died to
// something else in the meantime. With no legal target left, this does not
// resolve at all: it leaves the stack (a spell to its owner's graveyard, an
// ability to exile per CR 608.2m just below) with no effect -- not even a
// SubAbility chained onto it that names no target of its own (Defined$ You
// and the like). That distinguishes this from the per-target nil-checks
// primitives like effDealDamage already had (Task 18, effects/damage.go):
// those already skip a target that individually vanished, but nothing
// before this stopped the OTHER, untargeted parts of the same spell's
// script from running anyway once every target it had was gone. With only
// some targets still legal, resolution proceeds against exactly that
// narrowed set -- CR 608.2b's "resolves, doing as much as possible".
func (e *Engine) resolveTop() {
	id := e.G.Stack[len(e.G.Stack)-1]
	o := e.G.Obj(id)

	if o.Ability != nil {
		// A triggered or activated ability with no printed card: Ruling
		// T14-c / F3 -- Face() returns nil for these, so this branch must
		// run before anything below touches it. Task 20 is what actually
		// puts objects like this on the stack.
		//
		// CR 603.4 (intervening-if): a triggered ability's condition is
		// checked both when it would trigger AND again as it resolves; if
		// it no longer holds, the ability is removed from the stack and does
		// nothing. The trigger was queued because triggerConditionHolds was
		// true at trigger time (triggerMatches), but Scute Mob's "whenever
		// you control five or more lands" can be false by the time the
		// ability resolves -- here a real instant response destroyed one of
		// those lands. findTriggerForAbility identifies the T: line from the
		// SA pointer: it is false for an activated ability (whose SA comes
		// from Abilities, not a Triggers entry) so this recheck never
		// applies to one, and false for a source whose face has changed or
		// gone so a trigger-only rule never fires on unknown provenance.
		// The fizzle move is the same exile rest the ability branch uses for
		// CR 608.2b's no-legal-targets case: an ability "ceases to exist"
		// (608.2m) rather than moving to a card zone, and this build parks
		// such objects in exile. Ordered first because it decides whether
		// the ability does anything at all.
		if t, ok := e.findTriggerForAbility(o.Source, o.Ability); ok {
			if !e.triggerConditionHolds(t, o.Source) {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZExile, Text: "fizzled: intervening-if no longer holds"})
				e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this "+
					"ability's 'intervening-if' move without relocating it anywhere; sent to exile "+
					"instead of re-resolving forever")
				return
			}
		}
		// No triggered or activated ability this build produces ever
		// actually populates Targets (only Remembered): Task 20's
		// checkTriggers never calls askTarget, which is the only place
		// TargetsChosen is ever emitted from. So this is unreachable in
		// practice today, but a stack object is a stack object, and CR
		// 608.2b's "spell or ability" covers this shape too if a later
		// task ever gives a triggered ability a player-chosen target.
		targets := o.Targets
		// Fix round 2 (re-review N1): the gate is `spec != ""` -- "this
		// ability declares a targeting requirement" -- not `len(targets) > 0`
		// -- "this ability happens to have targets right now". The old form
		// used the latter as a proxy for the former, so an ability that NEEDS
		// a target but has none recorded skipped CR 608.2b's recheck entirely
		// and resolved. Zero recorded targets is zero LEGAL targets, which is
		// exactly what 608.2b counters.
		// Requirement N2: an ability that MAY target zero things (TargetMin$ 0)
		// and has none recorded resolves untargeted rather than fizzling --
		// targetMin(o.Ability)==0 && len(targets)==0 is the exemption.
		if spec := o.Ability.Params["ValidTgts"]; spec != "" && !(targetMin(o.Ability) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, spec, targetZones(o.Ability), o.Controller, o.Source, id)
			if len(legal) == 0 {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZExile, Text: "fizzled: no legal targets remain"})
				e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this "+
					"ability's 'fizzled: no legal targets' move without relocating it anywhere; "+
					"sent to exile instead of re-resolving forever")
				return
			}
			targets = legal
		}
		// CR 608.2m: a resolved ability just ceases to exist rather than
		// moving to a card zone. This build has no "ceases to exist" zone,
		// so it is parked in exile as the closest existing approximation.
		e.emit(events.Event{Kind: events.Resolve, Obj: id})
		// CR 603.5: an optional triggered ability goes on the stack regardless
		// (putTriggersOnStack pushes it unconditionally), and its controller
		// -- or whatever seat its OptionalDecider$ names -- chooses whether to
		// apply the effect as the ability resolves. With the Resolve event
		// already logged, pose that yes/no now and SPEND the resolution: a
		// yes re-enters it (resumeResolution runs the effect, exactly as the
		// tail below would have), a no lets the ability leave the stack
		// having done nothing. A decider who has left the game is nobody to
		// apply an effect to, so the ability ceases to exist (CR 800.4a) and
		// is parked in exile like the other ceased-to-exist rests. Activated
		// abilities and mandatory triggers (findTriggerForAbility returns
		// false for the former, or an OptionalDecider-less trigger for the
		// latter) fall straight through to their effect below.
		if t, ok := e.findTriggerForAbility(o.Source, o.Ability); ok {
			if spec := t.Params["OptionalDecider"]; spec != "" {
				who, askable := e.deciderFromSpec(spec, o.Controller, o.Remembered)
				if !askable {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZStack, To: state.ZExile, Text: "ceased to exist: its optional decider left the game"})
					e.ensureLeftTheStack(id, state.ZExile, "the optional decider of this ability left the game, so the "+
						"ability ceased to exist (CR 800.4a) and was parked in exile")
					return
				}
				e.askOptionalAtResolution(who, o, o.Ability, e.abilityLabel(o, t))
				return
			}
		}
		// The ability object itself has no Face, so its SVar table (needed
		// for Num's SVar indirection, e.g. Goblin Piledriver's "NumAtt$ +X")
		// comes from the permanent that granted it (o.Source) instead.
		// SVars are static card-script text that never changes after
		// parsing, so reading them live from the source's current Face at
		// resolution time is equivalent to a snapshot taken when the
		// trigger was queued, with no need for a new field to carry one
		// through the stack. A source that has since left the battlefield
		// (or ceased to exist) has nothing to read here and degrades to a
		// nil SVar table, same as before this ability object existed at
		// all, rather than panicking.
		var svars map[string]string
		if src := e.G.Obj(o.Source); src != nil {
			if sf := src.Face(); sf != nil {
				svars = sf.SVars
			}
		}
		// Ruling T20-b: Source must be o.Source (the permanent that has this
		// ability), not id (the transient stack-object wrapper) -- Defined$
		// Self, the most common Defined$ value in real trigger scripts,
		// resolves to Ctx.Source, and a wrapper ID means "Self" refers to a
		// stack object with no Face() that leaves play the instant this
		// resolves, so the effect would silently apply to nothing. The SVar
		// lookup two lines above already gets this right by reading from
		// o.Source; this was a one-line inconsistency, not a second design.
		ctx := &effects.Ctx{Source: o.Source, Controller: o.Controller,
			Targets: targets, Remembered: o.Remembered, TriggerContext: e.triggerContexts[id]}
		effects.SetSVars(ctx, svars)
		// CR 603.3c: the mode choice was announced at placement (pushTrigger
		// asked KModes and handleModes recorded the answer into ChosenModes).
		// Pre-seeding Ctx.Modes makes effCharm take its re-entry branch and
		// run exactly the chosen modes rather than asking again at
		// resolution. Nil for a non-modal trigger, for an activated ability,
		// and for any trigger the placement ask never reached.
		ctx.Modes = o.ChosenModes
		e.damaging = o.Source
		e.contChain = e.contChain[:0]
		effects.Resolve(e, ctx, o.Ability)
		e.damaging = 0
		if e.resume != nil {
			// A placement-announced modal ability can reach a nested ask during
			// this initial pass. Preserve every enclosing continuation exactly as
			// resumeResolution does for a nested ask reached on re-entry.
			e.resume.outer = e.buildContinuationChain(e.contChain, id, nil)
			// A mid-resolution ask (M2d-2): the effect that asked has set a
			// decision pending and recorded a resume point. The object stays
			// on the stack waiting for the answer -- entering the exile exit
			// below would discard it mid-resolution. The answered decision
			// re-enters the suspended effect through resumeResolution
			// (rules/resolution.go), which runs the rest of this same tail.
			return
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
		e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this resolved "+
			"ability's own move off the stack without relocating it anywhere; sent to exile "+
			"instead of re-resolving forever")
		return
	}

	f := o.Face()
	sa := f.SpellAbility()
	targets := o.Targets
	if sa != nil {
		// A modal spell's target declaration lives on its announced mode SVar,
		// not the outer Charm SA. Use the same selected declaration targetAsk
		// used during CR 601.2c, so its targets receive the ordinary CR 608.2b
		// legality recheck at resolution.
		targetSA := modalTargetSA(f, sa, o.ChosenModes)
		// Fix round 2 (re-review N1), the same correction as the ability
		// branch above, and the one that was actually reachable. Widening the
		// departed-player release hook in fix round 1 turned a stall into a
		// spell that RESOLVES with no targets at all: the caster is
		// eliminated while their target decision is outstanding, the hook
		// clears it so the match can continue, and the spell is left on the
		// stack with Targets nil. Under the old `len(targets) > 0` gate that
		// skipped the recheck and ran the whole script -- untargeted riders
		// included -- gaining the ELIMINATED caster 7 life in the re-review's
		// own reproduction. CR 608.2b counters a spell with no legal targets;
		// CR 800.4a says a departed player's spell ceases to exist. Neither
		// permits it to resolve.
		// Requirement N2, the same exemption as the ability branch: an
		// untargeted-with-Min-0 spell resolves rather than fizzling.
		if spec := targetSA.Params["ValidTgts"]; spec != "" && !(targetMin(targetSA) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, spec, targetZones(targetSA), o.Controller, id, id)
			if len(legal) == 0 {
				// CR 608.2b: every target became illegal. This spell does
				// not resolve -- no Resolve event, no script runs -- it goes
				// straight to its normal resting place, the same zone it
				// would reach after an ordinary resolution (an Aura instead
				// reaching the battlefield would be wrong here: CR 704.5m's
				// "nothing legal to attach to" is exactly this case for
				// that spell shape, and the resting zone is where it
				// belongs) -- exile instead of the graveyard for one cast
				// via Flashback (CR 702.32b).
				rest := spellRestZone(o)
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: rest, Text: "fizzled: no legal targets remain"})
				e.ensureLeftTheStack(id, rest, "a replacement fully discarded this "+
					"spell's 'fizzled: no legal targets' move without relocating it anywhere; "+
					"sent to its resting zone instead of re-resolving forever")
				return
			}
			targets = legal
		}
	}
	e.emit(events.Event{Kind: events.Resolve, Obj: id, Text: f.Name})
	if sa != nil {
		e.damaging = id
		ctx := &effects.Ctx{Source: id, Controller: o.Controller, Targets: targets}
		effects.SetSVars(ctx, f.SVars)
		// CR 601.2b: a modal spell's choice was recorded on its proposal before
		// targets and payment. Pre-seeding Modes makes effCharm execute exactly
		// that announcement instead of posing its old resolution-time ask.
		ctx.Modes = o.ChosenModes
		e.contChain = e.contChain[:0]
		effects.Resolve(e, ctx, sa)
		e.damaging = 0
		if e.resume != nil {
			// The cast-announced outer mode may itself contain an asking effect.
			// This is an initial resolution pass rather than a resume re-entry,
			// but its enclosing SubAbility continuations have the same lifetime.
			e.resume.outer = e.buildContinuationChain(e.contChain, id, nil)
			// A mid-resolution ask (M2d-2): same as the ability branch above
			// — the resolution is suspended with the object still on the
			// stack, and the answered decision re-enters it through
			// resumeResolution instead of this tail.
			return
		}
	}
	e.moveResolvedOffStack(o)
}

// spellRestZone is where a resolved (or fizzled) spell goes instead of the
// graveyard: exile for one cast via Flashback (CR 702.32b), the graveyard
// for everything else. o is read before the MoveZone that takes it off the
// stack, so its CastFlags (set by commitCast's CastInfo, if any) still
// reflect how it was cast.
func spellRestZone(o *state.Object) state.Zone {
	if o != nil && (o.CastFlags&state.FlagFlashback != 0 || o.IsCopy) {
		return state.ZExile
	}
	return state.ZGraveyard
}

// ensureLeftTheStack is CR 608.2m housekeeping, not a further game action:
// every one of resolveTop's five exits (the permanent-ETB Move above, the
// instant/sorcery Move above, and the three fizzle/ability-resolution Moves
// earlier in this function) emits a MoveZone meant to take id off the stack
// for good. If a ValidCard$-matching, ReplacementResult$-absent (this
// build's "Replaced") R:Event$ Moved replacement on some OTHER object
// intercepts that specific MoveZone and its own ReplaceWith$ does not itself
// relocate the card (e.g. it only gains life), nothing else ever removes id
// from e.G.Stack: the next priority round finds the same object on top and
// resolves it again, forever. Note this is NOT the shipped corpus's own
// Rest in Peace / Dryad Militant / Leyline of the Void shape: that shape's
// ReplaceWith$ names Defined$ ReplacedCard, which effects/context.go's
// Defined DOES model, and with ChangeZone's Origin$ All parsed as a real
// wildcard (effects/zone.go's ParseZones) it genuinely relocates the card
// to exile -- so a working Rest-in-Peace-shaped replacement takes the object
// off the stack itself and the guard never engages. The guard is for the
// replacements whose ReplaceWith$ truly leaves the card where it was.
//
// Originally added by Task 29 (Ruling T26-a) for the permanent-ETB exit
// alone; the final whole-branch review (Critical C1) measured the identical
// hazard, independently reachable, on the other four exits -- a two-card
// deck (an instant or an ability-granting permanent, plus a Rest-in-Peace-
// shaped enchantment already on the battlefield) reliably hung a real match
// at 100 000+ intents, never terminating, on turn 1. 24 corpus cards this
// build's own coverage gate already calls fully playable carry the shape
// (Rest in Peace, Dryad Militant, Leyline of the Void, ...), so this is
// reachable from ordinary deck-building, not adversarial card text. This
// helper is what makes every exit safe with one implementation instead of
// five copies of the same six lines.
//
// Held under applyingReplacement (saved and restored, not just set, in case
// a future caller ever reaches here already inside one) for the same reason
// the original ETB guard was (Task 29 review finding I-1): "ceases to
// exist" is engine bookkeeping, not a game event a card's own replacement
// gets to intercept a second time. why becomes the Note's Text, tailored by
// each call site to name which of the five moves was actually being
// guarded.
func (e *Engine) ensureLeftTheStack(id state.ObjID, to state.Zone, why string) {
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
		return
	}
	// fx44: a SUSPENDED replacement is legitimately still on the stack,
	// waiting for its answer — the ask it posed decides where the object goes
	// (Mox Diamond's ETB replacement parks the card exactly here). The guard
	// must distinguish "the replacement finished and moved nothing" (the
	// ordinary re-resolve hazard below, where e.resume is nil) from "the
	// replacement is waiting for an answer" (where the object must stay put
	// so resumeResolution can act on it). Deferring the park while suspended
	// leaves the object on the stack; the answered resume performs the real
	// move, and finishResumption's own o.Zone != state.ZStack check skips
	// this guard entirely once that move has happened.
	if e.Suspended() {
		return
	}
	saved := e.applyingReplacement
	e.applyingReplacement = true
	e.emit(events.Event{Kind: events.Note, Obj: id, Text: why})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: to})
	e.applyingReplacement = saved
}

// legalTargets is CR 608.2b's recheck, applied at resolution: the subset of
// targets that are still legal right now. An object target must still be in
// a zone the spec legitimately targets -- zones, computed by the same
// targetZones the offering askTarget used, is what no-longer-hardcodes the
// battlefield -- because matchesBase's own bare-type predicates (effects/
// filter.go), e.g. "Creature", read printed types straight off the Face with
// no zone check of their own, so a creature that died and is sitting in a
// graveyard would otherwise still look like a match. The old form required
// o.Zone == state.ZBattlefield, which made every non-battlefield legal
// target offered by askTarget (a stack spell for a counterspell; a
// Graveyard card for the Snapcaster shape) fizzle at resolution. It still
// satisfies spec, via the same effects.MatchesSpec call askTarget used to
// offer it as an option in the first place: no self-relative source,
// matching askTarget's own simplification, so a spec that would filter on
// Self/Other is exactly as (im)precise here as it was at cast time. A player
// target is legal for as long as they are still in the game; askTarget never
// applies MatchesPlayerSpec's finer You/Opponent distinction when it first
// offers every living player as an option (targetsPlayers below it), so this
// does not either -- rechecking against a filter the engine never enforced
// when the target was chosen would reject targets this build always
// considered fine.
func (e *Engine) legalTargets(targets []state.Target, spec string, zones []state.Zone, you state.PlayerID, source state.ObjID, self state.ObjID) []state.Target {
	var legal []state.Target
	// The resolution recheck, unlike a target offer, has this stack object's
	// Targets available. Targeted* predicates may read precisely this binding;
	// setting it here keeps their self-reference unavailable at announcement.
	sc := e.targetSpecContext(0, self, you)
	sc.ResolutionTargets = targets
	sc.Resolving = true
	for _, t := range targets {
		if t.IsPlayer {
			if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost {
				legal = append(legal, t)
			}
			continue
		}
		// CR 115.5: the resolving spell or ability is an illegal target for
		// itself. self is the stack object being resolved (not source, which
		// for an ability is the source PERMANENT and so is a legal target of
		// its own ability -- e.g. a creature's "target creature" ability on
		// itself). A target chosen at cast time for a different spell -- a
		// different copy of the same card, or a permanent -- is unaffected.
		// This is why the exclusion is keyed on the resolving object id, and
		// mirrors askTarget's own withholding so the two sites always agree.
		if t.Obj == self {
			continue
		}
		// CR 702.16c: a permanent that became protected from the resolving
		// source's qualities since the target was chosen is no longer a legal
		// target, exactly as askTarget withheld it at cast time -- so a
		// previously-offered target that gained matching protection mid-race
		// is dropped from resolution too (CR 608.2b). The source is resolved
		// through protectionSource so an ability fizzling here judges "the
		// source" as its Source permanent, the same object askTarget's own
		// filter has now been made to see (Critical C2 -- one definition).
		if o := e.G.Obj(t.Obj); o != nil && zoneIn(o.Zone, zones) &&
			effects.MatchesSpecCtx(e.G, spec, t.Obj, sc) &&
			!(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(t.Obj, you)) &&
			!e.protectedFrom(t.Obj, e.protectionSource(source)) {
			legal = append(legal, t)
		}
	}
	return legal
}

// zoneIn reports whether z is one of the zones in the set.
func zoneIn(z state.Zone, zones []state.Zone) bool {
	for _, candidate := range zones {
		if candidate == z {
			return true
		}
	}
	return false
}

// resolveAbility walks an SA chain, running each API's implementation. svars
// is the resolving face's SVar table (nil for an ability-only stack object),
// which is what lets Num's SVar indirection and primitives like Charm and
// Repeat -- which run a sub-ability named by SVar rather than the
// auto-linked "SubAbility$" -- actually resolve something outside a test.
func (e *Engine) resolveAbility(source state.ObjID, controller state.PlayerID,
	targets []state.Target, sa *cards.SA, svars map[string]string) {
	ctx := &effects.Ctx{Source: source, Controller: controller, Targets: targets}
	effects.SetSVars(ctx, svars)
	effects.Resolve(e, ctx, sa)
}

// Game, Emit and Rand satisfy effects.Host, which is how effects reach the
// engine without importing it. AddContinuous (layers.go), HasKeyword
// (layers.go) and Ask (resolution.go) round out the interface -- HasKeyword
// already existed for the layer system's own callers before effects.Host
// grew a method of the same name, and needed no change to satisfy it.
func (e *Engine) Game() *state.Game    { return e.G }
func (e *Engine) Emit(ev events.Event) { e.emit(ev) }
func (e *Engine) Rand(n int) int       { return e.rng.IntN(n) }

// CastThisTurn satisfies effects.Host's CastThisTurn for Count$ThisTurnCast
// (Task 17/Storm): the spells cast this turn by ANY player, counted from
// the same event log spellsCastThisTurn reads, so a replay that rebuilds the
// game derives the identical number -- never a live-only engine counter.
// Storm subtracts one (Count$ThisTurnCast/Minus1) because the resolving
// spell's own PutOnStack is already in the log by the time its trigger
// effect runs, which would otherwise overcount by exactly one.
func (e *Engine) CastThisTurn() int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.PutOnStack {
			n++
		}
	}
	return n
}

// targetsPlayers and targetsPermanents read the coarse shape of a ValidTgts
// spec. The per-object predicate work is effects.MatchesSpec.

// targetsPlayers reports whether a spec can name a player as a target, by
// looking at each alternative's BASE type (the part before any "."), never
// a substring scan: "Any" targets either an object or a player, "Player"/
// "Opponent"/"You" a player, while a predicate like "Creature.YouCtrl" IS an
// object filter (its ".YouCtrl" clause scopes the creature's controller,
// not the target being a player). The old substring form matched the "You"
// inside "YouCtrl" and wrongly offered players as Equip targets (Task 14
// found it: an Equip onto Creature.YouCtrl offered both players as options,
// which effAttach then had to refuse).
func targetsPlayers(spec string) bool {
	for _, alt := range strings.Split(spec, ",") {
		switch base, _, _ := strings.Cut(strings.TrimSpace(alt), "."); base {
		case "Player", "Any", "Opponent", "You":
			return true
		}
	}
	return false
}

func targetsPermanents(spec string) bool {
	for _, t := range [...]string{"Creature", "Any", "Permanent", "Artifact",
		"Enchantment", "Land", "Planeswalker", "Card"} {
		if strings.Contains(spec, t) {
			return true
		}
	}
	return false
}
