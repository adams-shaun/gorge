// Package rules: state-based actions (CR 704) and game end (CR 104.4).
//
// checkStateBased, checkGameOver and destroyLethalDamage all predate this
// file: Task 21 added destroyLethalDamage (combat depends on lethal-damage
// and zero-toughness destruction, and nothing in this codebase checked
// either before then), and stubs.go carried checkStateBased/checkGameOver
// as a placeholder ever since Task 3 -- a checkGameOver call and nothing
// else. This task deletes stubs.go outright and relocates all three here
// rather than losing them with it, and fills in the state-based actions M1
// actually specifies: a player at 0 or less life loses (CR 704.5a); an
// eliminated player's permanents leave the battlefield; the game ends in a
// win when exactly one seat remains and a draw (CR 104.4a) when none do --
// not, as stubs.go's own checkGameOver used to, an unconditional win for
// seat 0 when nobody survived. An attempted draw from an empty library
// (CR 121.4) only marks the player (events.DrawEmpty); the CR 704.5b loss is
// emitted here, at the next check, beside 704.5a's life loss and 704.5c's
// poison loss, so simultaneous losses end the game in a draw (CR 104.4a).

package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// maxSBAPasses bounds the fixed-point loop below. A game that needs more
// passes than this has a rules bug (a static-effect cycle where destroying
// one creature always makes another lethal, forever), and reporting a stuck
// game via a bounded loop is better than hanging the one goroutine running
// the whole match.
const maxSBAPasses = 32

// sbaAttempts is one checkStateBased call's memory of what its
// state-based-action passes have already tried, together with the size of
// the alive-player set they tried it under.
//
// objs is destroyLethalDamage's and the zero-loyalty and defeat scans', players is
// sweepDepartedPlayers', and tokens is ceaseDeadTokens' own. All are membership-only
// -- nothing ever ranges over any of them, so none can reach an event or the order
// of a decision's options. tokens is deliberately its OWN map: a token that is also a
// creature can be attempted by destroyLethalDamage and later need ceasing in the
// graveyard, and sharing a map would block the second. alive is rearm's watermark.
type sbaAttempts struct {
	objs     map[state.ObjID]bool
	tokens   map[state.ObjID]bool
	players  map[state.PlayerID]bool
	sagas    map[state.ObjID]bool
	dungeons map[state.ObjID]bool
	alive    int
	// skips counts, for the current pass, the candidates an attempt memory
	// above made the pass skip. A stable final pass with none skipped is
	// exactly what a fresh call's first pass would see, so the run is quiet
	// even if earlier passes emitted (sbaRecordQuiet).
	skips int
}

// rearm forgets every memory when the alive-player set has shrunk since it was
// last refreshed, so an attempt that a now-eliminated player's permanent blocked
// gets exactly one more chance under the smaller alive set -- and none at all
// while nobody dies. checkStateBased calls it before a pass scans and again after
// the pass's own eliminations, so the batch's attempts are recorded under the alive
// set they are made under and are not re-armed by those eliminations.
//
// Player.Lost is monotone, so alive never grows and this can fire at most
// len(Players) times per checkStateBased call: the loop stays bounded by work, not
// just by maxSBAPasses.
//
// The alive count closes one of the two ways a blocker can go inert mid-call:
// applyReplacements discovers replacements through forEachObject, which walks
// AliveFrom(0), so a replacement an eliminated player owns stops being consulted
// the instant they are marked Lost. It does not close the other: a replacement also
// goes inert by ceasing to match, because replacementMatches reads the MOVING
// object's flags, counters, zone and controller, every one of them writable by a
// ReplaceWith$ body. That residue self-heals on the next checkStateBased call.
//
// Do not widen the key by re-arming on "some other state-based action succeeded",
// or on any signal a removal produces: a death chain then buys one re-arm per link,
// and a blocked removal sweep goes from a flat 2 firings per Submit to 2/3/7/22/61
// for chain lengths 0/1/5/20/60. TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain
// catches it. A replacement that blocks an attempt forever is a legitimate fixed
// point: the attempted sets make the pass report no new work and the loop goes
// stable, silently, without exhausting maxSBAPasses.
// markTried records k in the attempt memory *m, allocating it on first use.
func markTried[K comparable](m *map[K]bool, k K) {
	if *m == nil {
		*m = make(map[K]bool)
	}
	(*m)[k] = true
}

func (a *sbaAttempts) rearm(alive int) {
	if alive >= a.alive {
		return
	}
	a.alive = alive
	clear(a.objs)
	clear(a.tokens)
	clear(a.players)
	clear(a.sagas)
}

// sbaLoss is one player CR 704.5a-c or 903.10 finds to have lost in a check.
type sbaLoss struct {
	player state.PlayerID
	text   string
}

// sbaBatch is what one state-based-action check found. CR 704.3 checks every
// condition, then performs everything it found "simultaneously as a single
// event", and CR 704.8 derives last known information from the state before any
// of it was performed. So each family below only reads the board and queues
// here; applySBABatch emits nothing until every family has read the same board.
type sbaBatch struct {
	losses []sbaLoss
	// dead is CR 704.5f-h: "lethal damage" casualties take the destruction
	// replacements, "toughness <= 0" ones are a plain move.
	dead []casualty
	// acts is every other action, in family order.
	acts []events.Event
	// counters is CR 704.5q. Applied after every departure: a creature that dies
	// in this check leaves with the counters it had when the check was made.
	counters []events.Event
}

func (b *sbaBatch) add(ev events.Event) { b.acts = append(b.acts, ev) }

func (b *sbaBatch) loses(p state.PlayerID) bool {
	for i := range b.losses {
		if b.losses[i].player == p {
			return true
		}
	}
	return false
}

func (b *sbaBatch) empty() bool {
	return len(b.losses) == 0 && len(b.dead) == 0 && len(b.acts) == 0 && len(b.counters) == 0
}

// departures lists the permanents the batch would move off the battlefield.
func (b *sbaBatch) departures() []state.ObjID {
	var ids []state.ObjID
	for i := range b.dead {
		ids = append(ids, b.dead[i].id)
	}
	for i := range b.acts {
		if ev := &b.acts[i]; ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
			ids = append(ids, ev.Obj)
		}
	}
	return ids
}

// applySBABatch performs the batch's board actions as one event: one look-back
// board and one zone, sacrifice and replacement batch across every family. An
// earlier member of the batch can have already moved an object a later one
// names (a creature-planeswalker with lethal damage and no loyalty), so each
// action is rechecked against where its object still is.
func (e *Engine) applySBABatch(b *sbaBatch) {
	if ids := b.departures(); len(ids) > 0 {
		e.BatchDepartures(ids)
		defer e.EndBatchDepartures()
	}
	for _, c := range b.dead {
		if o := e.G.Obj(c.id); o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if c.text == "lethal damage" && e.replaceSBADestruction(c.id) {
			continue
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: c.id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: c.text})
	}
	for _, ev := range b.acts {
		if !e.sbaActStale(ev) {
			e.emit(ev)
		}
	}
	for _, ev := range b.counters {
		if !e.sbaActStale(ev) {
			e.emit(ev)
		}
	}
}

// replaceSBADestruction offers a lethal-damage destruction to the replacements
// the Destroy effects consult: the regeneration shield, a shield counter (CR
// 122.1c), then umbra armor (CR 702.90), which removes the bearer's damage so
// the next check cannot re-kill it.
func (e *Engine) replaceSBADestruction(id state.ObjID) bool {
	return effects.ReplaceDestruction(e, id) || effects.ReplaceShieldCounter(e, id) ||
		effects.ReplaceUmbraArmor(e, id)
}

