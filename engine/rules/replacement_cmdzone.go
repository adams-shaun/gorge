package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	// kw:etbCounter and kw:ETBReplacement are implemented wholly by the
	// machinery above: both are R:Event$ Moved replacements (expanded from a
	// K: line by engine/cards/keywords.go) matched and applied here. Reading a card's
	// own tags is what a replacement registration means -- nothing elsewhere
	// in the tree registers them.
	//
	// kw:Devour and kw:Ravenous (CR 702.148) are the same idea for a K: line
	// that expands to an ETB trigger instead of a replacement (kw:Devour's
	// optional sacrifice + counter put, kw:Ravenous's X +1/+1-counter put
	// plus the X>=5 conditional draw, both in engine/cards/kw_*.go). The marker
	// exists only so the coverage ratchet sees the head as supported; the
	// machinery it needs (trig:ChangesZone, api:PutCounter, api:Draw, the
	// SVar-condition gate) is all registered under its own primitives.
	//
	// The four turn/mana replacement events register the same way: repl:Untap
	// (the Basalt Monolith class), repl:BeginPhase (the Necropotence class),
	// repl:Transform (the Sephiroth class) and repl:ProduceMana (the Virtue
	// of Strength class, whose ReplaceWith$ body DB$ ReplaceMana is a
	// registered API). Each is matched by replacementMatches's per-event
	// branch above and applied by applyReplacements's dispatch.
	//
	// The life classes register too: repl:GainLife (the Prevent$ GainLife
	// shape plus the CantGainLife static's replacement arm) and
	// repl:LifeReduced (the ReplaceCount$Amount/Twice shape), both applied by
	// applyLifeReplacements. repl:DamageDone and repl:Counter are this
	// ticket's own additions, matched by replacementMatches's DamageDone case
	// and CounterAllowed respectively.
	// kw:Bloodthirst (CR 702.54) is implemented by this file's
	// bloodthirstEntryMatch: the keyword line (printed K:Bloodthirst:<N> or a
	// layer-6 AddKeyword$ grant) is read at MoveZone→Battlefield collection
	// time from the entering object's DERIVED keyword list, so one read
	// covers both shapes -- a printed carrier and Twins of Discord's
	// `Affected$ Creature.Other+YouCtrl+Colorless | AddKeyword$ Bloodthirst:2`
	// grant, which cards-side expansion could never see. No cards-side
	// expansion exists: bloodthirst is a static ability whose whole meaning
	// is an entry-time conditional counter put, which is exactly what the
	// synthetic Repl below expresses.
	effects.RegisterNonAPI("kw:etbCounter", "kw:ETBReplacement", "kw:Devour", "kw:Ravenous", "kw:Bloodthirst", "kw:Sunburst",
		"repl:Untap", "repl:BeginPhase", "repl:Transform", "repl:ProduceMana",
		"repl:GainLife", "repl:LifeReduced", "repl:DamageDone", "repl:Counter",
		"repl:CreateToken", "repl:RollPlanarDice", "repl:Explore", "repl:Attached", "repl:Scry", "api:ReplaceToken",
		"repl:AddCounter", "api:ReplaceCounter",
		// repl:Connive (Leader, Super-Genius) and repl:LoseMana (Ozai, Horizon
		// Stone, Kruphix, Omnath) are applied by engine/rules/replacement_connive.go and
		// engine/rules/replacement_lose_mana.go; this makes the coverage ratchet see them.
		"repl:Connive", "repl:LoseMana",
		// repl:GameLoss / repl:GameWin (task fdn-repl-cant-lose) are the
		// "you can't lose the game" / "your opponents can't win the game"
		// CantHappen class (Herald of Eternal Dawn, the Platinum Angel family,
		// Abyssal Persecutor, Lich's Mastery). Matching and application live in
		// engine/rules/cantlose.go; this registration is what makes the coverage
		// ratchet see the heads as supported.
		"repl:GameLoss", "repl:GameWin",
		// repl:Draw / repl:DrawCards (task cli-20261004T233421Z-ff9b2344) are
		// the draw-replacement class (Bard King of Dale's "draw two instead",
		// Notion Thief's redirect, Breathstealer's Crypt's unless-pay, and the
		// DrawCards multi-card alias Quantum Riddler and Alms Collector read as
		// "draw that many plus one"). replacementMatches's ReplDraw/ReplDrawCards
		// arm reads ValidPlayer$/NotFirstCardInDrawStep$/ActivePhases$/
		// FirstExtraCardDrawnThisTurn$/ValidCause$/CheckSVar$ and the shared
		// condition gate; the bodies apply through the ordinary ReplaceWith$
		// path (runReplaceWith) and the DB$ Draw re-draw. repl:DrawCards shares
		// repl:Draw's events.Draw match (replacementEventNameMatches's one
		// alias), so both names register together.
		"repl:Draw", "repl:DrawCards", "repl:Mill",
		// repl:TurnFaceUp (task cli-20260924T031747Z-6d0658fc) is the "as this
		// is turned face up" class (Hooded Hydra's five +1/+1 counters, Karlov
		// Watchdog's CantHappen, Gift of Doom's attach), matched by
		// replacementMatches's TurnFaceUp case and applied by
		// applyTurnFaceUpReplacements.
		"repl:TurnFaceUp",
		// api:ReplaceDamage is handled inline by applyReplaceDamageBody (this
		// file) via the ReplaceDamage intercept in applyReplacements, never
		// through effects.Resolve/runReplaceWith -- this registration is the
		// census token only; a stub effects.Register handler would be dead code.
		"api:ReplaceDamage",
		// api:ReplaceSplitDamage is likewise inline (applyReplaceSplitDamageBody).
		"api:ReplaceSplitDamage")
}