// sbaActStale reports that an earlier action in the batch already moved the
// object this one is about.
func (e *Engine) sbaActStale(ev events.Event) bool {
	o := e.G.Obj(ev.Obj)
	switch {
	case o == nil || ev.Kind == events.DungeonComplete || ev.Kind == events.DungeonRemove:
		return false
	case ev.Kind == events.MoveZone:
		return o.Zone != ev.From
	}
	return o.Zone != state.ZBattlefield
}

// checkStateBased applies state-based actions until none apply, which is
// what CR 704.3 requires: they are checked and rechecked, not just once.
// TestSBALoopsUntilStable is the regression test for the "not just once"
// half -- a lord dying can make a creature its own static was keeping alive
// newly lethal, and that second creature must not survive to see a second,
// separate checkStateBased call before it is caught.
//
// One pass is one CR 704.3 check: every family reads the board and queues into an
// sbaBatch, then the batch applies as one event.
//
// A pass reports new work from what it attempted, not from what actually moved.
// Engine.emit runs replacement effects first (rules/trigger.go), so a move this
// file proposes can be replaced outright and the permanent stay where it was;
// judging by the outcome made every pass "find" that permanent lethal again and
// spend the whole maxSBAPasses budget, on every call for the rest of the match. The
// attempt memories (sbaAttempts) bound it to one attempt per object or player per
// call, and "changed" means an attempt was made, so a later pass still gets to see
// whatever a replacement's substitute effect changed elsewhere (a life loss that
// drives a player to 0). Whether an attempt succeeds also depends on who is alive,
// since replacements are only consulted for living controllers, so tried.rearm gives
// a blocked attempt one retry when a player leaves; see rearm before widening that
// key. If maxSBAPasses is nonetheless exhausted (a genuine cycle of new work, not a
// replacement), a Note event says so rather than the game quietly carrying on with
// state-based actions outstanding.
func (e *Engine) checkStateBased() {
	// A parked CR 704.5j batch whose ask is no longer outstanding re-poses it
	// before any SBA work: the batch is parked only together with its ask, and
	// an outstanding ask is the ONE reason the pass below refuses to apply. A
	// caller that posed its own decision at an unguarded point (the pre-fix
	// completeCombatPass tail) could displace the ask; without this re-pose the
	// parked batch would never be answered and -- because the pass below halts
	// while it is parked -- no state-based action would ever apply again. The
	// re-pose restores exactly the invariant parkLegendChoice establishes:
	// parked implies asked. It does not fire while another decision is
	// outstanding (nothing is re-asked under one); a stale flow marker in
	// e.choosing is deliberately NOT consulted, because the displaced ask left
	// e.choosing == chooseLegend behind and askLegendChoice re-sets it anyway.
	if e.legendBatch != nil && e.pending == nil {
		e.askLegendChoice()
		return
	}
	stable := false
	// The attempt memories start nil and are allocated on their first mark
	// (markTried): a read of a nil map is false, so the common call -- one
	// that attempts nothing -- allocates none of them.
	tried := &sbaAttempts{alive: e.G.AliveCount()}
	// Safety net for a duration-ending change folded outside Engine.emit
	// (the Updated replacement paths call events.Emit directly).
	e.expireControl(controlOnEvent)
	// The same safety for a static GainControl$ transfer: an SBA-pass change
	// (e.g. a legend rule binning the Aura) can end or newly want a static
	// grant without Engine.emit's tail having run the reconcile.
	e.reconcileControlStatics()
	// The quiet skip (engine/rules/sbaquiet.go): the pass loop last ran to a fixed
	// point without emitting anything, and nothing an SBA reads has moved
	// since, so running it again would apply nothing.
	ep0 := len(e.L.Events)
	if e.format == FormatCommander {
		e.collectCommanderArrivals()
	}
	quiet := len(e.cmdZoneDue) == 0 && e.sbaQuietNow()
	if quiet && !sbaQuietVerify {
		e.checkGameOver()
		e.releasePendingDecisionOfDepartedPlayer()
		return
	}
	e.sbaQuiet = sbaQuietKey{}
	e.sbaUnquiet = false
	// The fused per-permanent prefilter (engine/rules/sba_prefilter.go): an action
	// whose flag is down would find nothing on the board.
	facts := &sbaBoardFacts{at: -1}
	for pass := 0; pass < maxSBAPasses; pass++ {
		tried.skips = 0
		if e.format == FormatCommander {
			e.collectCommanderArrivals()
			if len(e.cmdZoneDue) > 0 {
				e.commanderZoneSBA()
			}
		}
		var b sbaBatch
		e.checkLoseConditions(&b)
		if e.sbaFacts(facts).counterPair {
			e.annihilateOppositeCounters(&b)
		}
		tried.rearm(e.G.AliveCount())
		e.destroyLethalDamage(&b, tried)
		groups := e.legendGroups(e.sbaFacts(facts).legend)
		// CR 704.5k: the world rule poses no choice, unlike the legend rule.
		if e.sbaFacts(facts).world {
			e.worldRule(&b)
		}
		if e.sbaFacts(facts).pw {
			e.planeswalkerZeroLoyalty(&b, tried)
		}
		if e.sbaFacts(facts).battle {
			e.battleZeroDefense(&b, tried)
		}
		e.ceaseDeadTokens(&b, tried)
		if e.sbaFacts(facts).combat {
			e.removeNoncreatureCombatants(&b)
		}
		if e.sbaFacts(facts).attach {
			e.attachmentSBAs(&b)
		}
		if e.sbaFacts(facts).paired {
			e.unpairNoncreatures(&b)
		}
		// CR 704.5z / 303.7a: Role exclusivity per Role controller.
		if e.sbaFacts(facts).role {
			e.roleSBA(&b)
		}
		if e.sbaFacts(facts).saga {
			e.checkSagas(&b, tried)
		}
		// CR 704.5t: a dungeon whose marker sits on its bottommost room and
		// whose room ability has left the stack is completed.
		e.dungeonCompletion(&b, tried)
		// CR 702.145d/g: the neither-to-day and neither-to-night checks.
		if e.sbaFacts(facts).dayNight {
			e.dayNightStateCheck(&b)
		}
		// The losses are made first: a player who has left makes no CR 704.5j
		// choice (CR 800.4a), and rearm below must see the alive set the batch's
		// own attempts are made under. That player's permanents stay until the
		// batch has applied, so one that dies in this check still dies.
		for _, l := range b.losses {
			e.losePlayer(l.player, l.text)
		}
		tried.rearm(e.G.AliveCount())
		// CR 704.5j: a duplicate legendary set asks its controller which member
		// to keep. The whole batch parks with the ask (parkLegendChoice): nothing
		// applies while it is pending, and the answer applies all of it. If a pose
		// is impossible (a decision is already outstanding -- a re-entrant pass
		// after another SBA parked its own ask), the legends and the casualties
		// wait for the pass after the answer.
		if g := e.firstLegendGroup(groups); g != nil {
			if e.pending == nil && e.choosing == chooseNone && e.legendBatch == nil {
				e.sweepDepartedPlayers(tried)
				e.parkLegendChoice(*g, b)
				if e.legendBatch != nil {
					if quiet {
						e.verifySBAQuiet(ep0)
					}
					return
				}
				continue
			}
			// Deferred on a runtime input (an outstanding decision): the pass
			// loop's no-op here is not a function of the board alone.
			e.sbaUnquiet = true
			b.dead = nil
		}
		changed := !b.empty()
		for _, c := range b.dead {
			markTried(&tried.objs, c.id)
		}
		e.applySBABatch(&b)
		if e.sweepDepartedPlayers(tried) {
			changed = true
		}
		if !changed {
			stable = true
			break
		}
	}
	if quiet {
		e.verifySBAQuiet(ep0)
	}
	if !stable {
		e.emit(events.Event{Kind: events.Note,
			Text: "state-based actions did not reach a fixed point within the pass budget"})
	} else {
		e.sbaRecordQuiet(ep0, tried.skips == 0)
	}
	e.checkGameOver()
	e.releasePendingDecisionOfDepartedPlayer()
}