// cmdZoneMove is one commander command-zone ask. For CR 903.9b it is a parked
// zone change (Task m32): the MoveZone event a commander is about to undergo,
// deferred until its owner decides whether to put it into the command zone
// instead. The answer re-emits the park as a real MoveZone -- to ZCommand on
// an accept, to the parked destination verbatim on a decline -- so the event
// log always carries the zone change that actually happened and a log-only
// replay reproduces it. For CR 903.9a (sba) the commander is already in the
// graveyard or exile and nothing is deferred. Plain value data (an
// events.Event plus the moving object's id), so Clone copies the queue with
// one slice copy.
type cmdZoneMove struct {
	ev     events.Event
	obj    state.ObjID
	before *triggerSnapshot // immutable SBA look-back, safe to share in Clone
	// sba marks a CR 903.9a state-based-action entry: the commander is
	// already in ev.From (graveyard or exile) and nothing is deferred. An
	// accept moves it ev.From -> command zone; a decline moves nothing.
	sba bool
}

// commanderZoneReplacementApplies is CR 903.9b's match predicate: a
// commander, in a Commander-format game, about to be put into its owner's
// hand or library -- from ANYWHERE. A graveyard or exile destination is
// NOT replaced: CR 903.9a makes that a state-based action
// (commanderZoneSBA), so the move happens and its triggers fire first.
//
//   - FormatCommander gate: a Constructed game never runs any of this.
//   - The two destinations: hand and library.
//   - "From anywhere": ev.From is never consulted.
//   - Ownership, not control: the lookup is the owner's Commanders list
//     (isCommander), and its owner is asked.
func (e *Engine) commanderZoneReplacementApplies(ev events.Event) bool {
	if e.format != FormatCommander {
		return false
	}
	if ev.To != state.ZHand && ev.To != state.ZLibrary {
		return false
	}
	if ev.Obj == e.cmdZoneDeclined {
		return false
	}
	return e.isCommander(ev.Obj)
}

// isCommander reports whether id is in its owner's Commanders list.
func (e *Engine) isCommander(id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || int(o.Owner) >= len(e.G.Players) {
		return false
	}
	for _, c := range e.G.Players[o.Owner].Commanders {
		if c == id {
			return true
		}
	}
	return false
}