// unpairNoncreatures ends the Soulbond pairing of a permanent that has stopped
// being a creature (CR 702.95e). It stays ended if the permanent becomes a
// creature again. A phased-out permanent keeps its pairing.
func (e *Engine) unpairNoncreatures(b *sbaBatch) {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil && o.Paired != 0 && !o.PhasedOut && !e.IsCreature(id) {
				b.add(events.Event{Kind: events.Pair, Obj: id})
			}
		}
	}
}

// legendGroup is one CR 704.5j duplicate set: two or more legendary permanents
// with the same name under ONE controller, in battlefield scan order. The
// controller of the set chooses which member survives; the rest go to their
// owners' graveyards.
type legendGroup struct {
	player state.PlayerID
	name   string
	ids    []state.ObjID
}

// legendBatch is one parked CR 704.5j application: the duplicate set whose
// controller is choosing which member to keep, everything else the same check
// found (applied only when the answer lands, so the WHOLE batch observes one
// pre-batch board, CR 704.3), and that board -- the same immutable trigger
// look-back snapshot cmdZoneMove.before shares. Plain value data, so Clone
// deep-copies the slices (clone.go) the way it deep-copies cmdZone.
type legendBatch struct {
	group    legendGroup
	dead     []casualty
	acts     []events.Event
	counters []events.Event
	before   *triggerSnapshot
}

// firstLegendGroup is the first duplicate set whose controller is still in the
// game: a departed player's permanents leave with them (CR 800.4a).
func (e *Engine) firstLegendGroup(groups []legendGroup) *legendGroup {
	for i := range groups {
		if p := groups[i].player; int(p) >= len(e.G.Players) || !e.G.Players[p].Lost {
			return &groups[i]
		}
	}
	return nil
}

// legendGroups collects CR 704.5j's duplicate sets: if two or more legendary
// permanents with the same name are controlled by the same player, the set's
// controller chooses one and the rest are put into their owners' graveyards.
// "Legendary" is a CHARACTERISTIC the layer system can change --
// CopyPermanent's NonLegendary$ True strips the supertype at layer 4 -- so
// the check reads the DERIVED type list (typeCharacteristics), never just the
// printed face: a non-legendary copy of a legend must not be gathered against
// its original. Sets with a single member are not duplicates and are dropped.
// A permanent a live `S:Mode$ IgnoreLegendRule` static exempts (Council of
// Reeds' "The 'legend rule' doesn't apply to creatures you control") never
// joins a set at all: exempt members are skipped before grouping, so two
// exempt permanents form no duplicate set and a set that loses members to an
// exemption can fall below the two-member threshold and drop entirely.
// The scan is deterministic (AliveFrom(0) seat order, each battlefield zone a
// slice, seen keyed on the current derived name), so the event stream is
// reproducible run to run; membership maps are never iterated.
func (e *Engine) legendGroups(mayPair bool) []legendGroup {
	// The exemption statics are collected once, in activeStatics' canonical
	// deterministic order, and reused for every candidate; each candidate is
	// matched with the static's own source/controller context so
	// `Creature.YouCtrl` is scoped to the static's controller, not the
	// duplicate set's. mayPair is mayHaveLegendPair's answer, which the
	// caller's fused board scan (sbaBoardFacts.legend) already holds.
	if !mayPair {
		return nil
	}
	exempt := e.legendExemptions()
	anyLType := e.activeHasLType()
	var all []legendGroup
	for _, p := range e.G.AliveFrom(0) {
		seen := make(map[string]int)
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			// The cheap gate is the derived Legendary read (sbaTypeFast): a
			// layer-4 AddType$ Legendary (Leyline of Singularity) makes a
			// printed nonlegendary permanent a legend-rule candidate
			// (CR 205.4, 704.5j).
			if o == nil || o.Face() == nil || !e.sbaTypeFast(o, sbaLegendary, anyLType) {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so it is not a duplicate legend.
				continue
			}
			if !legendaryUnderLayers(e, id) {
				continue
			}
			if e.legendRuleExempt(exempt, id) {
				continue
			}
			name := e.Name(id)
			if gi, ok := seen[name]; ok {
				all[gi].ids = append(all[gi].ids, id)
				continue
			}
			seen[name] = len(all)
			all = append(all, legendGroup{player: p, name: name, ids: []state.ObjID{id}})
		}
	}
	var groups []legendGroup
	for _, g := range all {
		if len(g.ids) >= 2 {
			groups = append(groups, g)
		}
	}
	return groups
}

// legendExemptions collects every live IgnoreLegendRule exemption the CR
// 704.5j scan must honour, from BOTH delivery routes: the printed
// S:Mode$ statics (activeStatics' canonical battlefield walk) first, then
// the Effect-delivered registrations (state.ContinuousEffect entries whose
// Restriction is IgnoreLegendRule, in e.active()'s CR 613 order -- sorted,
// deterministic, never a map). Hall of Echoes' resolved {5} ability
// (`DB$ Effect | StaticAbilities$ LegendPermission`, LegendPermission:
// `Mode$ IgnoreLegendRule | ValidCard$ Permanent.YouCtrl`) is the corpus
// carrier of the second route. legendRuleExempt matches both unchanged: the
// registered view's YouCtrl resolves against the effect's controller
// through staticSpecCtx exactly like a printed static's does.
// sbaQuietKindsSafe reads the same helper, so the quiet skip and the full
// pass can never disagree about whether an exemption is live.
func (e *Engine) legendExemptions() []staticView {
	exempt := e.activeStatics("IgnoreLegendRule")
	for _, ce := range e.active() {
		if ce.Restriction == effects.ModeIgnoreLegendRule {
			exempt = append(exempt, staticView{Source: ce.Source, Controller: ce.Controller,
				Params: ce.RestrictParams, SVars: ce.RestrictSVars, Remembered: ce.Remembered})
		}
	}
	return exempt
}

// legendRuleExempt reports whether one of the given live IgnoreLegendRule
// statics exempts the permanent from CR 704.5j. Each static's own "as long
// as" gate runs first through the shared continuousGateHolds grammar -- the
// same IsPresent$/IsPresent2$/PresentCompare$/CheckSVar$/Condition$ evaluator
// every other static consumer uses -- so a conditional exemption is live only
// while its condition holds. Brothers Yamazaki and Syr Joshua and Syr Saxon
// both require EXACTLY two permanents named on the battlefield
// (`IsPresent$ ... | PresentCompare$ EQ2`), so a third copy must NOT inherit
// the exemption; without this gate the statics would exempt unconditionally.
// Then each static's `ValidCard$` is evaluated against the candidate with
// `staticSpecCtx`, so `YouCtrl` and the rest of the player-relative grammar
// resolve against THAT static's source and controller (Council of Reeds
// exempts the creatures its own controller controls, not every creature on
// the board). An absent `ValidCard$` is Forge's "all cards" spelling --
// Mirror Gallery's unconditional "The legend rule doesn't apply." -- so it
// exempts every candidate. statics is passed in already collected
// (legendGroups calls activeStatics once) to keep the walk to a single
// deterministic pass.
func (e *Engine) legendRuleExempt(statics []staticView, id state.ObjID) bool {
	if len(statics) == 0 {
		return false
	}
	for _, sv := range statics {
		if !e.continuousGateHolds(sv) {
			continue
		}
		spec := strings.TrimSpace(sv.ParamStr(cards.PKValidCard))
		if spec == "" || e.matchesSpec(spec, id, e.staticSpecCtx(sv)) {
			return true
		}
	}
	return false
}