// collectCommanderArrivals scans the log from cmdZoneScan for MoveZone events
// that put a commander into a graveyard or exile and appends each such
// commander to cmdZoneDue once (CR 903.9a's "put into that zone since the
// last time state-based actions were checked").
func (e *Engine) collectCommanderArrivals() {
	n := len(e.L.Events)
	if e.cmdZoneScan > n {
		e.cmdZoneScan = n
	}
	for i := e.cmdZoneScan; i < n; i++ {
		ev := &e.L.Events[i]
		if ev.Kind != events.MoveZone || (ev.To != state.ZGraveyard && ev.To != state.ZExile) {
			continue
		}
		// CR 903.3b-c: a commander card that left the battlefield inside a
		// melded or mutated permanent moved with that permanent's MoveZone
		// (events.Move's SplitFrom stamp), not with one of its own. The
		// permanent may itself be a commander (a partner pair in one pile).
		if ev.From == state.ZBattlefield {
			e.collectSplitCommanders(ev.Obj, ev.To)
		}
		if !e.isCommander(ev.Obj) || slices.Contains(e.cmdZoneDue, ev.Obj) {
			continue
		}
		e.cmdZoneDue = append(e.cmdZoneDue, ev.Obj)
	}
	e.cmdZoneScan = n
	e.cmdZoneDep = e.G.DepSeq
}

// collectSplitCommanders queues, in seat then Commanders order, every
// commander card whose last move was as a component of the departed permanent
// from (Object.SplitFrom), put into to by a departure the scan has not yet
// checked (SplitDep above the cmdZoneDep watermark). A commander left in the
// zone by an EARLIER departure (declined, then from reanimated and dying
// again) stays below the watermark and is not asked a second time, while a
// host that only returned to the battlefield since (a blink: no departure,
// the ordinal does not move) does not invalidate the ask.
func (e *Engine) collectSplitCommanders(from state.ObjID, to state.Zone) {
	for p := range e.G.Players {
		for _, c := range e.G.Players[p].Commanders {
			if o := e.G.Obj(c); o != nil && o.SplitFrom == from && o.SplitDep > e.cmdZoneDep && o.Zone == to && !slices.Contains(e.cmdZoneDue, c) {
				e.cmdZoneDue = append(e.cmdZoneDue, c)
			}
		}
	}
}

// commanderZoneSBA is CR 903.9a / 704.6d. It walks cmdZoneDue in arrival
// order, drops every entry that is no longer eligible (not in a graveyard or
// exile any more, or its owner has left the game), and poses the choice for
// the FIRST eligible one: queued on cmdZone as an sba entry and asked of its
// owner. Entries behind it stay in cmdZoneDue for the check that follows the
// answer (Submit's tail). Nothing is posed while another decision or flow is
// outstanding -- the legend rule's guard -- and such a deferral marks the
// run unquiet so the next check is never skipped.
func (e *Engine) commanderZoneSBA() {
	if e.pending != nil || e.choosing != chooseNone || e.legendBatch != nil || len(e.cmdZone) > 0 {
		e.sbaUnquiet = true
		return
	}
	for len(e.cmdZoneDue) > 0 {
		id := e.cmdZoneDue[0]
		e.cmdZoneDue = e.cmdZoneDue[1:]
		o := e.G.Obj(id)
		if o == nil || (o.Zone != state.ZGraveyard && o.Zone != state.ZExile) || e.G.Players[o.Owner].Lost {
			continue
		}
		e.cmdZone = append(e.cmdZone, cmdZoneMove{
			ev:  events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZCommand, Player: o.Owner},
			obj: id, sba: true})
		e.askCommandZone(o.Owner)
		return
	}
}