// parkLegendChoice parks the whole SBA batch -- the duplicate set, every other
// action found in the same check, and the pre-batch look-back board -- and
// asks the set's controller which member to keep, in ONE step. The atomicity
// is the point: nothing is applied under an outstanding ask, so the board the
// controller chooses against is exactly the pre-batch board the casualties
// were scanned against (CR 704.3), and the parked state is always exactly
// "batch found, choice pending". The options are offered in battlefield scan
// order, so the deterministic bot's clamp fallback (option 0) keeps the
// battlefield-order first -- the survivor the pre-decision-channel build
// always picked. Only called when e.pending is nil and no flow owns e.choosing
// (checkStateBased guards this); a pose is never stranded without its ask.
func (e *Engine) parkLegendChoice(g legendGroup, b sbaBatch) {
	e.legendBatch = &legendBatch{
		group:    g,
		dead:     b.dead,
		acts:     b.acts,
		counters: b.counters,
		before:   e.snapshotTriggerBoard(),
	}
	e.askLegendChoice()
}

// askLegendChoice poses the CR 704.5j choice for the parked batch's duplicate
// set to its controller, in battlefield scan order (so the deterministic bot's
// clamp fallback keeps the battlefield-order first -- the survivor the
// pre-decision build always picked). It is the ONE construction site for the
// ask, shared by parkLegendChoice (the fresh pose) and checkStateBased's
// recovery re-pose (a parked batch whose decision was displaced): the two
// must offer an identical decision, or a recovered ask would accept an answer
// the original never offered.
func (e *Engine) askLegendChoice() {
	g := e.legendBatch.group
	// CR 104.2a: a pass whose losses leave one player (or none) ends the
	// game, so the legend choice is not posed in it either -- the duplicate
	// can appear in that very pass when the loser's departure returns a
	// card (Banishing Light leaving with its controller). The batch settles
	// as the departed-controller decline does, keeping the pass's other
	// simultaneous SBAs (CR 704.3), and checkGameOver ends the game at the
	// pass's end.
	if (int(g.player) < len(e.G.Players) && e.G.Players[g.player].Lost) || e.gameEndsNow() {
		// CR 800.4a: a player who has left the game makes no choices. The
		// departed controller's CR 704.5j choice is therefore unexercised and
		// declines deterministically to the battlefield-order first member --
		// the same outcome parkCommanderZoneMove gives a departed commander
		// owner. Applying rather than asking also means a parked batch can
		// never re-pose to a seat that can no longer answer it.
		e.applyLegendBatch(g.ids[0])
		return
	}
	opts := make([]decision.Option, len(g.ids))
	for i, id := range g.ids {
		opts[i] = decision.Option{Index: i, Kind: "keep", Label: g.name, Obj: id, Player: g.player}
	}
	d := &decision.Decision{Player: g.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt:  "Choose which " + g.name + " to keep; the rest are put into their owners' graveyards",
		Options: opts}
	e.choosing = chooseLegend
	e.ask(d)
}

// legendAnswer applies an answered CR 704.5j choice: the kept member is
// recorded through a Choose "legend_keep" event (a log marker, like riot's --
// the outcome itself is the MoveZone events below, so a log-only replay
// reproduces both branches), the parked lethal casualties are applied first
// with the parked pre-batch board (a KEPT member keeps its lethal-damage
// destruction path, so a regeneration shield can still save it -- exactly the
// treatment the pre-decision build gave its scan-order survivor), and the
// non-kept members go to their owners' graveyards as legend-rule departures
// (placement, not destruction -- no regeneration, no destruction replacement).
// The Submit tail's next checkStateBased pass re-runs every SBA on the settled
// board, so a further duplicate set is parked and asked there and any SBA the
// moves themselves caused is picked up. An answer with no batch parked (only
// reachable from a hand-built decision -- every real ask parks one) degrades
// to a Note, the same totality stance as handleCmdZone.
func (e *Engine) legendAnswer(d *decision.Decision, in decision.Intent) {
	b := e.legendBatch
	if b == nil {
		e.legendBatch = nil
		e.choosing = chooseNone
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "legend-rule decision answered with no batch parked"})
		return
	}
	kept := b.group.ids[0]
	if chosen := d.Chosen(in); len(chosen) == 1 {
		kept = chosen[0].Obj
	}
	e.applyLegendBatch(kept)
}

// applyLegendBatch settles the parked CR 704.5j batch with the given member
// kept: the kept member is recorded through a Choose "legend_keep" event (a
// log marker, like riot's -- the outcome itself is the MoveZone events below,
// so a log-only replay reproduces both branches), and the batch applies on the
// parked pre-batch board. A KEPT member keeps its lethal-damage destruction path,
// so a regeneration shield can still save it; the non-kept members go to their
// owners' graveyards as legend-rule departures (placement, not destruction -- no
// regeneration, no destruction replacement), and a member that is both is
// serialized by the legend rule. It is the ONE application site, shared by
// legendAnswer (an answered choice) and askLegendChoice's departed-controller
// decline (CR 800.4a), so the two paths can never settle a batch differently.
// The Submit tail's next checkStateBased pass re-runs every SBA on the settled
// board, so a further duplicate set is parked and asked there and any SBA the
// moves themselves caused is picked up.
func (e *Engine) applyLegendBatch(kept state.ObjID) {
	lb := e.legendBatch
	e.legendBatch = nil
	e.choosing = chooseNone
	if lb == nil {
		return
	}
	e.emit(events.Event{Kind: events.Choose, Obj: kept,
		Counter: "legend_keep", Player: lb.group.player})
	b := sbaBatch{counters: lb.counters}
	for _, c := range lb.dead {
		if c.id == kept || !slices.Contains(lb.group.ids, c.id) {
			b.dead = append(b.dead, c)
		}
	}
	for _, id := range lb.group.ids {
		if id != kept {
			b.add(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZBattlefield, To: state.ZGraveyard, Text: "legend rule"})
		}
	}
	b.acts = append(b.acts, lb.acts...)
	before := e.triggerBefore
	e.triggerBefore = lb.before
	e.applySBABatch(&b)
	e.triggerBefore = before
}

// legendaryUnderLayers reports whether the object's DERIVED type list still
// carries the Legendary supertype. A printed legend whose layer-4 effects
// strip it (CopyPermanent's NonLegendary$) is not legendary for CR 704.5j.
func legendaryUnderLayers(e *Engine, id state.ObjID) bool {
	for _, t := range e.typeCharacteristics(id, 0) {
		if strings.EqualFold(t, "Legendary") {
			return true
		}
	}
	return false
}

// worldUnderLayers reports whether the object's DERIVED type list still
// carries the World supertype. "World" is a characteristic the layer system
// can change (CR 613.1c AddTypes$/RemoveCardTypes$ at layer 4), so the world
// rule reads the derived list exactly as the legend rule reads Legendary -- a
// printed World permanent whose layers stripped the supertype is not a World
// permanent for CR 704.5k, and a printed non-World permanent a layer-4 effect
// granted it to is.
func worldUnderLayers(e *Engine, id state.ObjID) bool {
	for _, t := range e.typeCharacteristics(id, 0) {
		if strings.EqualFold(t, "World") {
			return true
		}
	}
	return false
}

// worldPermanents returns every permanent on the battlefield that carries the
// World supertype, GLOBALLY across all controllers, in a deterministic order:
// seat order (AliveFrom(0)), then each seat's battlefield zone in scan order.
// CR 704.5k's scope is global -- the rule counts every world permanent, not
// just one controller's -- and the departure order must not depend on a map
// iteration, so the result is one flat slice in that fixed order.
func (e *Engine) worldPermanents() []state.ObjID {
	var out []state.ObjID
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			if !worldUnderLayers(e, id) {
				continue
			}
			out = append(out, id)
		}
	}
	return out
}

// worldRule applies CR 704.5k: "If two or more permanents have the supertype
// world, all except the one that has had the world supertype for the shortest
// amount of time are put into their owners' graveyards. In the event of a tie
// for the shortest amount of time, all are put into their owners'
// graveyards."
//
// The rule poses NO choice -- unlike the CR 704.5j legend rule it is fully
// deterministic -- so it is an automatic SBA, not a parked ask. It is called
// directly from checkStateBased's pass loop (beside planeswalkerZeroLoyalty),
// not through a tried-set batch helper: it reads the whole battlefield every
// pass and converges because each call leaves at most one World permanent, so
// the next pass sees <2 and queues nothing. With fewer than two world permanents
// it queues nothing. Otherwise it
// finds the YOUNGEST permanent(s) by entry order: Object.Timestamp is assigned from
// Game.Clock on every battlefield entry and increases monotonically, so the
// greatest Timestamp is the most recently entered permanent, the one that has
// had the world supertype for the shortest amount of time. If exactly one
// permanent holds the maximum it survives and every other world permanent is
// put into its owner's graveyard; a tie for the maximum (two permanents that
// entered at the same instant, so neither has held it longer) puts ALL of them
// into their owners' graveyards.
//
// APPROXIMATION (recorded in the commit message and the ticket report, not in
// AGENTS.md): the engine tracks when a permanent ENTERED, not when it gained
// the world supertype. For the printed World permanents the two coincide (they
// have the supertype as they enter); a layer-4 effect that GRANTS World to an
// older permanent makes "shortest amount of time" really mean "most recently
// granted", which would need a supertype-gain timestamp the engine does not
// record. Entry order is therefore the honest stand-in, and the tie case still
// fires exactly as the CR requires.
//
// The departures are placements, not destruction: an ordinary MoveZone
// battlefield->graveyard with Text "world rule", so no regeneration or
// destruction-replacement applies (the same treatment the legend rule gives
// its non-kept members).
func (e *Engine) worldRule(b *sbaBatch) {
	if !e.mayHaveWorldPair() {
		return
	}
	worlds := e.worldPermanents()
	if len(worlds) < 2 {
		return
	}
	newest := worlds[0]
	newestCount := 0
	for _, id := range worlds {
		o := e.G.Obj(id)
		n := e.G.Obj(newest)
		switch {
		case o.Timestamp > n.Timestamp:
			newest, newestCount = id, 1
		case o.Timestamp == n.Timestamp:
			newestCount++
		}
	}
	// A unique youngest survives; a tie for the youngest spares none. Every
	// permanent that is not the unique survivor departs, in the deterministic
	// scan order worldPermanents established.
	for _, id := range worlds {
		if newestCount == 1 && id == newest {
			continue
		}
		b.add(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "world rule"})
	}
}

// chooseLegend is the CR 704.5j legend-rule controller choice (engine/rules/sba.go),
// the one decision an SBA poses outside a resolution -- the commander-zone
// and Siege asks share the property through their replacement parks. 44 is
// the next free value after chooseTokenReplace (43); the numbers matter only
// inside the package's switch tables.
const chooseLegend chooseFor = 44

// annihilateOppositeCounters queues CR 704.5q for permanents in fixed seat
// and battlefield order. Both removals are events so replay reconstructs the
// same counter state as the live game.
func (e *Engine) annihilateOppositeCounters(b *sbaBatch) {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so no state-based action sees it.
				continue
			}
			n := min(o.Counter("P1P1"), o.Counter("M1M1"))
			if n <= 0 {
				continue
			}
			b.counters = append(b.counters,
				events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: -n},
				events.Event{Kind: events.CounterChange, Obj: id, Counter: "M1M1", Amount: -n})
		}
	}
}

// checkLoseConditions queues CR 704.5a (life 0 or less) for every player not
// already marked Lost, in fixed seat order (Players is a slice walked by index,
// never a map, so simultaneous eliminations always emit in the same order run to
// run), then CR 704.5b (an attempted draw from an empty library since the last
// check), 704.5c (poison) and CR 903.10 (commander damage). A "can't lose"
// effect is read here, on the board the check is made on.
func (e *Engine) checkLoseConditions(b *sbaBatch) {
	loses := func(p *state.Player, reason, text string) bool {
		if p.Lost || b.loses(p.ID) || e.gameLossPrevented(p.ID, reason) {
			return false
		}
		b.losses = append(b.losses, sbaLoss{p.ID, text})
		return true
	}
	for i := range e.G.Players {
		if p := &e.G.Players[i]; p.Life <= 0 {
			loses(p, loseReasonLifeReachedZero, "life total is 0 or less")
		}
	}
	// CR 704.5b: a player who attempted to draw from an empty library since
	// the last check loses. The mark (state.Player.DrewFromEmpty, set by
	// markDrawEmpty / effects' drawFor) is consumed either way: a loss a
	// GameLoss replacement stops is not retried by a later check unless the
	// player attempts another such draw.
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if p.Lost || !p.DrewFromEmpty || b.loses(p.ID) {
			continue
		}
		if !loses(p, loseReasonMilled, "drew from an empty library") {
			b.add(events.Event{Kind: events.DrawEmpty, Player: p.ID, Amount: 1})
		}
	}
	// CR 704.5c: a player with ten or more poison counters loses. Ward's
	// AddCounterYou<N/POISON> cost emits PlayerCounterChange, so this belongs
	// in the shared SBA pass rather than in Ward's payment implementation.
	for i := range e.G.Players {
		if p := &e.G.Players[i]; p.Counter("POISON") >= 10 {
			loses(p, loseReasonPoisoned, "ten or more poison counters")
		}
	}
	// CR 903.10: 21 or more COMBAT damage from the same commander. Commander
	// damage is recorded per commander, so the slice is walked as two indices in
	// the same deterministic order. Only a Commander game has any.
	if e.format == FormatCommander {
		for i := range e.G.Players {
			p := &e.G.Players[i]
			for _, dmg := range p.CmdDamage {
				if dmg >= 21 {
					loses(p, loseReasonCommanderDamage, "commander damage (21 or more from one commander)")
					break
				}
			}
		}
	}
}