// parkCommanderZoneMove applies the CR 903.9 replacement to one matching
// zone change: the event is deferred (never logged, never applied -- the
// commander stays where it is) and its owner is asked whether to put the
// commander into the command zone instead. handleCmdZone emits the parked
// move for real when the answer lands.
//
// Dedup by object: while a commander's decision is outstanding the only
// engine work that can run is (a) the resolution-chain tail that already
// emitted the first park and (b) Submit's repeating state-based-action pass.
// A second park for the same commander is therefore either the SAME zone
// change being re-offered (the SBA pass re-finding a commander it already
// had tried -- the no-progress shape a replacement must not loop on, the
// stalledCastLimit lesson: the already-pending decision covers it, so the
// duplicate is dropped) or a second effect in the same chain that in the
// real rules would resolve AFTER the commander has already moved (its own
// CR 608.2b Origin$-driven recheck would then skip it, so dropping is the
// more-correct outcome, not merely safe). The move itself always happens
// exactly once, through the front decision.
//
// An owner who has left the game cannot exercise a "may" choice (CR 800.4a:
// a departed player makes no choices), so the unexercised choice is a
// decline: the original move happens unchanged, emitted here with
// cmdZoneDeclined set so the commander check that just matched cannot
// re-park it (CR 616.1, a replacement applies only once).
func (e *Engine) parkCommanderZoneMove(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return
	}
	owner := o.Owner
	if e.G.Players[owner].Lost {
		saved := e.cmdZoneDeclined
		e.cmdZoneDeclined = ev.Obj
		e.emit(ev)
		e.cmdZoneDeclined = saved
		return
	}
	for _, pm := range e.cmdZone {
		if pm.obj == ev.Obj {
			return
		}
	}
	e.cmdZone = append(e.cmdZone, cmdZoneMove{ev: ev, obj: ev.Obj, before: e.retainTriggerBefore()})
	if e.pending == nil {
		e.askCommandZone(owner)
	}
}

// askCommandZone poses the CR 903.9 choice for the FRONT parked move to its
// owner, following askTriggerOptional's shape: Min == Max == 1 over two
// options, first the "change the outcome" one, then the "let it happen"
// one. Only the front of the queue is ever asked -- see handleCmdZone's
// resumption for how the queue hands from one decision to the next.
func (e *Engine) askCommandZone(owner state.PlayerID) {
	pm := e.cmdZone[0]
	name := "this commander"
	if o := e.G.Obj(pm.obj); o != nil && o.Face() != nil && o.Face().Name != "" {
		name = o.Face().Name
	}
	dest := pm.ev.To.String()
	into := "Put " + name + " into the command zone"
	prompt := name + " would go to the " + dest + ": put it into the command zone instead?"
	leave := "Let it go to the " + dest
	keep := ""
	if pm.sba {
		dest = pm.ev.From.String()
		prompt = name + " is in the " + dest + ": put it into the command zone?"
		leave = "Leave it in the " + dest
		if pm.ev.From == state.ZExile {
			if keep = e.exileCastPermission(pm.obj); keep != "" {
				leave += " (it can still be cast from there)"
			}
		}
	}
	d := &decision.Decision{Player: owner, Kind: decision.KCommanderZone, Min: 1, Max: 1,
		Prompt: prompt,
		Source: pm.obj,
		Options: []decision.Option{
			{Index: 0, Kind: "command_zone", Label: into, Obj: pm.obj, Player: owner},
			{Index: 1, Kind: "leave", Label: leave, Obj: pm.obj, Player: owner, Mode: keep},
		}}
	// Inside a tape run's resolution the choice is served from the tape and
	// handled in place (the parked move applies where it happened), so a
	// later ask of the same resolution ("its controller may search") is
	// served too instead of being defaulted behind an outstanding decision.
	// Elsewhere it is posed as the engine flow's own decision.
	parkAsk(e, d)
}

// exileCastPermission names the standing permission to cast (or, for a
// land-front Adventure, play) id from exile that a move to the command zone
// would end (CR 400.7: a new object), or "": an Adventure spell that resolved
// (CR 715.3d), a card exiled by the Foretell action (CR 702.143a) or a plotted
// card (CR 702.170). The strings are the Kind/Mode values the exile walk
// offers those actions under, so a client can match the CR 903.9a ask to the
// later cast. The later-turn gates of foretell and plot are deliberately not
// read: the permission is held now and lost if moved.
func (e *Engine) exileCastPermission(id state.ObjID) string {
	o := e.G.Obj(id)
	switch {
	case o == nil || o.Zone != state.ZExile:
		return ""
	case adventureSpellFace(o) != nil && o.FaceIdx == 0 && e.adventureZoneAvailable(id):
		if o.Card.Faces[0].IsLand() {
			return "play_land"
		}
		return "adventure_recast"
	case e.foretoldExileIndex(id) >= 0:
		return "foretell_cast"
	case o.PlottedTurn > 0:
		return "plot_cast"
	}
	return ""
}

// handleCmdZone applies an answered CR 903.9 decision. For a CR 903.9a sba
// entry an accept moves the commander from its graveyard or exile to the
// command zone (an ordinary emit) and a decline moves nothing. For a CR
// 903.9b entry the front parked move is emitted for real -- to the command
// zone if the owner chose "command_zone", subject to every other replacement
// effect, with only the CR 903.9b commander replacement skipped
// (cmdZoneDeclined), if they chose "leave" -- and, if more moves are parked,
// the next one's owner is asked. The CR 903.9b replacement has already been
// offered for this zone change, and CR 616.1 lets a replacement effect apply
// only once, so the final move is never re-parked and never re-asked -- a
// decline therefore cannot spin the engine by re-offering the same choice,
// and the SBA pass that re-finds the commander after this answer degrades the
// same way it would for any other completed replacement (the dedup in
// parkCommanderZoneMove handled its in-flight copy).
//
// The owner's choice is in the log as the Intents entry plus the
// DecisionAsk/DecisionMade events every decision emits; the outcome is the
// MoveZone event below, so a log-only replay reproduces both branches from
// the log alone. An answer with no parked move (only reachable from a
// hand-built decision -- every real ask parks one) degrades to a Note, the
// same totality stance as handleModes.
func (e *Engine) handleCmdZone(d *decision.Decision, in decision.Intent) {
	if len(e.cmdZone) == 0 {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "commander-zone decision answered with no move parked"})
		return
	}
	pm := e.cmdZone[0]
	e.cmdZone = e.cmdZone[1:]
	accept := false
	if opts := d.Chosen(in); len(opts) == 1 && opts[0].Kind == "command_zone" {
		accept = true
	}
	switch {
	case pm.sba && accept:
		if o := e.G.Obj(pm.obj); o != nil && o.Zone == pm.ev.From {
			e.emit(pm.ev)
		}
	case pm.sba:
		// Decline: the commander stays where it is.
	case accept:
		saved, before := e.applyingReplacement, e.triggerBefore
		e.applyingReplacement, e.triggerBefore = true, pm.before
		e.emit(events.Event{Kind: events.MoveZone, Obj: pm.ev.Obj, From: pm.ev.From,
			To: state.ZCommand, Player: pm.ev.Player, Text: pm.ev.Text})
		e.applyingReplacement, e.triggerBefore = saved, before
	default:
		savedDeclined := e.cmdZoneDeclined
		before := e.triggerBefore
		e.triggerBefore, e.cmdZoneDeclined = pm.before, pm.obj
		e.emit(pm.ev)
		e.triggerBefore, e.cmdZoneDeclined = before, savedDeclined
	}
	if len(e.cmdZone) > 0 && e.pending == nil {
		// More commanders were parked in the same burst (a board wipe, a
		// multiple-SBA pass): hand the front of the queue to its owner.
		if o := e.G.Obj(e.cmdZone[0].obj); o != nil && int(o.Owner) < len(e.G.Players) {
			e.askCommandZone(o.Owner)
		}
	}
}