// sweepDepartedPlayers applies CR 800.4a to every Lost player -- from the losses
// this check just made, from an effect's "you lose the game", or from any future
// cause: their owned cards and controlled battlefield/stack objects cease to
// exist. Doing this generically, keyed only on p.Lost rather than on which branch
// set it, gives every kind of departure the same cleanup path.
//
// tried.players is this loop's memory, mirroring destroyLethalDamage's tried.objs:
// a player already swept this call is skipped entirely on later passes (one sweep
// attempt per player per checkStateBased call), and it reports whether a sweep
// emitted anything. A sweep that emitted no event at all (every departed object
// already ceased -- the steady state of every call after the first once a seat
// has left) changed nothing, so it is not new work for the fixed-point test. Any
// emission at all, including a blocking replacement's own substitute effect, is.
func (e *Engine) sweepDepartedPlayers(tried *sbaAttempts) bool {
	changed := false
	for i := range e.G.Players {
		p := &e.G.Players[i]
		if !p.Lost {
			continue
		}
		if tried.players[p.ID] {
			tried.skips++
			continue
		}
		markTried(&tried.players, p.ID)
		n0 := len(e.L.Events)
		e.ceaseDepartedObjects(p.ID)
		if len(e.L.Events) != n0 {
			changed = true
		}
	}
	return changed
}

// ceaseDepartedObjects applies CR 800.4a and 800.4e in stable arena order. A
// departed player's card-backed owned objects leave regardless of their current
// zone; stack objects that player controls cease whether card-backed or
// cardless. On the battlefield ownership decides the first step: an
// opponent-owned card the departed player controls is not ceased but, as
// CR 800.4a's last step, exiled. ZCeased retains the replay-stable arena
// tombstone without adding it to a zone.
//
// Moving an owned attacker clears its combat state, so that case continues
// before the 800.4e check rather than emitting a redundant EndCombatReset. The
// owner check also prevents that duplicate when multiple departed players are
// swept in seat order and an attacker owned by a later seat attacks an earlier
// one.
func (e *Engine) ceaseDepartedObjects(p state.PlayerID) {
	// CR 800.4a: the departed player's objects leave simultaneously, so every
	// battlefield departure in this sweep observes ONE pre-sweep board -- the
	// batch discipline destroyLethalDamage and the world rule follow. Without
	// it emit snapshots the whole game per departing permanent, which is
	// quadratic in a large board (60,001 tokens in
	// TestLargeEliminationSweepDoesNotTripLivelockWatcher took minutes).
	if e.triggerBefore == nil && e.ceaseSweepLeavesBattlefield(p) {
		own := e.snapshotTriggerBoard()
		e.triggerBefore = own
		defer e.closeTriggerWindow(own, nil)
	}
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone == state.ZCeased {
			continue
		}
		ownedCard := o.Owner == p && o.Card != nil
		controlledStackObject := o.Controller == p && o.Zone == state.ZStack
		if ownedCard || controlledStackObject {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
				From: o.Zone, To: state.ZCeased, Text: "player left the game"})
			continue
		}
		if !o.IsAttacking || int(o.Attacking) >= len(e.G.Players) || !e.G.Players[o.Attacking].Lost {
			continue
		}
		if o.Card != nil && int(o.Owner) < len(e.G.Players) && e.G.Players[o.Owner].Lost {
			continue
		}
		e.emit(events.Event{Kind: events.EndCombatReset, Obj: o.ID})
	}
	// CR 800.4a's last step: "if there are any objects still controlled by
	// that player, those objects are exiled." Every owned object and every
	// stack object p controls has left above, and every effect giving p
	// control ended when p left (grantEnded; expireControl runs on the
	// PlayerLost emit), so what remains is a permanent p controls with no
	// effect involved (Reanimate on another player's card). It is moved, not
	// ceased: its owner is still in the game.
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone == state.ZBattlefield && o.Controller == p {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID,
				From: state.ZBattlefield, To: state.ZExile, Text: "player left the game"})
		}
	}
}

// ceaseSweepLeavesBattlefield reports whether ceaseDepartedObjects(p) will
// move at least one permanent off the battlefield (the only departures whose
// triggers look back at the pre-departure board).
func (e *Engine) ceaseSweepLeavesBattlefield(p state.PlayerID) bool {
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone == state.ZBattlefield && ((o.Owner == p && o.Card != nil) || o.Controller == p) {
			return true
		}
	}
	return false
}

// casualty is one creature destroyLethalDamage has found lethal, together
// with which CR 704.5 clause is why -- Ruling T22-i (fix round 1): 704.5f
// (toughness <= 0) and 704.5g (lethal damage/deathtouch) are rules-distinct
// (only one of them is actually "destruction", so only one of them is
// something Indestructible or a replacement effect keyed on Destroy could
// ever apply to), and the log should be able to tell them apart even though
// Text carries no rules weight of its own.
type casualty struct {
	id   state.ObjID
	text string
}

// destroyLethalDamage performs the two CR 704.5 state-based actions combat
// damage depends on: a creature with toughness 0 or less (704.5f) is
// put into its owner's graveyard (not destroyed), and a creature with
// damage marked on it greater than or equal to its toughness, or with any
// damage at all from a source with Deathtouch (704.5g, via the Deathtouched
// counter damageStep marks), is destroyed unless Indestructible.
//
// This is general state-based-action logic, not combat-specific -- a
// creature killed by an ordinary DealDamage spell dies here exactly the same
// way one killed in combat does -- which is why it lives here rather than in
// combat.go, where Task 21 first added it.
//
// The Indestructible check reads the same way effDestroy/effDestroyAll
// (engine/effects/zone.go) already do; it is not one of the eight keywords Task 21
// registers as implemented, so a card that actually carries it still reads
// as unsupported for deck-routing purposes, but a creature that is
// Indestructible for some other already-working reason should not
// spuriously die to lethal damage.
//
// It only queues: applySBABatch moves the casualties once every family has read
// the same board (CR 704.3).
//
// tried.objs is shared across every pass of one checkStateBased call: an object
// already in it is skipped entirely, so the object will not be found lethal-and-new
// again until the next call, whether or not it actually left, unless a player has
// been eliminated in the meantime (sbaAttempts.rearm).
func (e *Engine) destroyLethalDamage(b *sbaBatch, tried *sbaAttempts) {
	anyLType := e.activeHasLType()
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if tried.objs[id] {
				tried.skips++
				continue
			}
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so lethal damage does not destroy it.
				continue
			}
			f := o.Face()
			// CR 702.114e: a bestowed-attached card is an Aura, not a creature,
			// so the creature SBAs (lethal damage/toughness) do not hit it.
			// CR 702.150c: the same for an attached Reconfigure card (not a
			// creature while attached -- marked damage does not destroy it).
			if f == nil || o.BestowedAttached() || o.ReconfiguredAttached() {
				continue
			}
			// Creature-ness for both sweeps below is the LAYER-DERIVED type
			// list, not the printed face: CR 704.5f/g read the permanent's
			// current types after all continuous effects. A non-creature
			// permanent an animation effect made a creature (an earthbent land,
			// a crewed Vehicle, an animated artifact) must be swept, or it is
			// immortal to both zero toughness and lethal damage. A creature
			// that LOST Creature to a layer effect is correctly not swept.
			// This is the same predicate combat legality uses
			// (engine/rules/combat.go), and for a face-down permanent
			// typeCharacteristics already bases the derived set on
			// FaceDownTypeWords (CR 708.5/Yedora), so the previous face-down
			// special case gave the same answer through it.
			if !e.sbaIsCreature(o, f, anyLType) {
				continue
			}
			if e.Toughness(id) <= 0 {
				b.dead = append(b.dead, casualty{id, "toughness <= 0"})
				continue
			}
			// CR 704.5g reads "damage from a source with deathtouch", not
			// "marked damage": the Deathtouched mark can stand alone when the
			// damage was dealt in COUNTER form (infect, CR 702.90b -- a 1/1
			// deathtouch infect creature deals its 1 as a -1/-1 counter and
			// nothing is marked), so the mark alone is lethal. The mark is
			// emitted only alongside damage that actually landed (both emit
			// sites guard on the applied amount) and is cleared in the same
			// cleanup block that clears marked damage, so a mark with no
			// damage and no counter-form hit behind it is unreachable.
			dtMark := o.Counter("Deathtouched")
			if o.Damage <= 0 && dtMark == 0 {
				continue
			}
			if o.Damage < e.Toughness(id) && dtMark == 0 {
				continue
			}
			if e.hasKeywordH(id, kwhIndestructible) {
				continue
			}
			b.dead = append(b.dead, casualty{id, "lethal damage"})
		}
	}
}

// planeswalkerZeroLoyalty performs CR 704.5i: a planeswalker with loyalty 0
// or less is put into its owner's graveyard (not destroyed -- the modern CR
// has no planeswalker-uniqueness SBA; the legend rule already handles
// legendary walkers through legendCasualties). The walker's face carries its
// starting loyalty, the Move-zone-change grant (engine/events/apply.go, CR 306.5b)
// put it on, and spell/ability damage removes counters from it (CR 306.8,
// engine/effects/damage.go); between the three, loyalty can only reach zero through
// damage or a face whose printed starting loyalty is literally 0 (6 corpus
// files), and either way this sweep is what removes the walker.
//
// Discipline follows destroyLethalDamage's: a tried-set (shared objs), one move
// per object per call, and the deterministic AliveFrom(0) seat /
// battlefield-slice order so the event stream is reproducible.
// TestPlaneswalkerSBABatchUsesPreDepartureBoard pins that simultaneous
// zero-loyalty departures observe each other (CR 603.10a). The move is not
// destruction (no ReplaceDestruction, no regeneration -- the same treatment
// "toughness <= 0" gets).
//
// A face whose printed starting loyalty this engine cannot READ (absent, or
// Loyalty:X -- Nissa, Steward of Elements, 2 corpus files) never reaches the
// sweep: CR 704.5i needs a known loyalty of 0 or less, and an unreadable one
// is not known-zero. Such a walker keeps its place on the battlefield with no
// loyalty counters -- the stand-in is recorded in AGENTS.md.
func (e *Engine) planeswalkerZeroLoyalty(b *sbaBatch, tried *sbaAttempts) {
	// stat:IgnorePlaneswalkerZeroLoyaltyRule (Sanctum Lurker): collected
	// lazily, once per sweep, only when a zero-loyalty walker is found.
	var exempt []staticView
	exemptRead := false
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if tried.objs[id] {
				tried.skips++
				continue
			}
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so zero loyalty does not bin it.
				continue
			}
			if o.FaceDown {
				// CR 708.5: a face-down battlefield permanent is a vanilla 2/2
				// creature and is never a planeswalker, so the zero-loyalty SBA
				// (CR 704.5i) must not sweep a manifested or cloaked card --
				// the same guard battleZeroDefense carries below.
				continue
			}
			f := o.Face()
			if f == nil || !f.IsPlaneswalker() {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(f.Loyalty))
			if err != nil || n < 0 {
				continue
			}
			if o.Counter("LOYALTY") > 0 {
				continue
			}
			if !e.IsPlaneswalker(id) {
				// CR 306.9/704.5i: only a planeswalker is put into the
				// graveyard; a printed walker that is not one now (Imprisoned
				// in the Moon's land, Luxior's creature) stays.
				continue
			}
			if !exemptRead {
				exempt, exemptRead = e.activeStatics("IgnorePlaneswalkerZeroLoyaltyRule"), true
			}
			if e.zeroLoyaltyExempt(exempt, id) {
				continue
			}
			markTried(&tried.objs, id)
			b.add(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZBattlefield, To: state.ZGraveyard, Text: "zero loyalty"})
		}
	}
}

// battleZeroDefense is the defeat state-based action for battles (CR
// 704.5h/310.11): a battle with no defense counters is DEFEATED -- exiled,
// not put into the graveyard -- and its owner's "may cast it transformed
// without paying its mana cost" offer (CR 310.11) is queued by Engine.emit's
// exile feed for startDefeatedCast to pose at the next step(). Modeled
// line-for-line on planeswalkerZeroLoyalty above (CR 704.5i): same tried-set,
// same AliveFrom(0) battlefield-slice determinism. The move is not destruction: no
// ReplaceDestruction, no regeneration shield consulted -- exactly the
// zero-loyalty/zero-toughness treatment.
func (e *Engine) battleZeroDefense(b *sbaBatch, tried *sbaAttempts) {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if tried.objs[id] {
				tried.skips++
				continue
			}
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			if o.PhasedOut {
				// CR 702.25b/d: a phased-out permanent is treated as though it
				// does not exist, so zero defense does not defeat it.
				continue
			}
			// A face-down card is a vanilla 2/2 creature (CR 708.5), never a
			// Battle: Face() returns the printed front face regardless of
			// FaceDown (and the entry grant above grants a face-down entry
			// no defense counters), so without this guard a manifested or
			// cloaked Battle would be swept out of the battlefield by the
			// defeat SBA the
			// instant it entered.
			if o.FaceDown {
				continue
			}
			f := o.Face()
			if f == nil || !f.IsBattle() {
				continue
			}
			if o.Counter("DEFENSE") > 0 {
				continue
			}
			markTried(&tried.objs, id)
			b.add(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZBattlefield, To: state.ZExile, Text: "defeated"})
		}
	}
}

// ceaseDeadTokens is CR 704.5d: a token ceases to exist the instant it
// leaves the battlefield. The state-based action is a plain zone check --
// any token whose CURRENT zone is neither the battlefield nor the stack nor
// already ceased is moved to ZCeased, the replay-stable arena tombstone with
// no game-zone membership, with Text "ceased to exist". The stack is excluded
// for the same reason Ephemeral's own IsCopy half exists: a token copy of a
// spell or ability legitimately sits there without being a permanent yet
// (Task 13 does not implement CopySpellAbility, so no card can produce this
// today, but the exclusion costs nothing and matches the brief exactly).
//
// tried.tokens bounds a replacement PERMANENTLY blocking the move (some "cards
// can't leave your graveyard" effect intercepting the move FROM graveyard): an object
// already attempted this call is skipped on later passes.
//
// Walks e.G.Objs by index -- the dense arena, never a map -- so multiple
// tokens or spell copies leaving their applicable zones cease in a fixed,
// reproducible order. A spell copy in any zone other than the stack ceases
// under CR 704.5e; permanent spell copies are converted to tokens on entry.
func (e *Engine) ceaseDeadTokens(b *sbaBatch, tried *sbaAttempts) {
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		// The field test first: the attempt memory is only consulted for a
		// token that would otherwise be ceased (the same set as before).
		// CR 722.3c's prepared exile copy is excluded: it was minted directly
		// in exile and is castable there, so it has not left the stack and
		// does not cease.
		if (o.IsToken || o.IsCopy) && !o.PreparedExileCopy() && o.Zone != state.ZBattlefield && o.Zone != state.ZStack && o.Zone != state.ZCeased {
			if tried.tokens[o.ID] {
				tried.skips++
				continue
			}
			markTried(&tried.tokens, o.ID)
			b.add(events.Event{Kind: events.MoveZone, Obj: o.ID,
				From: o.Zone, To: state.ZCeased, Text: "ceased to exist"})
		}
	}
}

// gameEndsNow reports whether checkGameOver would end the game on the
// current board: at most one player left (CR 104.2a's last-player win, which
// overrides every "can't win" effect, or CR 104.4a's draw). It changes nothing.
func (e *Engine) gameEndsNow() bool {
	if e.deferGameOver || e.G.Over {
		return false
	}
	return len(e.G.AliveFrom(0)) <= 1
}

// checkGameOver ends the game once at most one seat remains (CR 104.2a's
// "the last player left in the game wins", CR 104.4a's draw when nobody is).
// A concession is a way to be Lost (M2d-3 emits the same PlayerLost the
// life-loss path does), so a conceded game ends here exactly like one whose
// losers lost to life or the library.
// Ruling T22-b: this used to default the winner to seat 0 whenever nobody
// survived -- PlayerID's zero value is a real seat, so an unconditional
// `w := state.PlayerID(0)` silently crowned it every time elimination was
// simultaneous. There is no winner in that case; Amount: 1 tells Apply's
// GameOver case to record a draw (Game.Draw) instead of a winner.
func (e *Engine) checkGameOver() {
	// New deals opening hands as one genesis operation. Losses and every
	// other SBA are already real at that point, but the terminal GameOver is
	// deferred until New has emitted the public toss Note; GameOver must stay
	// last for the host's persisted burst-boundary contract.
	if e.deferGameOver || e.G.Over {
		return
	}
	alive := e.G.AliveFrom(0)
	if len(alive) > 1 {
		return
	}
	if len(alive) == 1 {
		// CR 104.2a: the last player in the game wins immediately, and the
		// win "overrides all effects that would preclude that player from
		// winning the game". It is therefore NOT routed through playerWins:
		// no GameWin CantHappen replacement (Abyssal Persecutor) can stop it.
		w := alive[0]
		e.emit(events.Event{Kind: events.GameOver, Player: w, Text: e.G.Players[w].Name})
	} else {
		e.emit(events.Event{Kind: events.GameOver, Amount: 1, Text: "draw"})
	}
	e.pending = nil
}

// dropDepartedFlow ends the flow that was waiting on d, the released decision
// of a player who has left the game. That player makes no further choices
// (CR 800.4a) and every stack object they controlled has ceased
// (ceaseDepartedObjects), so a cast they were proposing cannot continue, and
// a KChoose's routing marker -- with the mana-ability continuation it guarded
// -- belongs to nobody now. Left in place, both survive into the next seat's
// priority: the proposal as a cast "in flight", the marker read by the next
// KChoose answer's dispatch and by the legend-rule SBA gate.
func (e *Engine) dropDepartedFlow(d *decision.Decision) {
	if d.Kind == decision.KCommanderZone && len(e.cmdZone) > 0 {
		// The released ask's own parked move goes with it (CR 800.4a), and so
		// does every entry behind it whose owner has left the game: their
		// cards leave too, so there is nothing left to move. A living owner's
		// entry stays -- and since the released ask was the one that would
		// have handed the queue on (handleCmdZone's tail runs only on an
		// answer), the new front is posed here. Left parked it would block
		// every later commander's ask forever: commanderZoneSBA refuses to
		// pose while the queue is non-empty.
		e.cmdZone = e.cmdZone[1:]
		kept := e.cmdZone[:0]
		for _, pm := range e.cmdZone {
			o := e.G.Obj(pm.obj)
			if o != nil && int(o.Owner) < len(e.G.Players) && !e.G.Players[o.Owner].Lost {
				kept = append(kept, pm)
			}
		}
		e.cmdZone = kept
		if len(e.cmdZone) > 0 && e.pending == nil {
			if o := e.G.Obj(e.cmdZone[0].obj); o != nil && int(o.Owner) < len(e.G.Players) {
				e.askCommandZone(o.Owner)
			}
		}
	}
	if e.cast != nil && (e.cast.player == d.Player || e.cast.player == d.Acting()) {
		e.cast = nil
		e.deferredPush, e.deferredPushLKI = nil, nil
		if e.choosing == chooseCast {
			e.choosing = chooseNone // the dropped cast's own marker
		}
	}
	if d.Kind != decision.KChoose {
		return
	}
	switch e.choosing {
	case chooseMana:
		e.manaActivation = nil
	case chooseManaColor:
		e.manaColorActivation = nil
	case chooseManaDiscard, chooseManaExile, chooseManaSacrifice, chooseManaEvidence:
		e.ManaCost = nil
	case chooseManaUnless:
		e.manaUnlessActivation = nil
	}
	e.choosing = chooseNone
}

// releasePendingDecisionOfDepartedPlayer keeps a decision asked of a player
// who has since left the game from stranding the match. Only that player may
// answer it, Advance does nothing at all while e.pending is set, and with
// three or more seats the elimination does not end the game -- so without
// this the one goroutine running the match waits forever on an answer that
// can never come. With two seats checkGameOver above has already cleared
// e.pending, so this is the three-or-more case.
//
// Fix round 1, review finding F2: this used to fire only for Task 27's two
// trigger kinds, on the stated grounds that they were the only decisions
// asked from inside handle -- Submit runs handle, then this, then Advance, so
// a decision asked from inside handle is the only one state-based actions run
// underneath. That premise was FALSE, and the review traced the
// counterexample: handlePriority (legal.go) -> castSpell (stack.go) ->
// askTarget asks a KTarget decision from inside handle too. It is now
// deliberately keyed on nothing but "the player cannot answer", because
// enumerating which decisions can reach this state is precisely the reasoning
// that was wrong the first time.
//
// The continuation is resumeTriggerDrain for every kind, and it is the right
// one for all of them: state-based actions to a fixed point, then any queued
// triggers onto the stack, then priority to the next player who can actually
// take it. Re-entering priorityRound instead of resumeTriggerDrain would put
// a second checkStateBased call on priorityRound's own path, where step() has
// already reached the same fixed point -- giving a replacement-blocked
// state-based action a second attempt per step and moving sba.go's own
// measured firing counts (Ruling T22-p; see resumeTriggerDrain's header in
// turn.go). (Ruling T28-b: this used to say re-entering priorityRound "would
// re-run the draw step's draw" -- true before Task 28 moved the draw out of
// priorityRound entirely; the reason above is what actually survives that
// move.)
//
// This cannot loop. dropDepartedTriggers has already discarded the departed
// player's own triggers (CR 800.4a); an optional trigger whose DECIDER
// departed but whose controller lives is declined rather than re-asked; and
// e.pending is cleared before either continuation runs. If resuming a stack
// resolution reaches another ask, that ask sets e.pending before
// resumeTriggerDrain, whose pending guard makes the drain inert rather than
// overwriting the new decision or recursing through checkStateBased.
func (e *Engine) releasePendingDecisionOfDepartedPlayer() {
	d := e.pending
	if d == nil || e.G.Over {
		return
	}
	// A CR 722-redirected decision is stranded when EITHER seat departed:
	// the answering controller, or the controlled seat it acts for.
	departed := func(p state.PlayerID) bool { return int(p) < len(e.G.Players) && e.G.Players[p].Lost }
	if int(d.Player) >= len(e.G.Players) || !(departed(d.Player) || departed(d.Acting())) {
		return
	}
	e.pending = nil
	e.dropDepartedFlow(d)
	e.resumeTriggerDrain()
}
