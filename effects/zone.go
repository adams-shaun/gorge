package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ChangeZone", effChangeZone)
	Register("ChangeZoneAll", effChangeZoneAll)
	Register("Destroy", effDestroy)
	Register("DestroyAll", effDestroyAll)
	Register("Sacrifice", effSacrifice)
	Register("Manifest", effManifest)
	Register("Cloak", effCloak)
}

// ParseZone maps a Forge zone name to a state.Zone. Unknown names resolve to
// the graveyard, which is where the overwhelming majority of movement goes
// and is a safe default for an unmodelled destination.
func ParseZone(s string) state.Zone {
	z, ok := parseZone(s)
	if !ok {
		return state.ZGraveyard
	}
	return z
}

// ParseZoneWord is parseZone's exported form for callers that must react to
// an UNKNOWN zone word (fail closed) rather than silently degrading to a
// graveyard the way ParseZone does: the trigger-side PresentZone$ clause's
// recognised-vocabulary check shares this one classification with every
// other zone word so the two cannot disagree.
func ParseZoneWord(s string) (state.Zone, bool) {
	return parseZone(s)
}

func parseZone(s string) (state.Zone, bool) {
	switch strings.TrimSpace(s) {
	case "Hand":
		return state.ZHand, true
	case "Battlefield":
		return state.ZBattlefield, true
	case "Library":
		return state.ZLibrary, true
	case "Graveyard":
		return state.ZGraveyard, true
	case "Exile":
		return state.ZExile, true
	case "Stack":
		return state.ZStack, true
	case "Command":
		return state.ZCommand, true
	case "Ceased":
		return state.ZCeased, true
	}
	return 0, false
}

// ParseZones parses an Origin$ zone set. Any and All are wildcards. A false
// result means at least one token was unknown; callers must not treat it as a
// graveyard origin.
func ParseZones(s string) (zones []state.Zone, all, ok bool) {
	ok = true
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "Any" || part == "All" {
			all = true
			continue
		}
		z, known := parseZone(part)
		if !known {
			ok = false
			continue
		}
		if !zoneIn(zones, z) {
			zones = append(zones, z)
		}
	}
	return zones, all, ok
}

func zoneIn(zones []state.Zone, want state.Zone) bool {
	for _, z := range zones {
		if z == want {
			return true
		}
	}
	return false
}

// mixedOriginIncludesHand identifies every explicit multi-zone Origin$ that
// includes Hand. Such an effect needs one origin-aware hidden-zone chooser;
// the exact-Hand and exact-Library walkers cannot safely stand in for it.
func mixedOriginIncludesHand(zones []state.Zone, all bool) bool {
	return !all && len(zones) > 1 && zoneIn(zones, state.ZHand)
}

// changeZoneAltDestination resolves ChangeZone's conditional alternate
// destination (Forge's ChangeZoneEffect.handleAltDest): DestAltSVar$ names an
// SVar (or inline count expression) evaluated against the resolving host card
// and compared under DestAltSVarCompare$ (default GE1, i.e. truthy). When the
// condition holds, the move takes DestinationAlternative$ instead of
// Destination$.
//
// The optional "MANDATORY " prefix is stripped. Forge reads MANDATORY as the
// difference between forcing the alternate and offering the player a
// confirmAction; this engine has no destination-confirm ask, so BOTH branches
// take the alternate deterministically when the condition holds, and the
// non-mandatory shape records one Note disclosing the dropped confirm (the
// expansion-specific riders of six corpus carriers, all Destination$ Hand ->
// DestinationAlternative$ Battlefield). MANDATORY itself therefore changes no
// behaviour today; it is parsed so the two spellings cannot drift.
//
// Unlike CheckSVarHolds's other call sites, an unreadable condition here fails
// CLOSED to the primary destination (plus a Note): moving a card to a zone the
// condition cannot justify would be a silently wrong board, whereas keeping
// the primary is the pre-existing behaviour and therefore replay-safe.
func changeZoneAltDestination(h Host, c *Ctx, sa *cards.SA, primary state.Zone) state.Zone {
	cond := strings.TrimSpace(sa.Params["DestAltSVar"])
	if cond == "" {
		return primary
	}
	mandatory := false
	if rest, ok := strings.CutPrefix(cond, "MANDATORY "); ok {
		mandatory = true
		cond = strings.TrimSpace(rest)
	}
	holds, evaluated := CheckSVarHolds(h, c, cond, strings.TrimSpace(sa.Params["DestAltSVarCompare"]))
	if !evaluated {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "DestAltSVar$ " + strings.TrimSpace(sa.Params["DestAltSVar"]) +
				" is not a condition this engine can evaluate; the move takes the primary destination"})
		return primary
	}
	if !holds {
		return primary
	}
	alt, ok := ParseZoneWord(sa.Params["DestinationAlternative"])
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "DestAltSVar$ " + strings.TrimSpace(sa.Params["DestAltSVar"]) +
				" holds but DestinationAlternative$ " + strings.TrimSpace(sa.Params["DestinationAlternative"]) +
				" is not a zone this engine models; the move takes the primary destination"})
		return primary
	}
	if !mandatory {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "DestAltSVar$ " + strings.TrimSpace(sa.Params["DestAltSVar"]) +
				" holds: the alternate destination " + strings.TrimSpace(sa.Params["DestinationAlternative"]) +
				" is taken (Forge would ask which destination; this engine does not ask)"})
	}
	return alt
}

func effChangeZone(h Host, c *Ctx, sa *cards.SA) {
	to := changeZoneAltDestination(h, c, sa, ParseZone(sa.Params["Destination"]))
	var originZones []state.Zone
	var originAll bool
	if from, present := sa.Params["Origin"]; present {
		var valid bool
		originZones, originAll, valid = ParseZones(from)
		// OriginAlternative$ is Forge's "and/or" second origin: the zones
		// named there join Origin$ into ONE candidate set at the
		// choose-a-card-from-any-of-these-zones step ("search your graveyard,
		// hand, and/or library"). Every one of the corpus's 62 carriers pairs
		// it with Origin$ Library; without this merge the exact-Library branch
		// below sees a library-only origin and silently searches just that.
		// The merge is deliberately SCOPED to this parameter (altPresent below
		// gates the widened search branch): a compound Origin$ WITHOUT an
		// OriginAlternative$ keeps its pre-existing object path -- Eladamri,
		// Korvecdal's `Defined$ ChosenCard | Origin$ Library,Hand` must move
		// the already-chosen card, never pose a fresh whole-library pick.
		// Parse with the same vocabulary as Origin$. A zone word ParseZones
		// does not model (Sideboard) is noted loudly and dropped from the
		// merged set while every KNOWN zone keeps searching -- bailing the
		// whole effect (folding altValid into `valid`) would lose the library
		// half of invasion_of_arcavios's "library, graveyard, and/or outside
		// the game", a regression over the pre-OriginAlternative engine,
		// which still searched the library.
		var altPresent bool
		if alt, hasAlt := sa.Params["OriginAlternative"]; hasAlt {
			altPresent = true
			altZones, altAll, altValid := ParseZones(alt)
			for _, z := range altZones {
				if !zoneIn(originZones, z) {
					originZones = append(originZones, z)
				}
			}
			originAll = originAll || altAll
			if !altValid {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
					Text: "unrecognised ChangeZone OriginAlternative " + alt})
			}
		}
		hidden := strings.EqualFold(strings.TrimSpace(sa.Params["Hidden"]), "True")
		// ... and the branch excludes every origin the dedicated walkers own:
		// exactly-Library is the search below, exactly-Hand the hand movers,
		// a mixed-Hand origin the loud note -- and this branch must sit BEFORE
		// the unrecognised-Origin bail, because Origin$ Sideboard parses to no
		// modelled zone at all yet still resolves (Burning Wish's wish, whose
		// SubAbility$ self-exile must run). Origin$ All stays on the object
		// path too -- every no-Defined$ corpus line naming it carries Defined$
		// (all eight are Dauthi-shaped replacements), and a game-wide all-zones
		// pick would offer hidden hand/library cards by name. When a Defined$
		// DOES name the objects, Forge's resolver takes them without a choose
		// ask, reveals nothing and shuffles nothing (`!defined` fails both the
		// reveal and the shuffle conditions) -- exactly what the object path
		// below already performs, which is why Dauthi Voidwalker's Hidden$
		// "exile it instead" replacement carries no behaviour of its own
		// beyond this read.
		if hidden && sa.Params["Defined"] == "" &&
			!strings.EqualFold(strings.TrimSpace(sa.Params["Imprint"]), "True") &&
			!originAll && !mixedOriginIncludesHand(originZones, originAll) &&
			!zoneIn(originZones, state.ZLibrary) &&
			!(len(originZones) == 1 && originZones[0] == state.ZHand) {
			effHiddenPick(h, c, sa, to, originZones, originAll, valid, from)
			return
		}
		// hiddenpick1: Forge's SpellAbility.isHidden() (hasParam("Hidden") ||
		// the origin zones hold hidden info) routes the resolution through the
		// hidden-origin resolver (ChangeZoneEffect.changeHiddenOriginResolve)
		// even when the origin zones are PUBLIC. There the objects are not the
		// source default: with no Defined$ the fetch list is the origin zones'
		// cards matching ChangeType$ -- game-wide for a public origin when no
		// fetch player is named (Kor Skyfisher's bounce, Temur Sabertooth's
		// "another creature", the graveyard/exile mill follow-ups) or the
		// DefinedPlayer$/targeted player's own zones (Relic of Progenitus) --
		// and the resolver asks the chooser to pick ChangeNum$ of them. The
		// object path below would instead move the Defined() source default
		// silently (a self-bounce) or skip the player fetchers entirely (a
		// silent no-op). Origin$ Sideboard must resolve despite parsing to no
		// modelled zone (Burning Wish's wish, whose SubAbility$ self-exile
		// must run), which is why the branch sits before the
		// unrecognised-Origin bail. Origin$ All stays on the object path too
		// -- every no-Defined$ corpus line naming it carries Defined$ (all
		// eight are Dauthi-shaped replacements), and a game-wide all-zones
		// pick would offer hidden hand/library cards by name. When a Defined$
		// DOES name the objects, Forge's resolver takes them without a choose
		// ask, reveals nothing and shuffles nothing (`!defined` fails both the
		// reveal and the shuffle conditions) -- exactly what the object path
		// below already performs, which is why Dauthi Voidwalker's Hidden$
		// "exile it instead" replacement carries no behaviour of its own
		// beyond this read.
		if !valid {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unrecognised ChangeZone Origin " + from})
			return
		}
		// A ChangeZone whose origin set includes Library and no other hidden
		// walker's zone is the hidden-origin search, now spanning every zone
		// Origin$ plus OriginAlternative$ named (the and/or shapes). The
		// widened cross-zone shape fires ONLY when OriginAlternative$ is
		// present: a compound Origin$ alone keeps its existing dispatcher, so
		// a Defined$-bearing carrier (Eladamri, Korvecdal) still takes its
		// already-chosen objects. The
		// searching player may fail to find a card with the stated quality (Min
		// is always zero), and the answer resumes this same effect before its
		// SubAbility runs. The exact-Library spelling is the single-zone case of
		// the same path; the alternatives are PUBLIC zones (Graveyard, Exile,
		// Hand) whose candidates join the library's in one option list. A
		// mixed-Hand alternative (Gate to the Afterlife's Graveyard,Hand) is
		// deliberately OWNED here rather than by the mixed-origin note below,
		// because the search IS the origin-aware chooser that note says does not
		// exist: the fetch player sees their own hand, so no hidden information
		// is exposed by offering it by name.
		if zoneIn(originZones, state.ZLibrary) && !originAll &&
			!zoneIn(originZones, state.ZBattlefield) &&
			(altPresent || len(originZones) == 1) {
			// Forge treats a Defined$ that resolves to objects in a hidden
			// library as the already-selected fetch list, not as the owner of a
			// fresh whole-library search. This is structural rather than keyed to
			// Remembered: ChosenCard, TopOfLibrary once resolved, and future
			// object-valued Defined selectors share the same dispatcher. Only the
			// single-zone case takes it: with OriginAlternative$ present the
			// corpus carries no Defined$ (measured 0 of 62), so this is latent
			// rather than live.
			if len(originZones) == 1 && moveDefinedLibraryObjects(h, c, sa, to) {
				return
			}
			effSearchLibrary(h, c, sa, to, originZones)
			return
		}
		// A mixed origin which includes Hand needs one chooser over cards from
		// every origin. The exact-Hand handlers below cannot provide that
		// origin-aware option list, so record the gap before retaining the
		// object path for a Defined$ card that is already known. In particular,
		// a source-default mixed-origin picker (Kastral, the Windcrested) now
		// fails loudly rather than silently doing nothing. Keep this test on
		// parsed zones rather than a list of origin strings: every new
		// Hand,<other-zone> spelling takes this same visible fallback.
		if mixedOriginIncludesHand(originZones, originAll) {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot choose cards from mixed ChangeZone Origin$ " + from +
					" (an origin-aware hidden-zone chooser is not implemented)"})
		}
		// A ChangeZone from exactly Hand with no object selector is Forge's
		// hidden-origin hand put-back: the chooser picks ChangeNum$ cards (a
		// ChangeType$ filter narrows the pool; its absence -- Brainstorm's "put
		// two cards from your hand on top of your library", Jace, the Mind
		// Sculptor's [0] -- offers the whole hand). With no Defined$/
		// DefinedPlayer$/ValidTgts$ the object path below would resolve Defined
		// to the SOURCE default and then skip every candidate on the Origin$
		// precondition -- the silent no-op the handmove1 fix replaces with a
		// real hand choice (the rv2b extension drops handmove1's ChangeType$
		// requirement: the whole 239-line no-selector Origin$ Hand population
		// routes here now, 19 of it untyped).
		// An SVar or inline count expression is evaluated through Num where the
		// count grammar supports it (for example Wrenn and Seven's SVar X counts
		// lands in hand). An unknown count remains loud rather than falling through
		// to the old source-default no-op: it emits a Note and moves nothing.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] == "" &&
			sa.Params["DefinedPlayer"] == "" && sa.Params["ValidTgts"] == "" &&
			!strings.EqualFold(sa.Params["Imprint"], "True") {
			if _, supported := handMoveCountOf(h, c, sa); supported {
				effChangeZoneHand(h, c, sa, to)
				return
			}
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot choose ChangeNum$ " + strings.TrimSpace(sa.Params["ChangeNum"]) +
					" cards from hand (a non-literal count is not a bound this engine can evaluate)"})
			return
		}
		// A ChangeZone from exactly Hand whose hand OWNER is selected --
		// DefinedPlayer$-alone (Kynaios and Tiro's "each player may put a land
		// card from their hand onto the battlefield", Braids, Conjurer Adept,
		// Mindleech Ghoul) or ValidTgts$-alone naming the players (Karn
		// Liberated's "[+4]: Target player exiles a card from their hand",
		// Kyoki, Sanity's Eclipse) -- is the per-owner hidden-hand shape: one
		// chooser ask per hand owner, chained through the persisted
		// Ctx.HandMoveTarget cursor. The rv2b r2 finding: these shapes used to
		// fall through to the object path, where Defined() resolved to the
		// source (or to targets the Origin$ precondition then skipped because
		// they are PLAYERS, not hand cards) -- another silent no-op.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] == "" &&
			(sa.Params["DefinedPlayer"] != "" || sa.Params["ValidTgts"] != "") {
			effChangeZoneHandOwners(h, c, sa, to)
			return
		}
		// DefinedPlayer$ alongside a Defined$ that names concrete objects
		// (Wilt-Leaf Liege's DefinedPlayer$ ReplacedPlayer + Defined$
		// ReplacedCard, 1 corpus line) keeps the object path -- the moved
		// objects are already named -- but the owner parameter is unread
		// there, so the shape is loud about it rather than silent.
		if len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
			sa.Params["Defined"] != "" && sa.Params["DefinedPlayer"] != "" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "DefinedPlayer$ " + sa.Params["DefinedPlayer"] +
					" is unread next to Defined$ " + sa.Params["Defined"] +
					" (the move goes to the named objects alone)"})
		}
	}
	// WithCountersType$/WithCountersAmount$ make the move put counters on the
	// permanent it lands on the battlefield with -- the Undying expansion's
	// "return to the battlefield with a +1/+1 counter" (cards/keywords.go). The
	// CounterChange is emitted AFTER the MoveZone, so it lands on the moved
	// (new) object's back at its destination, exactly as Move waiting to run
	// first would want, and the counter survives onto the permanent because it
	// is added post-move. Counter (not the Move carrying it along) is what
	// keeps events/apply.go's Move from knowing anything about counters.
	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	// WithCounters* only takes effect when the object enters the battlefield.
	// Parsing a dynamic/malformed amount emits a Note, so do not parse it for
	// another destination where no CounterChange can ever be emitted.
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	targets := Defined(h, c, sa)
	// ValidTgts$ targeting whose ask was never offered: the placement ask
	// (rules' pushTrigger) reads only the trigger's OWN Execute SA, so a
	// deeper sub's ValidTgts$ -- the "when you do" family's shape (Forum
	// Filibuster's `TrigReturn`, 134 raw corpus T: chains reaching one) --
	// arrives here with no chosen targets and used to move nothing silently.
	// Offer the targets now, through the same Host.LegalTargets census the
	// announcement ask uses (targetZones' Origin$-implied graveyard included),
	// as a KChoose over the shared "choice" resume arm; the answer lands in
	// Ctx.Choice and the re-entered pass consumes it (fx42 scoping -- a nested
	// ChangeZone in the same chain poses its own ask). A host that cannot ask
	// takes the deterministic first-max stand-in (R-9, the same mirror the
	// effDig ask's botpolicy arm answers with option 0). The chosen bounds are
	// TargetMin$/TargetMax$ through the ordinary Num grammar (TrigReturn's
	// TargetMin$ 0 / TargetMax$ 1 -- "up to one"), clamped to the eligible
	// count; a bound pair that admits nothing (Min == Max == 0, or no eligible
	// candidate) poses no ask and moves nothing -- a decision nobody could
	// answer differently is never emitted.
	if ans, ok := changeZoneChosenTargets(h, c, sa); ok {
		targets = ans
	}
	// The O-Ring return shape (Journey to Nowhere, Leonin Relic-Warder): the
	// LEAVE-battlefield trigger's Execute is `DB$ ChangeZone | Defined$
	// Remembered`, and Forge reads the HOST CARD's remembered list there --
	// the cross-resolution memory RememberTargets$ wrote -- not this build's
	// Ctx binding. This build's trigger resolutions seed Ctx.Remembered with
	// the event capture (triggerRemembered), which for a self-trigger is
	// exactly [{source}], so the ctx set is distinguishable: when the
	// resolved set is exactly that capture and the source's persistent
	// Remembered is non-empty, the card's list is what the script meant.
	// Mid-chain readings are unaffected: a chain that remembered its own
	// source through the object path below wrote BOTH halves (ctx and
	// persistent), so the replacement is the same set; a hand-path
	// RememberChanged$ writes ctx only and leaves the persistent list empty,
	// so the guard keeps the ctx set.
	if sa.Params["Defined"] == "Remembered" {
		if len(targets) == 1 && !targets[0].IsPlayer && targets[0].Obj == c.Source {
			if src := h.Game().Obj(c.Source); src != nil && len(src.Remembered) > 0 {
				targets = append([]state.Target(nil), src.Remembered...)
			}
		}
	}
	// ForgetOtherTargets$ True (Journey to Nowhere, Leonin Relic-Warder):
	// Forge's ChangeZoneEffect.forgetOtherTargets -- forget every previously
	// remembered object before this effect resolves, so a source that
	// remembered something earlier (a re-entered O-Ring exiling a second
	// creature) remembers only its own targets and the return trigger
	// returns exactly this effect's set. Both halves clear: the resolution's
	// ctx list and the source's event-backed persistent one.
	if strings.EqualFold(strings.TrimSpace(sa.Params["ForgetOtherTargets"]), "True") {
		c.Remembered = nil
		clearEventRemembered(h, c)
	}
	// Imprint effects such as Chrome Mox select eligible cards from their
	// controller's hand. Keep them out of the generic hand mover so their
	// successful exile can be recorded in the replayable Imprint event.
	if len(targets) == 1 && targets[0].Obj == c.Source && !targets[0].IsPlayer &&
		len(originZones) == 1 && originZones[0] == state.ZHand && !originAll &&
		strings.EqualFold(sa.Params["Imprint"], "True") {
		targets = nil
		if c.ImprintDone {
			for _, id := range c.Imprint {
				targets = append(targets, state.Target{Obj: id})
			}
			c.Imprint, c.ImprintDone = nil, false
		} else {
			spec := sa.Params["ChangeType"]
			if spec == "" {
				spec = "Card"
			}
			for _, id := range h.Game().Zone(state.ZHand, c.Controller) {
				if o := h.Game().Obj(id); o != nil && MatchesSpecCtx(h.Game(), spec, id, c.SpecContext(c.Controller)) {
					targets = append(targets, state.Target{Obj: id})
				}
			}
			max := Num(h, c, sa, "ChangeNum", 1)
			if int32(len(targets)) > max {
				d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: int(max), Max: int(max), Source: c.Source,
					ResumeKind: "imprint", ResumeSA: sa, Prompt: "Choose a card to imprint"}
				for _, target := range targets {
					o := h.Game().Obj(target.Obj)
					d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "imprint", Obj: target.Obj, Label: o.Face().Name})
				}
				if h.Ask(d) {
					return
				}
				targets = targets[:max]
			}
		}
	}
	var imprinted []state.ObjID
	// Forge keeps every DB$ Effect in an implicit "effect" object in the
	// Command zone, and the corpus's one-shot idiom `DB$ ChangeZone | Defined$
	// Self | Origin$ Command | Destination$ Exile` is that effect object
	// exiling itself -- ending the effect after one use (Deflecting Palm's
	// RPreventNextFromSource: "the NEXT time the chosen source would deal
	// damage"). This build has no effect object, so when the chain resolves
	// inside an Effect-created replacement's body (Ctx.EffectFrame is bound
	// by rules' seedEffectReplCtx) and the ChangeZone names that frame's own
	// source, the shape ends exactly that registration. Everywhere else the
	// ordinary move below runs unchanged (it moves nothing: the named source
	// is not in the Command zone), so no other resolution changes.
	if f := c.EffectFrame; f.Source != 0 && f.Source == c.Source && to == state.ZExile && !originAll &&
		len(originZones) == 1 && originZones[0] == state.ZCommand &&
		len(targets) == 1 && !targets[0].IsPlayer && targets[0].Obj == c.Source {
		h.EndEffect(f.Source, f.Stamp)
		return
	}
	// The objects the move loop actually moved, in move order: ChangeZone's
	// AtEOT$ affected set is the MOVED objects (some carriers carry
	// RememberChanged$ and some do not, so the moved set is collected here
	// rather than read back out of Remembered).
	var moved []state.ObjID
	for _, t := range targets {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		// Origin$, when given, is a precondition: the object must actually be
		// where the script expects, or the movement does not happen. This is
		// also this build's only CR 608.2b guard for ChangeZone: a target
		// moved away by an earlier effect in the same resolution, or by a
		// response that has already resolved, is simply skipped rather than
		// moved a second time or moved from the wrong zone.
		if _, present := sa.Params["Origin"]; present && !originAll && !zoneIn(originZones, o.Zone) {
			continue
		}
		// Inlined rather than routed through settleChangeZoneMove: this loop
		// carries the exiled-with association and the RememberChanged$
		// event-backed rider (eventRemember) in a specific order (MoveZone,
		// exiled-with, RememberChanged, WithCounters) that predates the
		// shared settle helper, and neither is shared with that helper's
		// other callers (see settleChangeZoneMoveAs's doc comment). The
		// MoveZone event itself still goes through moveZoneEvent (every exile
		// mover shares that one constructor) plus the same Imprint$True/
		// ExiledWithSource-static IDs augmentation settleChangeZoneMoveAs
		// applies, so events.Apply's ExiledWith-scalar derivation (o.ExiledWith
		// = e.IDs[0]) fires here exactly as it does on that path -- Chrome
		// Mox's own DefinedCards$ ExiledWith read needs it, not just the
		// distinct ExiledCards list exiledWithAssociation below maintains.
		ev := moveZoneEvent(c, o.ID, o.Zone, to)
		if to == state.ZExile && len(ev.IDs) == 0 && (faceStaticsNameExiledWithSource(h, c.Source) || strings.EqualFold(strings.TrimSpace(sa.Params["Imprint"]), "True")) {
			ev.IDs = []state.ObjID{c.Source}
		}
		applyFaceDownMarker(h, sa, c, &ev, to)
		fromZone := o.Zone
		h.Emit(ev)
		moved = append(moved, o.ID)
		exiledWithAssociation(h, c, o.ID, to)
		if to == state.ZExile {
			recordExileReturn(h, c, sa, o.ID, fromZone, to)
		}
		if to == state.ZBattlefield {
			applyTransformed(h, c, sa, o.ID)
		}
		// RememberLKI$ True (Reanimate's "creature card" whose mana value the
		// chained lose-life SVar reads, RememberedLKI$CardManaCost) joins the
		// moved object to the ability's Remembered -- a resolution-local Ctx
		// value, replayed identically because replay re-runs the same SA. The
		// two flags stack; an object is not remembered twice.
		if strings.EqualFold(sa.Params["RememberLKI"], "True") &&
			!strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
		}
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
			eventRemember(h, c, o.ID)
		}
		// RememberTargets$ True (Journey to Nowhere's exile trigger, Bile
		// Blight's Pump sibling): the CHOSEN TARGETS join the ability's
		// Remembered, in both halves -- the ctx list the chain's later
		// sub-abilities read (Bile Blight's PumpAll Remembered.sameName) and
		// the source's event-backed persistent list, which a LATER, separate
		// resolution reads through Defined$ Remembered via the O-Ring rescue
		// above (Journey's leave-battlefield return trigger). Only a target
		// the move actually moved is remembered: a target skipped by the
		// Origin$ precondition was never exiled and must never come back.
		if strings.EqualFold(strings.TrimSpace(sa.Params["RememberTargets"]), "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: o.ID})
			eventRemember(h, c, o.ID)
		}
		eventForgetChanged(h, c, sa, o.ID)
		if withKind != "" && to == state.ZBattlefield {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: withKind, Amount: withAmt})
		}
		// GainControl$ hands the moved object to the named player (Reanimate:
		// "return target creature card... to the battlefield under your
		// control"). Only a battlefield entry can carry a control change (CR
		// 701.22a controls permanents); a card moved to a hidden or public
		// non-battlefield zone keeps its owner. Not part of the "inlined
		// rather than settleChangeZoneMove" scoping above -- GainControl$ is
		// unconditional on the move landing on the battlefield, the same as
		// settleChangeZoneMoveAs's own tail call.
		if to == state.ZBattlefield {
			applyGainControl(h, c, sa, o.ID)
			changeZoneAttachedTo(h, c, sa, o.ID)
		}
		// Tapped$ True (CR 110.5's entry state): the moved permanent enters
		// tapped. The object path did not apply this rider before, so a
		// targeted graveyard/exile return carrying it (Zuko's Conviction's
		// kicked alternate, every "return it to the battlefield tapped"
		// spell) entered untapped -- the same Tap event applyLibrarySearch
		// and the hand movers emit, so the entry state is a real event and
		// replay derives it.
		if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
			h.Emit(events.Event{Kind: events.Tap, Obj: o.ID, Player: c.Controller, Text: "entered tapped"})
		}
		// StaticEffect$ on the inlined object path: the same rider the shared
		// settle path applies for every other mover (the main loop deliberately
		// predates settleChangeZoneMoveAs and is not routed through it).
		if to == state.ZBattlefield {
			applyStaticEffect(h, c, sa, to, []state.ObjID{o.ID})
		}
		if strings.EqualFold(sa.Params["Imprint"], "True") && to == state.ZExile {
			if moved := h.Game().Obj(o.ID); moved != nil && moved.Zone == state.ZExile {
				imprinted = append(imprinted, o.ID)
			}
		}
	}
	if len(imprinted) > 0 {
		h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: imprinted})
	}
	// AtEOT$ (Puppeteer Clique's reanimation: "at the beginning of your next
	// end step, exile it"): schedule the end-step departure for every object
	// this move actually moved.
	scheduleAtEOT(h, c, sa, moved)
}

// changeZoneAttachedTo implements ChangeZone's AttachedTo$ param: "the moved
// card enters the battlefield attached to the resolved target" (Forum
// Filibuster's `AttachedTo$ DelayTriggerRememberedLKI` -- attach the returned
// Aura to the remembered token; the 42 raw corpus ChangeZone lines carrying
// the param: Self x15 is the dominant form, "return an Aura ... attached to
// CARDNAME"). The value is a Defined$-grammar selector, resolved with the
// ordinary resolver against a shallow SA that carries it in Defined$ (the
// same shape effToken's own AttachedTo$ rider takes), so every corpus
// spelling (Self, ParentTarget, TriggeredCardLKICopy, ChosenCard,
// DelayTriggerRememberedLKI, a card filter, ...) resolves without a second
// resolver. The Attach event is the same shape that rider emits: Obj is the
// MOVED card (it takes the AttachedTo back-reference), IDs[0] the target it
// attaches to. A value that resolves to no object -- a selector this grammar
// cannot evaluate, or a target that left play -- is ONE loud Note and the
// card enters unattached (an Aura's unattached state), never a guessed
// target and never a silent skip. Battlefield destinations only: the param
// on a move that does not enter the battlefield has no CR meaning (nothing
// can be attached in a hidden zone) and is left unread.
func changeZoneAttachedTo(h Host, c *Ctx, sa *cards.SA, moved state.ObjID) {
	val := strings.TrimSpace(sa.Params["AttachedTo"])
	if val == "" || moved == 0 {
		return
	}
	sub := *sa
	sub.Params = map[string]string{"Defined": val}
	// A bare card-filter spelling ("Creature" -- Retether's mass return;
	// "Creature.YouCtrl" -- One Last Job, Storm Herald, Nomad Mythmaker;
	// "Creature.sharesCreatureTypeWith <ref>" -- Runed Crown) is not a
	// Defined$ referent (definedSpec has no case for it) and MUST NOT ride
	// Defined's source fallback: that would fasten the moved Aura to the
	// resolving spell/ability itself, an attach the CR 704.5m SBA then
	// sweeps the moment the source leaves play. When knownDefinedTargets
	// cannot classify the value, resolve it as a battlefield card filter --
	// the same walk the Valid-prefixed branch runs -- so a spelling this
	// grammar cannot evaluate fails closed to the loud Note below, never to
	// a guessed attach. A value that is already the Valid-prefixed filter
	// form (Mantle of the Ancients' "Valid Creature.EnchantedBy") keeps its
	// own branch.
	if val != "Valid" && !strings.HasPrefix(val, "Valid ") {
		if _, ok := knownDefinedTargets(h, c, val); !ok {
			sub.Params["Defined"] = "Valid " + val
		}
	}
	var to state.ObjID
	for _, t := range Defined(h, c, &sub) {
		if !t.IsPlayer {
			to = t.Obj
			break
		}
	}
	if to == 0 || h.Game().Obj(to) == nil {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "ChangeZone AttachedTo$ " + val + " resolved to nothing; the card enters unattached"})
		return
	}
	h.Emit(events.Event{Kind: events.Attach, Obj: moved, IDs: []state.ObjID{to}})
}

// applyFaceDownMarker stamps a just-built ChangeZone MoveZone with the
// face-down encoding the card text asks for. Two spellings reach it, and they
// mean different CR things:
//
//   - ExileFaceDown$ True (Necropotence's "exile the top card of your library
//     face down"): the "exiled_with_face_down" decode sets Object.FaceDown AND
//     records the exiling source as the ExiledWith association -- the same
//     encoding Hideaway's face-down exile uses. The IDs provenance payload is
//     cleared so the two carriers cannot disagree on one event.
//   - FaceDown$ True (Yedora, Grave Gardener; the manifest marker's own
//     spelling) on a battlefield entry: the "entered_face_down" decode folds
//     Object.FaceDown plus the optional FaceDownSetType$/FaceDownPower$/
//     FaceDownToughness$ payload the card text names, exactly as a Manifest
//     does. A hand/library-origin face-down entry is marked Secret (its face
//     would otherwise leak through the transcript), matching the Manifest
//     precedent; a graveyard-origin one stays public (CR 708.9 already
//     revealed it on leaving the battlefield).
//   - FaceDown$ True on an exile destination (Tezzeret's Reckoning): the card
//     is put into exile face down WITHOUT an ExiledWith association, so the
//     bare spelling uses its own "face_down" marker rather than borrowing
//     ExileFaceDown$'s source-carrying one.
//
// It is called from every ChangeZone mover (the object path, the shared
// settle helper the hand/library routes use, and applyLibrarySearch), so the
// read composes with each without a second caller-side branch.
func applyFaceDownMarker(h Host, sa *cards.SA, c *Ctx, ev *events.Event, to state.Zone) {
	faceDown := strings.EqualFold(strings.TrimSpace(sa.Params["FaceDown"]), "True")
	exileFaceDown := strings.EqualFold(strings.TrimSpace(sa.Params["ExileFaceDown"]), "True")
	switch {
	case to == state.ZExile && exileFaceDown:
		ev.Counter = "exiled_with_face_down"
		ev.Amount = int32(c.Source)
		ev.IDs = nil
	case to == state.ZExile && faceDown:
		ev.Counter = "face_down"
		ev.Amount = 0
		ev.IDs = nil
	case to == state.ZBattlefield && faceDown:
		setType := strings.TrimSpace(sa.Params["FaceDownSetType"])
		power, hasPower := NumResolved(h, c, sa, "FaceDownPower", 0)
		toughness, hasTough := NumResolved(h, c, sa, "FaceDownToughness", 0)
		hasPT := hasPower || hasTough
		ev.Counter = events.FaceDownEntryCounterFor(setType, power, toughness, hasPT)
		if ev.From == state.ZHand || ev.From == state.ZLibrary {
			ev.Secret = true
		}
	}
}

// settleChangeZoneMove is the one settle path every ChangeZone mover shares:
// the MoveZone itself, then RememberChanged$ (the moved object joins the
// ability's Remembered -- a DelayedTrigger running as a later SubAbility of
// the same chain captures it, and the value is a parameter of the ongoing
// resolution (Ctx), not game state, so mutating it here is fine), then the
// WithCountersType$/WithCountersAmount$ entry counters when the move lands on
// the battlefield. Keeping the object path and the hand-choice path on this
// one helper means the two cannot drift apart on any of the three.
func settleChangeZoneMove(h Host, c *Ctx, sa *cards.SA, id state.ObjID, from, to state.Zone, withKind string, withAmt int32) {
	settleChangeZoneMoveAs(h, c, sa, id, from, to, withKind, withAmt, 0, false)
}

// settleChangeZoneMoveAs is the one settle path every ChangeZone mover shares
// (settleChangeZoneMove is its event-Player-unset form), plus the explicit
// event-Player form: a hidden-zone move of ANOTHER player's card carries that
// player as the event's Player -- the same attribution the library search's
// move applies -- so the view layer's hidden-card redaction sees the move the
// way the owner does. The MoveZone itself, then RememberChanged$ (the moved
// object joins the ability's Remembered -- a DelayedTrigger running as a
// later SubAbility of the same chain captures it, and the value is a
// parameter of the ongoing resolution (Ctx), not game state, so mutating it
// here is fine), then the WithCountersType$/WithCountersAmount$ entry
// counters when the move lands on the battlefield. Keeping the object path
// and the hand-choice path on this one helper means the two cannot drift
// apart on any of the three. Tapped$ True is event-backed for the hidden
// library paths, but not for a card entering from hand; before every such
// move this common path makes the narrowing replay-visible rather than
// silently entering the card untapped.
//
// The exiled-with association and the RememberChanged$ event-backed rider
// (eventRemember) are NOT done here: they are scoped to the two ORIGINAL
// ChangeZone movers that carried them before this helper existed (the
// object-target loop in effChangeZone and applyLibrarySearch's hidden-search
// mover), not to every caller of this now-shared settle path -- widening
// their scope here would move acceptance-game replay hashes beyond the
// reviewed change.

// gainControlOf resolves a ChangeZone SA's GainControl$ parameter (Reanimate's
// "onto the battlefield under your control", Control Magic-family Steal
// effects' "under your control") and returns the player the moved object must
// come under the control of. Forge's ChangeZoneEffect names the gain target
// in that one parameter: "True" and "You" both mean the resolving
// controller (the corpus's 287 True lines and 43 You lines); every other
// value is a player selector resolved through the shared Defined grammar
// (ChosenPlayer, Targeted, Player.IsRemembered, ParentTarget, ...), taking
// the first resolved player deterministically. The third return reports
// whether the parameter is PRESENT at all; the second whether the value
// resolved. An unresolvable value (a spec whose resolution names no player,
// or one the Defined grammar does not know) is false and the caller is loud
// rather than silently keeping the owner -- the same fail-closed convention
// every unread parameter here follows.
func gainControlOf(h Host, c *Ctx, sa *cards.SA) (state.PlayerID, bool, bool) {
	raw, present := sa.Params["GainControl"]
	if !present || strings.TrimSpace(raw) == "" {
		return 0, false, true
	}
	if strings.EqualFold(raw, "True") || strings.EqualFold(raw, "You") {
		return c.Controller, true, true
	}
	targets, known := definedSpec(h, c, raw)
	if !known {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "unrecognised ChangeZone GainControl$ " + raw})
		return 0, false, true
	}
	for _, t := range targets {
		if t.IsPlayer {
			return t.Player, true, true
		}
		if o := h.Game().Obj(t.Obj); o != nil {
			return o.Controller, true, true
		}
	}
	// The selector resolved (its grammar is known) but named no living
	// player: nothing to hand control to, and no silent owner-keep either.
	return 0, false, true
}

// applyGainControl emits the ControlChange that hands a just-moved object to
// the GainControl$ player, after the Move has placed it. Order matters: the
// move establishes the entry (controller = owner, CR 400.7), the control
// change is the CR 701.22a "gains control" step on top, and replay folds the
// two events in the same order live does.
func applyGainControl(h Host, c *Ctx, sa *cards.SA, id state.ObjID) {
	p, ok, present := gainControlOf(h, c, sa)
	if !present || !ok {
		return
	}
	if o := h.Game().Obj(id); o != nil && o.Controller != p {
		h.Emit(events.Event{Kind: events.ControlChange, Obj: id, Player: p,
			Text: "GainControl"})
	}
}

// applyTransformed implements ChangeZone's Transformed$ True: a double-faced
// card this effect moves to the battlefield enters TRANSFORMED (CR 711.10a:
// a transforming double-faced card enters with its back face up when an
// effect says so) — the Ojer Axonil death trigger's "return it to the
// battlefield tapped and transformed", the Kytheon/Kumano "return it
// transformed" returns. The flip is the one FlipFace event effSetState
// emits, to the face AFTER the one the card carries out of its zone, so
// replay folds move-then-flip in the same order live does. A card with fewer
// than two faces is not a transform and is left alone.
func applyTransformed(h Host, c *Ctx, sa *cards.SA, id state.ObjID) {
	if !strings.EqualFold(strings.TrimSpace(sa.Params["Transformed"]), "True") {
		return
	}
	o := h.Game().Obj(id)
	if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
		return
	}
	next := (int(o.FaceIdx) + 1) % len(o.Card.Faces)
	h.Emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: int32(next), Text: "Transformed"})
}

func settleChangeZoneMoveAs(h Host, c *Ctx, sa *cards.SA, id state.ObjID, from, to state.Zone, withKind string, withAmt int32, player state.PlayerID, hasPlayer bool) {
	if from == state.ZHand && to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
		notePlayer := c.Controller
		if hasPlayer {
			notePlayer = player
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: notePlayer,
			Text: "Tapped$ True on a hand ChangeZone is not implemented; the card enters untapped"})
	}
	ev := moveZoneEvent(c, id, from, to)
	if to == state.ZExile && len(ev.IDs) == 0 && (faceStaticsNameExiledWithSource(h, c.Source) || strings.EqualFold(strings.TrimSpace(sa.Params["Imprint"]), "True")) {
		// The S: static spelling of the same provenance need: a source whose
		// own Static lines name ExiledWithSource (Intellect Devourer's
		// MayPlay+ExiledWithSource grant) tracks its exiles exactly like the
		// SVar shapes exileProvenanceNeeded covers; Imprint$ True is the
		// Chrome Mox spelling, feeding the Defined.Imprinted reflected-mana
		// selector. Extra IDs on an exile move are inert for every consumer
		// that never reads them.
		ev.IDs = []state.ObjID{c.Source}
	}
	if hasPlayer {
		ev.Player = player
	}
	applyFaceDownMarker(h, sa, c, &ev, to)
	h.Emit(ev)
	if to == state.ZExile {
		recordExileReturn(h, c, sa, id, from, to)
	}
	// RememberLKI$ True (the corpus's 77 ChangeZone lines -- Reanimate's
	// "creature card" whose mana value the chained lose-life SVar reads,
	// RememberedLKI$CardManaCost) joins the moved object to the ability's
	// Remembered. Same Ctx binding RememberChanged$ uses: a resolution-local
	// value, replayed identically because replay re-runs the same SA. The two
	// flags stack; an object is not remembered twice. RememberLKI$
	// Targeted (2 lines, a different capture point -- the CHOSEN target, not
	// the moved object) is left to its own work and is not silently folded
	// into this read.
	if strings.EqualFold(sa.Params["RememberLKI"], "True") &&
		!strings.EqualFold(sa.Params["RememberChanged"], "True") {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
	}
	if strings.EqualFold(sa.Params["RememberChanged"], "True") {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
	}
	if withKind != "" && to == state.ZBattlefield {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: withKind, Amount: withAmt})
	}
	// GainControl$ hands the moved object to the named player. Only a
	// battlefield entry can carry a control change (CR 701.22a controls
	// permanents); a card moved to a hidden or public non-battlefield zone
	// keeps its owner.
	if to == state.ZBattlefield {
		applyGainControl(h, c, sa, id)
		applyTransformed(h, c, sa, id)
		// StaticEffect$ <name> (the "return it ... It's a Spirit Detective"
		// rider): the named Continuous static registers onto the moved card
		// once its move and entry riders are settled. A no-op on every SA
		// without the parameter.
		applyStaticEffect(h, c, sa, to, []state.ObjID{id})
	}
}

// exiledWithAssociation emits Forge's ChangeZoneEffect.handleExiledWith
// association for a non-token card this effect just exiled: the host's
// distinct exiledCards collection. It is deliberately NOT an ImprintCards$
// association: DefinedCards$ ExiledWith consumes this list, while
// ImprintedController only consumes explicit ImprintCards$ entries. Scoped to
// the object-target loop and applyLibrarySearch, the two movers that carried
// this association originally.
func exiledWithAssociation(h Host, c *Ctx, id state.ObjID, to state.Zone) {
	if to != state.ZExile || c.Source == 0 {
		return
	}
	if o := h.Game().Obj(id); o != nil && !o.IsToken {
		h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: []state.ObjID{id}, Text: "exiled-with"})
	}
}

// recordExileReturn reads ChangeZone's Duration$ parameter. The corpus's
// whole ChangeZone Duration$ population (118 raw .cards/cardsfolder lines:
// 111 Origin$ Battlefield, 6 Hand, 1 Graveyard, plus 7 on ChangeZoneAll)
// carries the single value UntilHostLeavesPlay -- the Oblivion Ring /
// Banisher Priest pattern, "exile ... until CARDNAME leaves the battlefield".
// For it, the exiled object joins the source's event-backed ExileReturn
// association carrying the zone it was exiled from; when the source leaves
// the battlefield the rules sweep (Engine.sweepExileReturn) returns every
// object still in exile to that zone under its owner's control, which is
// Forge's own return semantic for the duration (a battlefield-origin exile
// comes back to the battlefield, a hand-origin one to the hand). A card is
// only recorded when the move actually landed in exile, and a token is never
// recorded: a token that left the battlefield has ceased to exist (CR 111.7)
// and must not come back. Any other Duration$ value is loud (a Note) rather
// than silently inert, the convention every unread parameter here follows.
func recordExileReturn(h Host, c *Ctx, sa *cards.SA, id state.ObjID, from, to state.Zone) {
	raw, present := sa.Params["Duration"]
	if !present || strings.TrimSpace(raw) == "" {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(raw), "UntilHostLeavesPlay") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "unmodelled ChangeZone Duration$ " + strings.TrimSpace(raw)})
		return
	}
	if to != state.ZExile || c.Source == 0 {
		return
	}
	if o := h.Game().Obj(id); o == nil || o.Zone != state.ZExile || o.IsToken {
		return
	}
	h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: []state.ObjID{id},
		Amount: int32(from), Text: "until-host-leaves"})
}

// handChangeNum reads the SA's ChangeNum$ as a plain integer literal
// (absent = 1, Forge's ChangeZoneEffect default for this shape). The second
// return is false for anything else -- a non-integer, negative, or value
// outside a decision count's signed 32-bit range -- and the caller routes
// that SA to the pre-existing object path
// instead: evaluating SVar/Count$ count expressions here is a scoped-out
// follow-up, not part of handmove1.
func handChangeNum(sa *cards.SA) (int32, bool) {
	v, present := sa.Params["ChangeNum"]
	if !present || v == "" {
		return 1, true
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n < 0 {
		return 0, false
	}
	return int32(n), true
}

// effChangeZoneHand is the whole-hand shape (handmove1/rv2b r1): Origin$ Hand
// with no player selector -- the resolving controller's own hand is the one
// owner, and the controller is its own chooser (Brainstorm, Jace the Mind
// Sculptor's [0], Sawtooth Loon, Burgeoning). The mechanics -- the ask gate
// (the dig1/effDiscard strict-supersets rule), explicit markers plus real
// card/script text for markerless optionality (never an assumed "may"), the
// fx42 re-entry scoping, the R-9 stand-in, the Destination$ Library placement
// (absent LibraryPosition$ = TOP in answer order; Shuffle$ True randomises instead)
// and the unread-parameter list -- are handMoveOwnersWalk's, which this
// delegates to with the one-owner, chooser==owner, no-random configuration.
// Only a literal ChangeNum$ (or its absent default 1) reaches here: the
// routing in effChangeZone Notes a non-literal before this is ever called.
func effChangeZoneHand(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	count, _ := handMoveCountOf(h, c, sa)
	handMoveOwnersWalk(h, c, sa, to, []state.PlayerID{c.Controller}, count, false, nil, false)
}

// handMoveCount is the ChangeNum$ bound one hidden-hand walk carries: either
// a fixed bound resolved once for the whole resolution, or the per-owner
// eligible count (ChangeNum$ NumInHand / HandSize -- Forge's "all matching
// cards in that hand" texts: Eradicate, Extirpate, Kotose, Lost Legacy, The
// Great Aurora).
type handMoveCount struct {
	fixed    int32
	perOwner bool
}

// handMoveCountOf classifies a hidden-hand walk's ChangeNum$. Absent reads as
// 1 (Forge's ChangeZoneEffect default). "NumInHand"/"HandSize" are the
// per-owner spellings. A plain integer literal is that literal. An SVar-named
// count (ChangeNum$ X / Y over an SVar: body) or a bare "X" (the paid X,
// CR 107.3i) resolves through the ordinary count evaluator, bound to the
// resolving context. Anything else returns false and the caller is loud (a
// Note) rather than degrading to a silent zero-count no-op.
func handMoveCountOf(h Host, c *Ctx, sa *cards.SA) (handMoveCount, bool) {
	raw := strings.TrimSpace(sa.Params["ChangeNum"])
	if raw == "" {
		return handMoveCount{fixed: 1}, true
	}
	if strings.EqualFold(raw, "NumInHand") || strings.EqualFold(raw, "HandSize") {
		return handMoveCount{perOwner: true}, true
	}
	if n, ok := handChangeNum(sa); ok {
		return handMoveCount{fixed: n}, true
	}
	resolvable := raw == "X" || strings.HasPrefix(raw, "Count$") ||
		strings.HasPrefix(raw, "Sacrificed$") || strings.HasPrefix(raw, "TriggerCount$") ||
		strings.HasPrefix(raw, "TriggerCountMax$")
	if c != nil && c.SVars != nil {
		if _, exists := c.SVars[raw]; exists {
			resolvable = true
		}
	}
	if !resolvable {
		return handMoveCount{}, false
	}
	n := Num(h, c, sa, "ChangeNum", 1)
	if n < 0 {
		n = 0
	}
	return handMoveCount{fixed: n}, true
}

// effChangeZoneHandOwners implements the owner-SELECTED hidden-hand shape
// (rv2b r2): Origin$ Hand with DefinedPlayer$-alone (Kynaios and Tiro's "each
// player may put a land card from their hand onto the battlefield", Braids,
// Conjurer Adept, Mindleech Ghoul) or ValidTgts$-alone naming the players
// whose hand moves (Karn Liberated's "[+4]: Target player exiles a card from
// their hand", Kyoki, Sanity's Eclipse). One chooser ask per hand owner,
// chained across owners through the persisted Ctx.HandMoveTarget cursor --
// the walk restarts on every answer, skips the owners already answered, and
// asks the next one -- exactly effDig's per-target continuation, but with a
// REAL ask for every later owner rather than a deterministic stand-in (the
// hand owners are few and each ask is short). Whose hand and who answers are
// the two selectors this shape carries: the owners come from
// DefinedPlayer$/ValidTgts$, the chooser from Chooser$ (Forge's default is
// the hand owner). Every shape this function cannot model emits a Note and
// moves nothing -- the finding's floor: never a silent no-op.
func effChangeZoneHandOwners(h Host, c *Ctx, sa *cards.SA, to state.Zone) {
	owners, ok := handMoveOwners(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "cannot resolve the hand owner (DefinedPlayer$ " + strings.TrimSpace(sa.Params["DefinedPlayer"]) +
				"); no hand card moves"})
		return
	}
	if len(owners) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "the hand owner selector names no player this engine can resolve; no hand card moves"})
		return
	}
	count, ok := handMoveCountOf(h, c, sa)
	if !ok {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "cannot choose ChangeNum$ " + strings.TrimSpace(sa.Params["ChangeNum"]) +
				" cards from a selected hand (a count this engine cannot evaluate)"})
		return
	}
	choosers := make([]state.PlayerID, len(owners))
	for i, owner := range owners {
		ch, ok := handMoveChooserFor(h, c, sa, owner)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "Chooser$ " + strings.TrimSpace(sa.Params["Chooser"]) +
					" is not a chooser this engine can resolve; no hand card moves"})
			return
		}
		choosers[i] = ch
	}
	random := strings.EqualFold(strings.TrimSpace(sa.Params["AtRandom"]), "True")
	handMoveOwnersWalk(h, c, sa, to, owners, count, random, func(_ Host, _ *Ctx, _ *cards.SA, owner state.PlayerID) (state.PlayerID, bool) {
		return choosers[ownerIndex(owners, owner)], true
	}, true)
}

// ownerIndex is the position of owner in owners (owners is small and built
// without duplicates).
func ownerIndex(owners []state.PlayerID, owner state.PlayerID) int {
	for i, p := range owners {
		if p == owner {
			return i
		}
	}
	return 0
}

// handMoveOwners resolves whose hands an owner-selected hidden-hand ChangeZone
// moves from. DefinedPlayer$ takes precedence and resolves through the same
// deterministic selector grammar the library search uses (searchPlayers); a
// ValidTgts$-alone line's chosen targets are the hand owners. It fails
// CLOSED: a player spec this build does not model returns ok=false and the
// caller emits its loud Note -- degrading an unmodelled selector to the
// resolving controller's hand would move (and reveal) cards from the WRONG
// player's hidden hand, which is worse than moving none.
func handMoveOwners(h Host, c *Ctx, sa *cards.SA) ([]state.PlayerID, bool) {
	if spec := strings.TrimSpace(sa.Params["DefinedPlayer"]); spec != "" {
		if _, modelled := definedSpec(h, c, spec); !modelled {
			return nil, false
		}
		return searchPlayers(h, c, sa), true
	}
	// ValidTgts$-alone: Defined's own rule names the chosen targets.
	owners := make([]state.PlayerID, 0, len(c.Targets))
	seen := make(map[state.PlayerID]bool, len(c.Targets))
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer {
			// A hand-ownership selector that resolved to a card is not a shape
			// this chooser can honour.
			return nil, false
		}
		p := PlayerOf(h, c, t)
		if int(p) >= len(h.Game().Players) || seen[p] {
			continue
		}
		seen[p] = true
		owners = append(owners, p)
	}
	return owners, true
}

// handMoveChooserFor resolves who answers one owner's hidden-hand ask.
// Forge's default for the shape is the hand owner (Kynaios and Tiro's "each
// player may put", Mindleech Ghoul's "defending player exiles a card from
// their hand"); Chooser$ You is the caster picking out of another player's
// hand (Kitesail Freebooter, Witness the End), Chooser$ Targeted the chosen
// target (Karn Liberated), and the TriggeredTarget/TriggeredPlayer spellings
// the causing event's bound player (Kheru Mind Eater, Widespread Panic). An
// unmodelled value fails closed (ok=false) so the caller is loud rather than
// handing the ask to a guessed seat.
func handMoveChooserFor(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID) (state.PlayerID, bool) {
	switch strings.TrimSpace(sa.Params["Chooser"]) {
	case "", "Owner":
		return owner, true
	case "You":
		return c.Controller, true
	case "Targeted":
		if len(c.Targets) > 0 {
			return PlayerOf(h, c, c.Targets[0]), true
		}
		return owner, true
	case "TriggeredTarget":
		if c.TriggerTarget.IsPlayer {
			return c.TriggerTarget.Player, true
		}
		if c.TriggerTarget.Obj != 0 {
			if o := h.Game().Obj(c.TriggerTarget.Obj); o != nil {
				return o.Controller, true
			}
		}
		return owner, true
	case "TriggeredPlayer":
		if c.TriggerPlayer.IsPlayer {
			return c.TriggerPlayer.Player, true
		}
		return owner, true
	}
	return owner, false
}

// handMoveOwnersWalk is the ONE hidden-hand mover both shapes share: for each
// owner in order the eligible pool is that owner's hand filtered through
// ChangeType$ (absent: the whole hand), the bound is the count (perOwner:
// that pool's own size), and the pick is one of three shapes -- the chained
// ask (STRICTLY more eligible cards than the bound: a real KChoose to the
// chooser, Min 0 when the take is optional else the bound, Max the bound, the
// answer re-entering through ResumeKind "hand_move" with ResumeTarget
// binding it to this owner), the no-choice deterministic take (a REQUIRED
// move with eligible <= bound: every eligible card moves, in hand order, no
// ask), or AtRandom$'s engine-random pick (no ask: randomness, not a player
// choice, picks). An OPTIONAL move takes the choice path whenever there is
// at least one eligible card, including eligible <= bound: declining remains
// a meaningful answer even when taking every card is the only nonempty pick
// (an empty-only ChangeNum$ 0 still resolves through AskEmpty). The re-entry
// contract (fx42 scoping): Ctx.HandMove/HandMoveDone/HandMoveTarget are captured and
// cleared at the top of the walk; owners before the cursor completed before a
// later owner suspended and are skipped, the cursor's owner consumes the
// answer (moved exactly as answered, revalidated against the CURRENT hand
// and filter), and owners after it continue the chain.
//
// The whole-hand shape (handmove1/rv2b r1) is the one-owner case of this
// walk, with the chooser == the owner -- the r1 contracts (ask shape, card
// text/explicit-marker optionality, R-9 stand-in text, zero-eligible silence,
// library tail) are this walk's contracts, unchanged. Still unread here, each
// a scoped-out follow-up:
// Destination$ Hand/Sideboard oddities (2 lines), and any ConditionPresent$/
// ConditionDefined$ gate (the engine-wide Condition* gap).
func handMoveOwnersWalk(h Host, c *Ctx, sa *cards.SA, to state.Zone, owners []state.PlayerID,
	count handMoveCount, random bool, chooserFor func(Host, *Ctx, *cards.SA, state.PlayerID) (state.PlayerID, bool),
	eventPlayer bool) {
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card" // the whole hand: Brainstorm, Jace's [0], Sawtooth Loon
	}
	g := h.Game()
	// fx42 scoping: capture and clear the answered pick (and the cursor that
	// binds it to the owner that asked) BEFORE anything else, so a nested
	// hand-move ask below cannot inherit them.
	ans := c.HandMove
	done := c.HandMoveDone
	cursor := c.HandMoveTarget
	c.HandMove, c.HandMoveDone, c.HandMoveTarget = nil, false, 0
	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	// Match the object path: WithCounters* has no effect away from the
	// battlefield, and parsing a dynamic amount there must not emit a Note.
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	// settleHandMove settles one chosen card: exactly the shared ChangeZone
	// mover; on the owner-SELECTED shapes the Move event also carries the
	// hand's OWNER as its Player (a hidden-zone move of another player's
	// card -- the same attribution the library search's move carries),
	// while the whole-hand shape keeps its historical event shape
	// (eventPlayer false, the r1 golden contract). settleChangeZoneMoveAs is
	// also the one loud Tapped$ True fallback for every hand-origin mover, so
	// concrete Defined$ objects and future hand-owner selectors cannot silently
	// miss the unsupported entry state.
	settleHandMove := func(id state.ObjID, owner state.PlayerID) {
		settleChangeZoneMoveAs(h, c, sa, id, state.ZHand, to, withKind, withAmt, owner, eventPlayer)
	}
	for i, owner := range owners {
		hand := zoneOf(g, state.ZHand, owner)
		eligible := make([]state.ObjID, 0, len(hand))
		for _, id := range hand {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
			}
		}
		if done && i < cursor {
			// This owner answered on an earlier pass, before a later owner
			// suspended the walk. Re-running it could move a second batch, so
			// skip it (effDig's per-target continuation contract).
			continue
		}
		if done && i == cursor {
			// Re-entry: move exactly the answered cards that still sit in THIS
			// owner's hand and still match the filter (a stray answer must not
			// move an object that left the hand meanwhile), in the player's
			// answer order.
			var moved []state.ObjID
			for _, id := range ans {
				if !containsID(hand, id) {
					continue
				}
				o := g.Obj(id)
				if o == nil || o.Zone != state.ZHand {
					continue
				}
				if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
					continue
				}
				settleHandMove(id, owner)
				moved = append(moved, id)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			// AtEOT$ on the hand walk: the owner's answered batch is the
			// affected set, scheduled per owner BEFORE the walk can suspend on a
			// later owner's ask (a suspension must not lose this batch's
			// registrations -- the re-entry skips already-answered owners and
			// never re-schedules them).
			scheduleAtEOT(h, c, sa, moved)
			continue
		}
		n := count.fixed
		if count.perOwner {
			n = int32(len(eligible))
		}
		if len(eligible) == 0 || n == 0 {
			// No eligible card, or an empty-only ChangeNum$ 0 choice: both
			// complete silently before optionality can matter (AskEmpty's
			// shared contract).
			continue
		}
		var moved []state.ObjID
		if random {
			// AtRandom$ True: the engine picks, not a player -- ChangeNum$
			// random distinct eligible cards (corpus: always 1, mandatory),
			// through the seeded generator, so the pick replays.
			pool := append([]state.ObjID(nil), eligible...)
			for k := int32(0); k < n && len(pool) > 0; k++ {
				j := h.Rand(len(pool))
				settleHandMove(pool[j], owner)
				moved = append(moved, pool[j])
				pool = append(pool[:j], pool[j+1:]...)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			scheduleAtEOT(h, c, sa, moved)
			continue
		}
		// NumInHand/HandSize means "all matching cards in that hand", an
		// intrinsically required all-cards move (Eradicate, Extirpate, The
		// Great Aurora). Its count semantics settle optionality even when the
		// script has neither marker nor explanatory text; preserve an explicit
		// Optional$ marker should a future script carry one.
		intrinsicAll := count.perOwner && strings.TrimSpace(sa.Params["Optional"]) == "" && strings.TrimSpace(sa.Params["Mandatory"]) == ""
		optional, optionalKnown := handTakeOptional(h, c, sa, to)
		if intrinsicAll {
			optional, optionalKnown = false, true
		}
		if !optionalKnown {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "cannot determine whether the markerless hand move is optional; no hand card moves"})
			return
		}
		if int32(len(eligible)) <= n && !optional {
			// A required move with no possible nonempty selection alternative
			// takes every eligible card deterministically. An OPTIONAL move
			// must still ask here: declining is a distinct, legal answer even
			// when every nonempty answer takes all eligible cards.
			for _, id := range eligible {
				settleHandMove(id, owner)
				moved = append(moved, id)
			}
			handLibraryTail(h, g, sa, c.Source, owner, moved, to)
			scheduleAtEOT(h, c, sa, moved)
			continue
		}
		chooser := owner
		if chooserFor != nil {
			chooser, _ = chooserFor(h, c, sa, owner)
		}
		min := int(n)
		if optional {
			min = 0 // "you may put": none is a legal answer
		}
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
			Min: min, Max: int(n), Source: c.Source,
			ResumeKind: "hand_move", ResumeSA: sa, ResumeTarget: i,
			// The re-entered walk revalidates the answered cards against the
			// SAME filter it offered them under (Card.IsRemembered in Vizkopa
			// Confessor's PickOne, whose remembered population is ctx-level
			// only -- RememberRevealed$), so the ask must RIDE that set the way
			// every other mid-resolution ask boundary does (attach.go,
			// counters.go, play.go): without it the rebuild loses the ctx-level
			// Remembered and the revalidation re-eligible-matches nothing.
			ResumeRemembered: copyTargets(c.Remembered),
			Prompt:           handMovePromptFor(sa, to, int(n), chooser == owner)}
		for _, id := range eligible {
			name := "a card"
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "hand_move", Label: name, Obj: id, Player: owner})
		}
		// The shared ask boundary (effects.Ask): a ChangeNum$ 0 pick over a
		// nonempty eligible hand is Min == Max == 0 -- the empty-answer-only
		// shape -- so it is never posted; AskEmpty resolves silently through
		// the stand-in below, which moves zero cards.
		oc := Ask(h, d)
		if oc == AskAsked {
			return // resolution suspended; the answer re-enters with Ctx.HandMove set.
		}
		// R-9: a host without a decision channel cannot ask a player, so it
		// supplies the deterministic answer in the player's place -- the first
		// ChangeNum eligible cards in the same ordered eligible list the
		// decision's options were built from.
		if oc == AskNoHost {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "moves the first matching card(s) from hand (no engine host to ask)"})
		}
		for k := int32(0); k < n && int(k) < len(eligible); k++ {
			settleHandMove(eligible[k], owner)
			moved = append(moved, eligible[k])
		}
		handLibraryTail(h, g, sa, c.Source, owner, moved, to)
		scheduleAtEOT(h, c, sa, moved)
	}
}

// handTakeOptional reads Forge's optional-vs-mandatory markers for a
// hidden-origin hand move. A missing marker is NOT an optional default:
// Volrath's Dungeon is markerless but requires its target to put a card back.
// Forge does not encode that distinction in ChangeZone's parameters, so the
// markerless may-shapes are recognised from their card/script text (Burgeoning,
// Oviya, Volcanic Spite); text we cannot classify fails closed and loudly at
// the caller rather than granting an invented decline.
func handTakeOptional(h Host, c *Ctx, sa *cards.SA, to state.Zone) (optional, known bool) {
	if strings.EqualFold(strings.TrimSpace(sa.Params["Mandatory"]), "True") {
		return false, true
	}
	if o := strings.TrimSpace(sa.Params["Optional"]); o != "" {
		return strings.EqualFold(o, "True") || strings.EqualFold(o, "You"), true
	}
	text := sa.Params["SpellDescription"]
	if o := h.Game().Obj(c.Source); o != nil && o.Face() != nil {
		text += "\n" + o.Face().Oracle
	}
	if strings.TrimSpace(text) == "" {
		return false, false
	}
	return handMoveTextOptional(text, to)
}

// handMoveTextOptional recognises the actual English may-forms for the one
// ChangeZone move being resolved. "Put any number" is optional even without
// the word may. It requires the destination's action verb and a hand
// reference IN THE SAME sentence: an unrelated "may put" elsewhere on a
// multi-ability card is not evidence that this move may be declined.
// Conversely, text that lacks a matching action is unknown, so the caller
// emits its fail-closed Note.
func handMoveTextOptional(text string, to state.Zone) (optional, known bool) {
	text = strings.ToLower(text)
	var action string
	switch to {
	case state.ZBattlefield, state.ZLibrary:
		action = "put"
	case state.ZExile:
		action = "exile"
	case state.ZGraveyard:
		action = "discard"
	case state.ZHand:
		action = "return"
	default:
		return false, false
	}
	if handMovePhraseMentionsHand(text, "may "+action) ||
		handMovePhraseFollowsHandReveal(text, "may "+action) ||
		(to == state.ZLibrary && handMovePhraseMentionsHand(text, "may shuffle")) ||
		(handMovePhraseMentionsHand(text, "any number") && handMovePhraseMentionsHand(text, action)) {
		return true, true
	}
	if handMovePhraseSupportsRequiredMove(text, action) ||
		handMovePhraseFollowsHandChoice(text, action) ||
		(to == state.ZLibrary && handMovePhraseSupportsRequiredMove(text, "shuffle")) {
		return false, true
	}
	return false, false
}

// handMovePhraseSupportsRequiredMove also accepts "choose" in the action's
// sentence: a preceding RevealHand can make the later "You choose ... and
// exile that card" sentence omit the word hand (Thought-Knot Seer), but it is
// still an unambiguously required chooser action.
func handMovePhraseSupportsRequiredMove(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[begin:end], "hand") || strings.Contains(text[begin:end], "choose") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseFollowsHandChoice recognises the same hidden-hand sequence
// when Forge split its selection and movement into sentences: "reveal their
// hand. You choose a card from it. Exile that card" (Kitesail Freebooter).
// It scans only the action sentence and its three predecessors, all of which
// must establish the hand -> choice -> pronoun chain.
func handMovePhraseFollowsHandChoice(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		contextStart := begin
		for n := 0; n < 3 && contextStart > 0; n++ {
			prior := strings.TrimRight(text[:contextStart], ".; ")
			if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
				contextStart = j + 1
			} else {
				contextStart = 0
			}
		}
		context := text[contextStart:end]
		if (strings.Contains(text[begin:end], "that card") || strings.Contains(text[begin:end], " it")) &&
			strings.Contains(context, "hand") && strings.Contains(context, "choose") && strings.Contains(context, "it") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseFollowsHandReveal recognises the usual two-sentence hidden
// hand wording: "Target player reveals their hand. You may put ... from it."
// The pronoun is enough only immediately after a hand-reveal sentence, so an
// unrelated optional action elsewhere cannot make this move optional.
func handMovePhraseFollowsHandReveal(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		begin := 0
		if j := strings.LastIndexAny(text[:i], ".;"); j >= 0 {
			begin = j + 1
		}
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		prior := strings.TrimRight(text[:begin], ".; ")
		if j := strings.LastIndexAny(prior, ".;"); j >= 0 {
			prior = prior[j+1:]
		}
		if strings.Contains(prior, "hand") && strings.Contains(text[begin:end], "it") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePhraseMentionsHand keeps text classification local to the sentence
// carrying a candidate action. Forge's Oracle text uses periods for sentence
// boundaries; semicolons also separate instructions often enough to be a safe
// boundary here.
func handMovePhraseMentionsHand(text, phrase string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], phrase)
		if i < 0 {
			return false
		}
		i += start
		end := len(text)
		if j := strings.IndexAny(text[i:], ".;"); j >= 0 {
			end = i + j
		}
		if strings.Contains(text[i:end], "hand") {
			return true
		}
		start = i + len(phrase)
	}
}

// handMovePrompt builds the human-readable ask text, naming the top/bottom
// placement when the destination is the library (the one destination where
// WHERE matters to the chooser), and whose hand it is when the chooser is
// not the hand's owner (Chooser$ You: the caster picks out of another
// player's hand).
func handMovePromptFor(sa *cards.SA, to state.Zone, n int, own bool) string {
	dest := handDestPhrase(to)
	if to == state.ZLibrary {
		if strings.TrimSpace(sa.Params["LibraryPosition"]) == "-1" {
			dest = "the bottom of your library"
		} else {
			dest = "the top of your library"
		}
	}
	whose := "your hand"
	if !own {
		whose = "that player's hand"
		if to == state.ZLibrary {
			if strings.TrimSpace(sa.Params["LibraryPosition"]) == "-1" {
				dest = "the bottom of that player's library"
			} else {
				dest = "the top of that player's library"
			}
		}
	}
	return "Choose " + strconv.Itoa(n) + " card(s) from " + whose + ": they move to " + dest
}

// handLibraryTail is the post-move library placement both hidden-origin
// movers end with. A hand put-back only shuffles when its own script says
// so (Shuffle$ True -- Slowtrip), while a library search shuffles by
// default; the placement itself is the shared libraryOrderPlacement helper,
// with Forge's absent-LibraryPosition$ default (TOP) applied for the hand
// path -- Brainstorm and Jace's [0] name no LibraryPosition$ and their
// oracle puts the cards on top.
func handLibraryTail(h Host, _ *state.Game, sa *cards.SA, source state.ObjID, owner state.PlayerID, moved []state.ObjID, to state.Zone) {
	if to != state.ZLibrary || len(moved) == 0 {
		return
	}
	// DestinationAlternative$/LibraryPositionAlternative$ (Dream Cache's "both
	// on top of your library or both on the bottom", 1 raw line) is a modal
	// destination choice this engine cannot yet ask: the alternative is named
	// in a Note and the primary destination/position is taken
	// deterministically, so the unsupported shape is never silent.
	if alt := strings.TrimSpace(sa.Params["DestinationAlternative"]); alt != "" || strings.TrimSpace(sa.Params["LibraryPositionAlternative"]) != "" {
		h.Emit(events.Event{Kind: events.Note, Obj: source, Player: owner,
			Text: "DestinationAlternative$ " + alt + " is not a choice this engine can ask; the cards take the primary destination"})
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Shuffle"]), "True") &&
		!strings.EqualFold(strings.TrimSpace(sa.Params["NoShuffle"]), "True") {
		shuffleLibraryExplicit(h, sa, owner)
		return // a shuffled library has no meaningful LibraryPosition$
	}
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	if position != "" && position != "0" && position != "-1" {
		h.Emit(events.Event{Kind: events.Note, Obj: source, Player: owner,
			Text: "LibraryPosition$ " + position + " is not implemented; the cards go on top"})
	}
	libraryOrderPlacement(h, owner, moved, position == "-1")
}

// handDestPhrase names the hand-move destination in the human-readable
// prompt; its own vocabulary so it cannot drift into digDestPhrase's or
// destinationPhrase's.
func handDestPhrase(to state.Zone) string {
	switch to {
	case state.ZBattlefield:
		return "the battlefield"
	case state.ZGraveyard:
		return "the graveyard"
	case state.ZExile:
		return "exile"
	case state.ZLibrary:
		return "the library"
	case state.ZHand:
		return "the hand"
	default:
		return "its destination"
	}
}

// withCounterAmount parses WithCountersAmount$ (default 1). Malformed values
// must be loud, not silently default to 1 (the reviewer's item): a wrong
// counter count on a Returning permanent is a hard-to-spot board-shape bug. A
// Note event (the way Resolve surfaces an unimplemented API) keeps this
// deterministic and replay-log-visible rather than dropping to a log line the
// event log cannot account for. The movement still proceeds with the safe
// default 1.
func withCounterAmount(h Host, c *Ctx, sa *cards.SA) int32 {
	v := strings.TrimSpace(sa.Params["WithCountersAmount"])
	if v == "" {
		return 1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "malformed WithCountersAmount " + v})
		return 1
	}
	return int32(n)
}

// effSearchLibrary implements the hidden-origin ChangeZone shape. The option
// list is rebuilt deterministically from library order and ChangeType$, while
// the answer is carried only as option indices and object ids through the
// ordinary KChoose/resume mechanism.
//
// zones is the full origin set (Origin$ merged with OriginAlternative$):
// Library is the hidden half, and any public zones in the set (Graveyard,
// Exile, Hand) contribute their owner's matching cards to the SAME option
// list, exactly Forge's choose-a-card-from-any-of-these-zones step. The
// library is searched first in candidate order so a pure-library search's
// option list -- and therefore its chain heads -- is unchanged.
//
// This round deliberately handles only the first resolved library when
// DefinedPlayer$/Defined$ names several players. A single effect cannot yet
// persist its place in a multi-player loop across more than one suspended ask;
// restarting the primitive would otherwise re-ask the first library. The
// narrowing and its measured corpus population are recorded in AGENTS.md.
func effSearchLibrary(h Host, c *Ctx, sa *cards.SA, to state.Zone, zones []state.Zone) {
	players := searchPlayers(h, c, sa)
	if len(players) == 0 {
		return
	}
	owner := players[0]
	g := h.Game()
	lib := zoneOf(g, state.ZLibrary, owner)

	// A ShuffleNonMandatory$ search's may-shuffle confirm was answered: the
	// moves already happened in the first pass, so this pass is tail-only --
	// the answered shuffle (or the kept order) and the LibraryPosition$
	// placement. searchShuffleTail consumes and clears the pair before
	// continuing (fx42 scoping), so a nested search poses its own confirm.
	if c.SearchShuffle != "" {
		searchShuffleTail(h, c, sa, owner, nil, to)
		return
	}

	if c.SearchDone {
		chosen := append([]state.ObjID(nil), c.Search...)
		// Scope the answer to this primitive. Any asking primitive reached by
		// the SubAbility chain must pose its own decision.
		c.Search, c.SearchDone = nil, false
		applyLibrarySearch(h, c, sa, owner, to, chosen, zones)
		return
	}

	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	// Candidate order: the library first (in library order), then each public
	// origin zone in the order given by the parsed origin set. Dedupe across
	// zones so a card can never be offered twice. `eligible` is the ordered
	// list both the decision options and the R-9 stand-in read, so its order
	// is load-bearing for determinism.
	eligible := make([]state.ObjID, 0, len(lib))
	seen := make(map[state.ObjID]bool, len(lib))
	for _, id := range lib {
		if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			eligible = append(eligible, id)
			seen[id] = true
		}
	}
	for _, z := range zones {
		if z == state.ZLibrary {
			continue
		}
		for _, id := range zoneOf(g, z, owner) {
			if seen[id] {
				continue
			}
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				eligible = append(eligible, id)
				seen[id] = true
			}
		}
	}
	// WithTotalCMC$ is the cumulative mana-value budget over the found cards
	// (Protean Hulk: "any number of creature cards with total mana value 6 or
	// less"), the exact parameter effDig reads on its own window. A card
	// whose own mana value exceeds the budget can never be found, and the
	// running sum of the picks must not exceed it either; the mechanics
	// mirror effDig's (affordable filter, Decision.MaxSum + Option.Value on
	// the wire, a greedy stand-in). Absent the param the budget is 0,
	// budgetEligible == eligible and every read below is a no-op, so a
	// non-budget search emits byte-identically. The CR 701.23b/701.23d Min
	// semantics below are unchanged; only the affordable pool they are read
	// over is narrowed.
	budget, hasBudget := NumResolved(h, c, sa, "WithTotalCMC", 0)
	if budget < 0 {
		budget = 0
	}
	budgetEligible := eligible
	if hasBudget {
		budgetEligible = make([]state.ObjID, 0, len(eligible))
		for _, id := range eligible {
			if manaValueOf(g, id) <= int(budget) {
				budgetEligible = append(budgetEligible, id)
			}
		}
	}
	max := Num(h, c, sa, "ChangeNum", 1)
	if max < 0 {
		max = 0
	}
	if max > int32(len(budgetEligible)) {
		max = int32(len(budgetEligible))
	}
	// CR 701.23b/701.23d decide the minimum: a search whose card filter states
	// only a quantity must find that many (or as many as the zone holds), so
	// Min is forced up to Max; a stated-quality search keeps the fail-to-find
	// allowance of Min 0. `max` is already clamped to the number of eligible
	// cards, so a quantity-only search never asks for more than the library
	// holds (701.23d's "as many as possible"). This is a property of the
	// filter, not of Forge's Mandatory$ parameter.
	min := int32(0)
	if !SearchStatesQuality(spec) {
		min = max
	}
	// An empty choice is not a choice: asking it suspends a real engine host
	// until it submits an empty answer, even though no answer can differ.
	// Complete the fail-to-find directly (including its required shuffle).
	if min == 0 && max == 0 {
		// A submitted search answer resumes in a fresh Ctx, so remembered
		// objects do not leak into its SubAbility chain. Preserve that existing
		// continuation contract while omitting the otherwise meaningless ask.
		c.Remembered = nil
		applyLibrarySearch(h, c, sa, owner, to, nil, zones)
		return
	}
	// greedy is the deterministic stand-in take under the cumulative budget
	// (bound by the ChangeNum cap): with no budget every card fits and greedy
	// is exactly the first max cards of eligible -- the take the pre-budget
	// stand-in applied -- so the R-9 fallback stays byte-identical there. It
	// is computed before the Min below is finalised, because the budget can
	// strand a quantity-only search's forced Min.
	greedy := make([]state.ObjID, 0, len(budgetEligible))
	running := 0
	for _, id := range budgetEligible {
		if int32(len(greedy)) >= max {
			break
		}
		mv := manaValueOf(g, id)
		if hasBudget && running+mv > int(budget) {
			continue
		}
		running += mv
		greedy = append(greedy, id)
	}
	// A budget can strand a quantity-only search's forced Min: max was
	// clamped to len(budgetEligible), but the running sum may fit fewer than
	// that (library [3MV, 4MV], ChangeNum 2, WithTotalCMC 6 -- the greedy
	// take is one card), so Min == Max == 2 would pose an ask Decision
	// .Validate rejects for EVERY 2-pick -- a real host could never submit
	// and the match stalls. Lower the Min to the greedy count -- effDig's
	// mandatory-budget rule (cardflow.go), which its sibling effHiddenPick
	// applies too -- so a satisfying answer always exists. (Measured 0
	// corpus carriers combine a quantity-only filter with WithTotalCMC$;
	// this is general-correctness code in the direction of no wedge.)
	if hasBudget && min > int32(len(greedy)) {
		min = int32(len(greedy))
	}
	// The prompt must not offer a choice the decision will refuse. A
	// quantity-only search has Min == Max, so "up to" would be a lie the
	// player only discovers when their answer is rejected.
	count := strconv.Itoa(int(max))
	prompt := "Search a library: choose up to " + count + " card(s)"
	if min == max {
		prompt = "Search a library: choose " + count + " card(s)"
	}
	chooser := searchChooser(h, c, sa)
	// NoLooking$ True (Forge's line-1020 gate: with NoLooking the searching
	// player never LOOKS at the library -- no delayedReveal -- so the choose
	// is made over card backs): the options must not carry card names. The
	// IsRemembered legs of the Cultivate family and the seek-style shapes
	// route here; without this read the option labels leaked the library's
	// order one look at a time.
	noLooking := strings.EqualFold(strings.TrimSpace(sa.Params["NoLooking"]), "True")
	// DifferentNames$ True (Realms Uncharted): the picked cards must have
	// distinct names. One option per card name carries that name in Group, so
	// Decision.Validate's mutual-exclusion rule refuses any answer naming the
	// same card twice -- the wire enforces what Forge's one-at-a-time loop
	// (the DifferentNames fetchList filter) enforces there. The apply side
	// dedupes a host that bypassed the wire (applyLibrarySearch).
	differentNames := strings.EqualFold(strings.TrimSpace(sa.Params["DifferentNames"]), "True")
	// Forge's EACH multi-type search grammar ("EACH Forest & Plains"): with
	// every per-type cap at most 1 -- every corpus carrier -- the pick is
	// structured, not a flat count: one option per eligible card, the
	// type's ordinal in Option.Group, one decision whose Max is the number
	// of listed types that have at least one eligible card. The Group
	// exclusivity contract (decision.Decision.Validate) enforces at-most-one
	// per Group on the wire, which IS one pick per type; the ordinary
	// "search" resume arm carries the ordered picks, and applyLibrarySearch
	// re-checks each against the union matcher, so no new Ctx field and no
	// resume change. Min stays 0: the spec states a quality, so the
	// fail-to-find allowance (CR 701.23b) is kept -- a listed type with no
	// eligible card simply contributes no options and no Group, and its
	// pick is the one the player cannot make.
	eachSubs, isEach := eachAlternatives(spec)
	eachStructured := isEach && max <= 1 && SearchStatesQuality(spec)
	d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
		Min: int(min), Max: int(max), MaxSum: int(budget), Source: c.Source,
		ResumeKind: "search", ResumeSA: sa,
		// The walk's Remembered rides the ask (rules restores it on the
		// resume) so the re-entered eligibility recheck and the SubAbility$
		// after this one still see the cards RememberChanged$ captured -- a
		// cast spell's mid-resolution Remembered lives only in the resolving
		// Ctx frame, and without the ride the answer's recheck (and Nissa's
		// Pilgrimage's "one onto the battlefield" leg) would re-resolve
		// IsRemembered against an empty set and move nothing. A nested hidden
		// search resumes in the same resolution too, so the fetch list built
		// by a preceding search stays available to Card.IsRemembered and
		// Defined$ Remembered in the rest of this chain.
		ResumeRemembered: copyTargets(c.Remembered),
		Prompt:           prompt}
	if eachStructured {
		groups := 0
		sc := c.SpecContext(c.Controller)
		for ti, sub := range eachSubs {
			var typeIDs []state.ObjID
			for _, id := range lib {
				if MatchesSpecCtx(g, sub, id, sc) {
					typeIDs = append(typeIDs, id)
				}
			}
			if len(typeIDs) == 0 {
				continue
			}
			for _, id := range typeIDs {
				name := "a card"
				if o := g.Obj(id); o != nil && o.Face() != nil && !noLooking {
					name = o.Face().Name
				}
				d.Options = append(d.Options, decision.Option{Index: len(d.Options),
					Kind: "search", Label: name, Obj: id, Player: owner,
					Group: strconv.Itoa(ti)})
			}
			groups++
		}
		d.Min, d.Max = 0, groups
		d.Prompt = "Search a library: choose one card of each listed type"
		// The budget is NOT enforced on the structured branch (its options
		// carry no Value, so a MaxSum the wire advertises would be a cap
		// Validate sums to 0 over -- meaningless, and misleading to a
		// consumer). Clear it: 0 corpus carriers combine EACH with
		// WithTotalCMC$, and a future one needs per-type budget mechanics
		// designed, not a silent half-read.
		d.MaxSum = 0
	} else {
		if isEach {
			// A measured-absent shape kept loud rather than silently wrong:
			// a per-type ChangeNum$ above 1 (or a quantity-only EACH spec)
			// keeps the ordinary flat-count path over the union -- its
			// candidates are correct, its pick structure is not one-per-type.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "EACH ChangeType with per-type count above 1 resolves as a flat count"})
		}
		for _, id := range budgetEligible {
			name := "a card"
			var cardName string
			if o := g.Obj(id); o != nil && o.Face() != nil {
				cardName = o.Face().Name
				if !noLooking {
					name = cardName
				}
			}
			opt := decision.Option{Index: len(d.Options),
				Kind: "search", Label: name, Obj: id, Player: owner}
			// Only a budget search carries a Value: Option.Value is omitempty,
			// so a non-budget search's option list serialises byte-identically.
			if hasBudget {
				opt.Value = manaValueOf(g, id)
			}
			if differentNames && cardName != "" {
				opt.Group = cardName
			}
			d.Options = append(d.Options, opt)
		}
	}
	// The shared ask boundary (effects.Ask) refuses to post a decision whose
	// only legal answer is the empty one -- with zero eligible cards max
	// clamps to 0 and a stated-quality search's Min is already 0, so that is
	// exactly the Squadron Hawk fail-to-find shape that used to soft-lock the
	// game. AskEmpty resolves it silently through the stand-in below: the
	// search still shuffles, and a fail-to-find is legitimate under
	// CR 701.23b, so nothing is degraded and no R-9 Note is recorded.
	oc := Ask(h, d)
	if oc == AskAsked {
		return
	}
	// R-9: a host without a decision channel cannot ask a player, so it
	// supplies a deterministic answer in the player's place. For a
	// quantity-only search (CR 701.23d) the decision would refuse to find
	// fewer than Min cards, so the stand-in takes the first Min eligible
	// cards -- in the same ordered eligible list the decision's options
	// were built from -- or all of them when the library holds fewer
	// (701.23d's "as many as possible"). For a stated-quality search
	// (CR 701.23b) finding nothing is a legitimate fail-to-find, so the
	// stand-in still finds nothing, exactly as before. Either way the
	// search's unconditional shuffle still happens. An AskEmpty run takes
	// the same stand-in silently (no Note): skipping the ask is the correct
	// resolution, not a degradation.
	var picked []state.ObjID
	if !SearchStatesQuality(spec) {
		n := int(min)
		if n > len(greedy) {
			n = len(greedy)
		}
		if n > 0 {
			// DifferentNames$ True makes the stand-in distinct-name aware too:
			// a first-Min run over duplicate names would move two same-named
			// cards the apply side would then have to silently drop under the
			// Min the decision promised. (No corpus card pairs DifferentNames$
			// with WithTotalCMC$, so the budget greedy and this walk never
			// compete; the budget's greedy is the pick when both are present.)
			if differentNames && !hasBudget {
				seen := make(map[string]bool, n)
				for _, id := range eligible {
					if len(picked) >= n {
						break
					}
					var cardName string
					if o := g.Obj(id); o != nil && o.Face() != nil {
						cardName = o.Face().Name
					}
					if seen[cardName] {
						continue
					}
					seen[cardName] = true
					picked = append(picked, id)
				}
			} else {
				picked = append(picked, greedy[:n]...)
			}
		}
		if oc == AskNoHost {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "finds " + strconv.Itoa(n) + " card(s) (no engine host to ask)"})
		}
	} else if oc == AskNoHost {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
			Text: "finds no card (no engine host to ask)"})
	}
	applyLibrarySearch(h, c, sa, owner, to, picked, zones)
}

// libraryFetch is one owner and the direct-library objects moved for them.
// The slice stays in Defined$ order; a map would make emitted move/shuffle
// events nondeterministic.
type libraryFetch struct {
	owner state.PlayerID
	ids   []state.ObjID
}

// moveDefinedLibraryObjects implements Forge's hidden-origin Defined$ fetch
// list. When Defined$ resolves to object(s), those identities are the list to
// move; they do NOT select a library owner for a new search. As with the
// ordinary object path, ChangeType$ does not re-filter an already named
// object. This covers Remembered, ChosenCard, TopOfLibrary, BottomOfLibrary,
// and every future object-valued Defined selector through the same dispatch.
//
// Object selectors from Hand and Graveyard already use effChangeZone's normal
// object path. Library is the exceptional origin because it otherwise enters
// effSearchLibrary. A Defined$ yielding only player targets still belongs to
// the search-owner path below. Each touched owner is shuffled once, even when
// another sub-effect already moved every fetched object: Nissa's Pilgrimage's
// final fetch-list step is the script's shuffle point after its chosen Forest
// entered the battlefield.
//
// Optional$ True is a choice over the whole known fetch list, not permission
// to silently move it. Kenessos's DBBottom is the corpus example: after its
// player declines to put the revealed card onto the battlefield, they may put
// that card on the bottom. The yes/no decision suspends before either a move
// or a shuffle; its answer is scoped in Ctx so a nested optional fetch cannot
// inherit it. A no-host run keeps the previous deterministic mover (yes), the
// R-9 fallback used by the other optional mid-resolution effects.
//
// An unrecognised Defined$ selector is a fail-closed no-op here. In
// particular it must not pass through Defined's public source fallback: that
// fallback would make an unknown selector look like an object fetch list and
// silently consume the hidden-origin effect. A resolved player target takes
// the ordinary search-owner path below, regardless of which selector yielded
// it; the target kind, not a closed spelling list, defines the role.
func moveDefinedLibraryObjects(h Host, c *Ctx, sa *cards.SA, to state.Zone) bool {
	if strings.TrimSpace(sa.Params["Defined"]) == "" {
		return false
	}
	if _, owner := sa.Params["DefinedPlayer"]; owner {
		return false
	}
	g := h.Game()
	targets, known := knownDefinedTargets(h, c, sa.Params["Defined"])
	if !known {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognised Defined library fetch " + sa.Params["Defined"]})
		return true
	}
	var fetches []libraryFetch
	objectList := false
	playerList := false
	addOwner := func(p state.PlayerID) int {
		for i := range fetches {
			if fetches[i].owner == p {
				return i
			}
		}
		fetches = append(fetches, libraryFetch{owner: p})
		return len(fetches) - 1
	}
	for _, t := range targets {
		if t.IsPlayer {
			playerList = true
			continue
		}
		objectList = true
		o := g.Obj(t.Obj)
		if o == nil || int(o.Owner) >= len(g.Players) {
			continue
		}
		i := addOwner(o.Owner)
		if o.Zone == state.ZLibrary {
			fetches[i].ids = append(fetches[i].ids, o.ID)
		}
	}
	if !objectList {
		// A resolved player list identifies whose library to search. Deriving
		// that role from the resolved target kind covers every selector with a
		// player binding (Remembered, Targeted, ChosenPlayer, and future ones),
		// instead of losing an unlisted spelling to a direct-fetch no-op. An
		// empty object fetch (an empty Remembered/ChosenCard list or an empty
		// library's TopOfLibrary) is still a direct fetch and must not degrade
		// to a fresh whole-library search.
		return !playerList
	}

	optional := strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True")
	answer := c.DefinedLibraryMove
	c.DefinedLibraryMove = "" // fx42 scoping: a nested fetch asks for itself.
	if optional && answer == "" {
		prompt := strings.TrimSpace(sa.Params["OptionalPrompt"])
		if prompt == "" {
			prompt = "Move the selected card(s)?"
		}
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
			Source: c.Source, ResumeKind: "defined_library_optional", ResumeSA: sa,
			ResumeRemembered: append([]state.Target(nil), c.Remembered...), Prompt: prompt,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Label: "Yes", Player: c.Controller},
				{Index: 1, Kind: "no", Label: "No", Player: c.Controller},
			}}
		if Ask(h, d) == AskAsked {
			return true
		}
		// AskNoHost cannot represent a decline. Preserve the prior direct-move
		// fallback rather than leaving a headless resolution suspended.
		answer = "yes"
	}
	if optional && answer == "no" {
		return true
	}

	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	// The AtEOT$ rider's affected set, collected across every fetch and
	// scheduled by ONE call after the loop (one Note per call, never per
	// owner).
	var ateotMoved []state.ObjID
	for i := range fetches {
		f := &fetches[i]
		moved := make([]state.ObjID, 0, len(f.ids))
		for _, id := range f.ids {
			o := g.Obj(id)
			// Recheck at the point of movement: a malformed or stale Defined$
			// target must not move an object from a new zone.
			if o == nil || o.Zone != state.ZLibrary || o.Owner != f.owner {
				continue
			}
			settleChangeZoneMove(h, c, sa, id, state.ZLibrary, to, withKind, withAmt)
			eventForgetChanged(h, c, sa, id)
			moved = append(moved, id)
			ateotMoved = append(ateotMoved, id)
			if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
				h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: f.owner, Text: "entered tapped"})
			}
		}
		shuffleLibrary(h, sa, f.owner)
		placeLibraryObjects(h, sa, f.owner, moved, to)
		// Explicit Reveal$ on a Defined$ fetch list (Forge reveals movedCards
		// whenever Reveal$ names the effect, defined or not): the same public
		// Note payload applyLibrarySearch emits -- no auto-reveal here, since
		// a Defined$ list is never revealed by default.
		if strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True") && len(moved) > 0 {
			h.Emit(events.Event{Kind: events.Note, Player: f.owner, IDs: moved})
		}
	}
	scheduleAtEOT(h, c, sa, ateotMoved)
	return true
}

// effManifest implements Forge's Manifest primitive (Reality Shift's
// "its controller manifests the top card of their library", Whisperwood
// Elemental's bare `DB$ Manifest` trigger body): move the top card of each
// named player's library onto the battlefield FACE DOWN (CR 708.5). The move
// is the REAL card object -- never a token mint: the manifested 2/2 keeps
// the object's identity, so if it dies it reaches the graveyard as itself
// (CR 708.9's reveal is the FaceDown clear on leaving the battlefield, and
// the view's FaceDown redaction hides the face from non-controllers while it
// stays in play). Each move is one Secret MoveZone with Player set to the
// manifesting player: Secret is what keeps the event's Obj out of every
// other seat's projection (redaction rule 1) -- a library-to-battlefield
// move would otherwise stay public under rule 2 and leak the face through
// the transcript.
//
// Scope, measured over the corpus's 33 plain-Manifest lines: the default
// top-card shape (the 9 bare `DB$ Manifest` trigger bodies), a
// `DefinedPlayer$` selector through searchPlayers's grammar (Reality
// Shift's `TargetedController`) and a literal/SVar `Amount$` (default 1;
// a value resolving to <= 0 manifests nothing, no event) are implemented.
// Every other shape -- `Defined$` object manifests, the `Choices$`
// chooser forms, `RememberManifested$ True`, an unresolvable `Amount$`
// body (Y, or X outside a cast's own X-value) -- emits the SAME loud
// "unimplemented API Manifest" note the unimplemented-API fallback emits
// and moves nothing: fail loud, never silently move the wrong card.
// ManifestDread is a DIFFERENT API (31 corpus files) and stays on that
// fallback; turning a face-down permanent face up (CR 708.6) is not
// implemented anywhere (AGENTS.md's manifest row).
func effManifest(h Host, c *Ctx, sa *cards.SA) {
	if strings.TrimSpace(sa.Params["Defined"]) != "" ||
		strings.TrimSpace(sa.Params["Choices"]) != "" ||
		strings.EqualFold(strings.TrimSpace(sa.Params["RememberManifested"]), "True") {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented API Manifest"})
		return
	}
	amount := int32(1)
	if raw, present := sa.Params["Amount"]; present {
		// X/Y (and any body Num's grammar cannot resolve) are out of scope:
		// loud, never a degraded count silently moving a wrong number of
		// cards.
		if raw == "X" || raw == "Y" {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API Manifest"})
			return
		}
		n, ok := NumResolved(h, c, sa, "Amount", 1)
		if !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API Manifest"})
			return
		}
		amount = n
	}
	if amount <= 0 {
		return
	}
	g := h.Game()
	for _, p := range searchPlayers(h, c, sa) {
		if int(p) >= len(g.Players) {
			continue
		}
		n := amount
		if l := int32(len(g.Zone(state.ZLibrary, p))); l < n {
			n = l
		}
		for i := int32(0); i < n; i++ {
			// Index 0 is the TOP of the library (the end a Draw takes). The
			// MoveZone fold removes the object as it lands, so the zone is
			// re-read each iteration.
			top := g.Zone(state.ZLibrary, p)[0]
			h.Emit(events.Event{Kind: events.MoveZone, Obj: top, Player: p,
				From: state.ZLibrary, To: state.ZBattlefield,
				Counter: "entered_face_down", Secret: true})
		}
	}
}

// effCloak implements Forge's Cloak primitive (veiled_ascension's upkeep
// trigger, unexplained_absence's per-player cloak, cryptic_coat's ETB cloak):
// CR 708.5's cloak variant -- the named card objects move onto the
// battlefield FACE DOWN as 2/2 creatures with ward {2}. The move is the REAL
// card object, the effManifest shape with a different Counter value (one
// Secret MoveZone per card, "entered_cloaked" instead of
// "entered_face_down" -- the marker events/apply.go folds into
// state.Object.Cloaked, which rules/layers.go and rules/trigger_match.go
// read for the ward {2}; the view's FaceDown redaction covers both
// variants). A cloak's Defined$ names card objects that may sit in the
// library, exile or hand (the Remembered carriers move cards that just
// arrived there), so unlike effManifest the object shapes resolve directly.
//
// Scope, measured over the corpus's 11 Cloak lines: the per-player top-card
// shapes (DefinedPlayer$, or Defined$ TopOfLibrary whose listed player's OWN
// library is taken -- never the resolver's), the ctx-Remembered object
// shapes (become_anonymous, hide_in_plain_sight, expose_the_culprit), a
// literal/SVar Amount$ (default 1; a value resolving to <= 0 cloaks
// nothing, no event) and the riders Tapped$ (enter tapped, the
// MoveZone-then-Tap pair), Shuffle$ (the standard Secret shuffle of each
// affected player's library afterwards) and RememberCloaked$ (each cloaked
// object joins the resolution's Remembered -- cryptic_coat's chained attach)
// are implemented. SubAbility$ is free (the ordinary chain). Every other
// shape -- the Choices$ cloak-from-hand chooser (vannifar), Defined$
// ValidLibrary (etrata), an unresolvable Amount$ body -- emits the SAME loud
// "unimplemented API Cloak" note the unimplemented fallback emits and moves
// nothing: fail loud, never silently move the wrong card. Turning a cloaked
// card face up (CR 708.6) is not implemented anywhere (the
// Morph/Megamorph/Disguise ticket owns the shared turn-face-up path).
func effCloak(h Host, c *Ctx, sa *cards.SA) {
	loud := func() {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unimplemented API Cloak"})
	}
	defined := strings.TrimSpace(sa.Params["Defined"])
	if strings.TrimSpace(sa.Params["Choices"]) != "" ||
		strings.Contains(defined, "ValidLibrary") {
		loud()
		return
	}
	amount := int32(1)
	if raw, present := sa.Params["Amount"]; present {
		// X/Y (and any body Num's grammar cannot resolve) are out of scope:
		// loud, never a degraded count silently moving a wrong number of
		// cards.
		if raw == "X" || raw == "Y" {
			loud()
			return
		}
		n, ok := NumResolved(h, c, sa, "Amount", 1)
		if !ok {
			loud()
			return
		}
		amount = n
	}
	if amount <= 0 {
		return
	}
	tapped := strings.EqualFold(strings.TrimSpace(sa.Params["Tapped"]), "True")
	shuffle := strings.EqualFold(strings.TrimSpace(sa.Params["Shuffle"]), "True")
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberCloaked"]), "True")
	g := h.Game()
	shuffled := make(map[state.PlayerID]bool)
	cloak := func(id state.ObjID) {
		o := g.Obj(id)
		if o == nil || o.Zone == state.ZBattlefield {
			// A missing object moves nothing; a card already on the
			// battlefield is not a cloak candidate (every corpus shape sources
			// from library/exile/hand).
			return
		}
		from := o.Zone
		// Player rides the CLOAKED card's controller: Secret is what keeps
		// the event's Obj out of every other seat's projection (redaction
		// rule 1), and the seat that may look at a face-down card is its
		// controller (CR 708.5).
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, Player: o.Controller,
			From: from, To: state.ZBattlefield,
			Counter: "entered_cloaked", Secret: true})
		if remember {
			// RememberCloaked$ is ctx level (the cryptic_coat attach chain
			// reads it within the same resolution); the persistent list is
			// left alone, the RememberChanged$ convention.
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
		if tapped {
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: o.Controller,
				Text: "entered tapped"})
		}
		shuffled[o.Owner] = true
	}
	if strings.TrimSpace(sa.Params["DefinedPlayer"]) != "" {
		// The per-player top-card shape (unexplained_absence's
		// "Defined$ TopOfLibrary | DefinedPlayer$ RememberedController"):
		// each listed player's OWN top Amount$ cards -- searchPlayers's
		// DefinedPlayer$ precedence, the effManifest loop's move shape.
		for _, p := range searchPlayers(h, c, sa) {
			if int(p) >= len(g.Players) {
				continue
			}
			n := amount
			if l := int32(len(g.Zone(state.ZLibrary, p))); l < n {
				n = l
			}
			for i := int32(0); i < n; i++ {
				// Index 0 is the TOP of the library (the end a Draw takes).
				// The MoveZone fold removes the object as it lands, so the
				// zone is re-read each iteration.
				top := g.Zone(state.ZLibrary, p)[0]
				cloak(top)
			}
		}
	} else {
		switch defined {
		case "", "TopOfLibrary":
			// The bare top-card shape (veiled_ascension, ransom_note,
			// cryptic_coat): the resolving controller's top card, the
			// TopOfLibrary selector's own anchor (effects/context.go).
			targets, ok := definedSpec(h, c, "TopOfLibrary")
			if !ok {
				loud()
				return
			}
			for _, t := range targets {
				if t.IsPlayer {
					continue
				}
				cloak(t.Obj)
			}
		case "Remembered":
			// The Remembered-object shape (become_anonymous,
			// hide_in_plain_sight, expose_the_culprit): each remembered card
			// object cloaks from wherever it sits now (library top, exile,
			// hand).
			for _, t := range objectsOf(copyTargets(c.Remembered)) {
				cloak(t.Obj)
			}
		default:
			loud()
			return
		}
	}
	if shuffle {
		// Shuffle$ True: the standard Secret events.Shuffle for each player
		// whose library lost a card (become_anonymous and expose_the_culprit
		// carry it; the deterministic order comes from the host's own rng
		// path every other library shuffle uses).
		for _, p := range g.AliveFrom(c.Controller) {
			if shuffled[p] {
				shuffleLibraryOrder(h, p)
			}
		}
	}
}

// searchPlayers resolves whose library is searched. DefinedPlayer$ takes
// precedence over Defined$; with neither, the source controller searches.
func searchPlayers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	spec, explicit := sa.Params["DefinedPlayer"]
	if !explicit {
		spec, explicit = sa.Params["Defined"]
	}
	if !explicit || strings.TrimSpace(spec) == "" {
		return []state.PlayerID{c.Controller}
	}

	var targets []state.Target
	switch spec {
	case "RememberedController":
		for _, t := range c.Remembered {
			if t.IsPlayer {
				targets = append(targets, t)
			} else if o := h.Game().Obj(t.Obj); o != nil {
				targets = append(targets, state.Target{Player: o.Controller, IsPlayer: true})
			}
		}
	default:
		// Defined only reads the Defined key, so a tiny temporary SA lets this
		// helper share its deterministic selector grammar without mutating the
		// immutable compiled SA.
		targets = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
	}
	seen := make(map[state.PlayerID]bool)
	out := make([]state.PlayerID, 0, len(targets))
	for _, t := range targets {
		p := PlayerOf(h, c, t)
		if int(p) >= len(h.Game().Players) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// searchChooser resolves who answers the search prompt. You is the default;
// Targeted uses the first chosen target, and Opponent uses the first living
// opponent in deterministic turn order.
func searchChooser(h Host, c *Ctx, sa *cards.SA) state.PlayerID {
	switch sa.Params["Chooser"] {
	case "Targeted":
		if len(c.Targets) > 0 {
			return PlayerOf(h, c, c.Targets[0])
		}
	case "Opponent":
		for _, p := range h.Game().AliveFrom(c.Controller) {
			if p != c.Controller {
				return p
			}
		}
	}
	return c.Controller
}

// hiddenPickPlayers resolves whose cards the hidden-origin pick offers, the
// pick's analogue of searchPlayers: DefinedPlayer$ through the shared
// selector grammar first, then the targeted PLAYERS (Forge's
// getFirstTargetedPlayer for a usesTargeting effect), then the source
// controller -- Forge's getDefinedPlayers(null) defaults to "You".
func hiddenPickPlayers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	if strings.TrimSpace(sa.Params["DefinedPlayer"]) != "" {
		return searchPlayers(h, c, sa)
	}
	if _, targeted := sa.Params["ValidTgts"]; targeted {
		var out []state.PlayerID
		seen := make(map[state.PlayerID]bool, len(c.Targets))
		for _, t := range c.Targets {
			if !t.IsPlayer {
				continue
			}
			p := PlayerOf(h, c, t)
			if int(p) >= len(h.Game().Players) || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
		if len(out) > 0 {
			return out
		}
	}
	return []state.PlayerID{c.Controller}
}

// hiddenPickChooser resolves who answers the pick. A Chooser$ spelling wins
// (Targeted/Opponent through searchChooser's grammar, You the controller);
// with none the decider is the fetch player, exactly Forge's
// `decider = Objects.requireNonNullElse(chooser, player)`.
func hiddenPickChooser(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID) state.PlayerID {
	switch sa.Params["Chooser"] {
	case "Targeted", "Opponent":
		return searchChooser(h, c, sa)
	case "You":
		return c.Controller
	}
	return owner
}

// effHiddenPick is Forge's changeHiddenOriginResolve for a Hidden$ True
// ChangeZone whose origin zones are PUBLIC (Battlefield, Graveyard, Exile,
// Command, Stack) or name no modelled zone at all (Origin$ Sideboard: this
// engine holds no outside-the-game cards, so the wish finds nothing -- one
// loud note says so, and the SubAbility$ chain still runs, Burning Wish's
// self-exile included). With no Defined$ the fetch list is the origin zones'
// cards matching ChangeType$ -- game-wide for a public origin when no fetch
// player is named, the named fetch player's own zones otherwise -- and the
// chooser picks ChangeNum$ of them (Mandatory$ True makes the pick
// compulsory; otherwise Min 0, "you may"). Zones that hold hidden info are
// handled by their own walkers (Library's search, Hand's movers); this one
// never offers a hidden card by name, and a public-origin pick never
// shuffles (Forge's shuffle condition needs Library in the origin).
func effHiddenPick(h Host, c *Ctx, sa *cards.SA, to state.Zone, originZones []state.Zone, originAll bool, originValid bool, from string) {
	// fx42 scoping: capture and clear the answered pick (and the cursor that
	// binds it to the fetch player that asked) before anything else, so a
	// nested pick below cannot inherit them.
	ans := c.HiddenPick
	done := c.HiddenPickDone
	cursor := c.HiddenPickTarget
	c.HiddenPick, c.HiddenPickDone, c.HiddenPickTarget = nil, false, 0
	if !originValid {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "ChangeZone Origin$ " + from + " includes a zone this engine does not model (no outside-the-game cards exist); nothing is offered from it"})
	}
	players := hiddenPickPlayers(h, c, sa)
	// Forge branches on the origin zones, not on the fetch player: game-wide
	// only when the origin holds no hidden-info zone and no fetch player was
	// named (Kor Skyfisher's ChangeType$ filter does the scoping).
	gameWide := !zoneIn(originZones, state.ZHand) && !zoneIn(originZones, state.ZLibrary) &&
		strings.TrimSpace(sa.Params["DefinedPlayer"]) == ""
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	max := Num(h, c, sa, "ChangeNum", 1)
	if max < 0 {
		max = 0
	}
	mandatory := strings.EqualFold(strings.TrimSpace(sa.Params["Mandatory"]), "True")
	noLooking := strings.EqualFold(strings.TrimSpace(sa.Params["NoLooking"]), "True")
	withKind := sa.Params["WithCountersType"]
	var withAmt int32
	if to == state.ZBattlefield && withKind != "" {
		withAmt = withCounterAmount(h, c, sa)
	}
	// WithTotalCMC$ is the cumulative mana-value budget over the picked cards
	// (Lively Dirge's DBReturn, Technomancer, Legion's Chant, Pair o' Dice
	// Lost: "return up to N creature cards with total mana value M or less"),
	// the exact parameter effDig reads on its own window. A card whose own
	// mana value exceeds the budget can never be picked, and the running sum
	// of the picks must not exceed it either; the mechanics below mirror
	// effDig's (affordable filter, Decision.MaxSum + Option.Value on the wire,
	// a greedy stand-in take, a mandatory Min lowered to what the budget
	// affords). Absent the param the budget is 0, budgetEligible == eligible
	// and every read below is a no-op, so a non-budget pick emits
	// byte-identically. Present but unresolvable degrades to budget 0 --
	// Num's documented convention.
	budget, hasBudget := NumResolved(h, c, sa, "WithTotalCMC", 0)
	if budget < 0 {
		budget = 0
	}
	apply := func(owner state.PlayerID, ids []state.ObjID) []state.ObjID {
		g := h.Game()
		moved := make([]state.ObjID, 0, len(ids))
		for _, id := range ids {
			o := g.Obj(id)
			// Recheck at the point of movement: the answered card must still
			// sit in an origin zone and match the filter, or it stays.
			if o == nil || !zoneIn(originZones, o.Zone) ||
				!MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				continue
			}
			settleChangeZoneMoveAs(h, c, sa, id, o.Zone, to, withKind, withAmt, o.Owner, true)
			moved = append(moved, id)
			if strings.EqualFold(sa.Params["RememberChanged"], "True") {
				eventRemember(h, c, id)
			}
			eventForgetChanged(h, c, sa, id)
			if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
				h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: o.Owner, Text: "entered tapped"})
			}
		}
		// Explicit Reveal$ on the pick (Karn, the Great Creator's [-2]): the
		// same public Note payload the library search's reveal emits. No
		// default auto-reveal here: the pick's origin zones are public, so
		// every offered name was already known.
		if strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True") && len(moved) > 0 {
			h.Emit(events.Event{Kind: events.Note, Player: owner, IDs: moved})
		}
		// AtEOT$ rides the pick's moved set as well (latent: no corpus carrier
		// reaches the hidden pick with the param today, but a future one must
		// not be dropped silently).
		scheduleAtEOT(h, c, sa, moved)
		return moved
	}
	for i, owner := range players {
		var eligible []state.ObjID
		addPool := func(ids []state.ObjID) {
			for _, id := range ids {
				o := h.Game().Obj(id)
				if o == nil || !MatchesSpecCtx(h.Game(), spec, id, c.SpecContext(c.Controller)) {
					continue
				}
				eligible = append(eligible, id)
			}
		}
		if gameWide {
			// Forge's game.getCardsIn(origin): every player's cards in the
			// origin zones -- the Game.Zone accessor is per (zone, player), so
			// the game-wide pool is the union over players in seat order,
			// deterministic.
			for _, z := range originZones {
				for p := range h.Game().Players {
					addPool(h.Game().Zone(z, state.PlayerID(p)))
				}
			}
		} else {
			for _, z := range originZones {
				addPool(h.Game().Zone(z, owner))
			}
		}
		// budgetEligible is the pickable set: spec-matching AND individually
		// affordable under WithTotalCMC$ (no budget => identical to eligible).
		budgetEligible := eligible
		if hasBudget {
			budgetEligible = make([]state.ObjID, 0, len(eligible))
			for _, id := range eligible {
				if manaValueOf(h.Game(), id) <= int(budget) {
					budgetEligible = append(budgetEligible, id)
				}
			}
		}
		m := max
		if m > int32(len(budgetEligible)) {
			m = int32(len(budgetEligible))
		}
		// greedy is the deterministic stand-in take under the cumulative
		// budget: walk budgetEligible in pool order and take each card only
		// while the running sum still fits, bounded by the pick count m. With
		// no budget every card fits and greedy is exactly the first m cards of
		// eligible -- the take the pre-budget stand-in applied -- so the R-9
		// fallback stays byte-identical there.
		greedy := make([]state.ObjID, 0, len(budgetEligible))
		running := 0
		for _, id := range budgetEligible {
			if int32(len(greedy)) >= m {
				break
			}
			mv := manaValueOf(h.Game(), id)
			if hasBudget && running+mv > int(budget) {
				continue
			}
			running += mv
			greedy = append(greedy, id)
		}
		if done && i < cursor {
			// This fetch player answered on an earlier pass, before a later
			// one suspended the walk (the hand walk's continuation contract).
			continue
		}
		if done && i == cursor {
			apply(owner, ans)
			continue
		}
		if len(budgetEligible) == 0 || m == 0 {
			// No eligible card, or an empty-only ChangeNum$ 0 pick: both
			// complete silently before optionality can matter, and a public
			// origin has no shuffle to fail to perform.
			continue
		}
		chooser := hiddenPickChooser(h, c, sa, owner)
		prompt := strings.TrimSpace(sa.Params["SelectPrompt"])
		if prompt == "" {
			prompt = "Choose up to " + strconv.Itoa(int(m)) + " card(s)"
		}
		d := &decision.Decision{Player: chooser, Kind: decision.KChoose,
			Min: 0, Max: int(m), MaxSum: int(budget), Source: c.Source,
			// The same remembered ride the hand_move ask carries: the
			// re-entered effHiddenPick revalidates against ChangeType$, which
			// can be a ctx-Remembered predicate.
			ResumeKind: "hidden_pick", ResumeSA: sa, ResumeTarget: i,
			ResumeRemembered: copyTargets(c.Remembered),
			Prompt:           prompt}
		if mandatory {
			d.Min = int(m)
		}
		// A mandatory budget pick whose m exceeds what the budget affords must
		// not demand more picks than it can pay for: lower the Min to the
		// forced greedy count so the ask can be satisfied (effDig's rule).
		if hasBudget && d.Min > len(greedy) {
			d.Min = len(greedy)
		}
		for _, id := range budgetEligible {
			name := "a card"
			if o := h.Game().Obj(id); o != nil && o.Face() != nil && !noLooking {
				name = o.Face().Name
			}
			opt := decision.Option{Index: len(d.Options),
				Kind: "hidden_pick", Label: name, Obj: id, Player: owner}
			// Only a budget pick carries a Value: Option.Value is omitempty, so
			// a non-budget pick's option list serialises byte-identically.
			if hasBudget {
				opt.Value = manaValueOf(h.Game(), id)
			}
			d.Options = append(d.Options, opt)
		}
		oc := Ask(h, d)
		if oc == AskAsked {
			return
		}
		// R-9: a host without a decision channel cannot ask, so it takes the
		// forced greedy take over the budget-eligible pool -- under a budget
		// the cumulative cap decides which cards fit (effDig's exact mirror);
		// without one greedy is the first m eligible cards, and the
		// DifferentNames$ variant below keeps its distinct-named-first walk
		// (no corpus card carries DifferentNames$ beside WithTotalCMC$, so the
		// two stand-ins never compete).
		var picked []state.ObjID
		if differentNamesEnabled(sa) && !hasBudget {
			seen := make(map[string]bool, m)
			for _, id := range eligible {
				if len(picked) >= int(m) {
					break
				}
				var cardName string
				if o := h.Game().Obj(id); o != nil && o.Face() != nil {
					cardName = o.Face().Name
				}
				if seen[cardName] {
					continue
				}
				seen[cardName] = true
				picked = append(picked, id)
			}
		} else {
			picked = append(picked, greedy...)
		}
		if oc == AskNoHost {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: chooser,
				Text: "picks " + strconv.Itoa(len(picked)) + " card(s) (no engine host to ask)"})
		}
		apply(owner, picked)
	}
}

// differentNamesEnabled is the one DifferentNames$ read shared by the two
// hidden-origin walkers that enforce it, so their option/answer contract
// cannot drift.
func differentNamesEnabled(sa *cards.SA) bool {
	return strings.EqualFold(strings.TrimSpace(sa.Params["DifferentNames"]), "True")
}

// landTypesOf lists the land subtypes o's face names, in face order: the
// supertypes (Basic, Snow) and the Land type word are stripped, so a Basic
// Forest leaves exactly Forest. The search paths this serves are
// Land.Basic-filtered (ShareLandType$ only rides hidden-library searches),
// so a face whose remaining types are empty never contributes a type.
func landTypesOf(o *state.Object) []string {
	f := o.Face()
	if f == nil {
		return nil
	}
	out := make([]string, 0, len(f.Types))
	for _, t := range f.Types {
		switch t {
		case "Land", "Basic", "Snow":
			continue
		}
		out = append(out, t)
	}
	return out
}

// SharedLandTypes reports whether every named object shares at least one
// land type: fewer than two objects is trivially true, otherwise the
// intersection of their land-subtype sets must be non-empty. Both readers of
// ShareLandType$ True go through it so the wire rule (rules.Submit) and the
// host-bypass trim (applyLibrarySearch) cannot drift on what "share" means.
func SharedLandTypes(g *state.Game, ids []state.ObjID) bool {
	if len(ids) <= 1 {
		return true
	}
	var shared map[string]bool
	for _, id := range ids {
		o := g.Obj(id)
		if o == nil {
			return false
		}
		types := landTypesOf(o)
		if shared == nil {
			shared = make(map[string]bool, len(types))
			for _, t := range types {
				shared[t] = true
			}
			continue
		}
		ok := false
		keep := make(map[string]bool, len(types))
		for _, t := range types {
			if shared[t] {
				ok = true
				keep[t] = true
			}
		}
		if !ok {
			return false
		}
		shared = keep
	}
	return true
}

// trimSharedLandTypes is SharedLandTypes' deterministic enforcement for a
// host that bypassed the wire: keep the first chosen object in answer order
// and every later one that still shares a type with everything kept before
// it.
func trimSharedLandTypes(g *state.Game, chosen []state.ObjID) []state.ObjID {
	if len(chosen) <= 1 {
		return chosen
	}
	var shared map[string]bool
	out := make([]state.ObjID, 0, len(chosen))
	for _, id := range chosen {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		types := landTypesOf(o)
		if shared == nil {
			shared = make(map[string]bool, len(types))
			for _, t := range types {
				shared[t] = true
			}
			out = append(out, id)
			continue
		}
		ok := false
		keep := make(map[string]bool, len(types))
		for _, t := range types {
			if shared[t] {
				ok = true
				keep[t] = true
			}
		}
		if !ok {
			continue
		}
		out = append(out, id)
		shared = keep
	}
	return out
}

func applyLibrarySearch(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID, to state.Zone, chosen []state.ObjID, zones []state.Zone) {
	// The search-control/replacement boundary (Opposition Agent's class):
	// the moves this function emits are the moves OF A SEARCH, and the host
	// that models that fact scopes its FoundSearchingLibrary$ replacements
	// and ControlOpponentsSearchingLibrary$ redirects to them. The optional
	// hooks keep test hosts (which do not model the state) working.
	if b, ok := h.(interface {
		BeginLibrarySearch(owner state.PlayerID)
		EndLibrarySearch()
	}); ok {
		b.BeginLibrarySearch(owner)
		defer b.EndLibrarySearch()
	}
	g := h.Game()
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	// DifferentNames$ True (Realms Uncharted): the options carried one Group
	// per card name, so a validated wire answer cannot repeat a name. A host
	// that bypassed the wire (bot clamp top-up, a direct resume) is deduped
	// here deterministically -- first per name in answer order -- so the
	// engine and its clients cannot drift on what the constraint means.
	if strings.EqualFold(strings.TrimSpace(sa.Params["DifferentNames"]), "True") {
		seenNames := make(map[string]bool, len(chosen))
		deduped := make([]state.ObjID, 0, len(chosen))
		for _, id := range chosen {
			name := ""
			if o := g.Obj(id); o != nil && o.Face() != nil {
				name = o.Face().Name
			}
			if name != "" && seenNames[name] {
				continue
			}
			if name != "" {
				seenNames[name] = true
			}
			deduped = append(deduped, id)
		}
		chosen = deduped
	}
	// ShareLandType$ True (Myriad Landscape): every chosen card must share at
	// least one land type with the rest ("up to two basic land cards that
	// share a land type"). A validated wire answer cannot violate it
	// (rules.Submit rejects such an intent and the decision stays pending),
	// so this trim is the host-bypass guard -- the same shape the
	// DifferentNames dedupe above serves: keep the first chosen card in
	// answer order and every later one that still shares a type with
	// everything kept before it.
	if strings.EqualFold(strings.TrimSpace(sa.Params["ShareLandType"]), "True") {
		chosen = trimSharedLandTypes(g, chosen)
	}
	moved := make([]state.ObjID, 0, len(chosen))
	for _, id := range chosen {
		o := g.Obj(id)
		if o == nil || o.Owner != owner || !zoneIn(zones, o.Zone) ||
			!MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
			continue
		}
		// A chosen candidate from a PUBLIC origin zone (OriginAlternative$
		// Graveyard/Hand/Exile) moves through the ordinary cross-zone settle:
		// no library shuffle or LibraryPosition$ placement follows it, and
		// because it never left the library nothing here can disturb the
		// library order. The library half keeps the existing direct emit, which
		// is where the exile-provenance IDs and the Imprint rider live.
		if o.Zone != state.ZLibrary {
			withKind := ""
			var withAmt int32
			if to == state.ZBattlefield && sa.Params["WithCountersType"] != "" {
				withKind = sa.Params["WithCountersType"]
				withAmt = withCounterAmount(h, c, sa)
			}
			settleChangeZoneMoveAs(h, c, sa, id, o.Zone, to, withKind, withAmt, owner, true)
			// AttachedTo$ on an alternative-zone pick (Boonweaver Giant's "put
			// it onto the battlefield attached to CARDNAME", Runed Crown, Arachnus
			// Web): the same rider the library branch below applies -- without it
			// an Aura found in the graveyard or hand enters unattached and the
			// CR 704.5m SBA sweeps it. GainControl$/WithCounters*/Transformed$
			// already ride settleChangeZoneMoveAs above.
			if to == state.ZBattlefield {
				changeZoneAttachedTo(h, c, sa, id)
			}
			moved = append(moved, id)
			// settleChangeZoneMoveAs already appended the object to the
			// resolution's Remembered for RememberChanged$; only the persistent
			// event-backed half is left here. RememberSearched$ is not read
			// there, so its ctx append is made here too.
			if strings.EqualFold(sa.Params["RememberChanged"], "True") {
				eventRemember(h, c, id)
			}
			if strings.EqualFold(strings.TrimSpace(sa.Params["RememberSearched"]), "True") {
				c.Remembered = append(c.Remembered, state.Target{Obj: id})
				eventRemember(h, c, id)
			}
			eventForgetChanged(h, c, sa, id)
			if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
				h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: owner, Text: "entered tapped"})
			}
			continue
		}
		ev := moveZoneEvent(c, id, state.ZLibrary, to)
		ev.Player = owner
		applyFaceDownMarker(h, sa, c, &ev, to)
		h.Emit(ev)
		if to == state.ZExile && c.Source != 0 {
			if o := g.Obj(id); o != nil && !o.IsToken {
				h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, IDs: []state.ObjID{id}, Text: "exiled-with"})
			}
		}
		moved = append(moved, id)
		if to == state.ZBattlefield && sa.Params["WithCountersType"] != "" {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id,
				Counter: sa.Params["WithCountersType"], Amount: withCounterAmount(h, c, sa)})
		}
		// GainControl$ on a library search (Act on Impulse's "you may play
		// those cards" family's put-onto-battlefield relatives): same settle
		// order as every other mover -- move first, then the control change,
		// then the AttachedTo$ rider (the Origin$ Library ChangeZone lines
		// carrying it, e.g. an "return an Aura ... attached to CARDNAME" search).
		if to == state.ZBattlefield {
			applyGainControl(h, c, sa, id)
			changeZoneAttachedTo(h, c, sa, id)
		}
		if strings.EqualFold(sa.Params["RememberChanged"], "True") {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
			eventRemember(h, c, id)
		}
		if strings.EqualFold(strings.TrimSpace(sa.Params["RememberSearched"]), "True") {
			// RememberSearched$ True (Tempt with Discovery's tempting offer):
			// the cards the search found join the resolution's Remembered --
			// the same Ctx set RememberChanged$ feeds -- so the follow-up sub
			// ("for each opponent who searched, search again") counts them
			// through Count$RememberedSize and repeats the search that many
			// times.
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
			eventRemember(h, c, id)
		}
		eventForgetChanged(h, c, sa, id)
		if to == state.ZBattlefield && strings.EqualFold(sa.Params["Tapped"], "True") {
			// This establishes the object's entry state; it is not the CR
			// 701.21a event of becoming tapped. Text is part of the replayed
			// event payload, so rules can distinguish it from an ordinary Tap
			// while replay folds the same tapped state.
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: owner, Text: "entered tapped"})
		}
		// StaticEffect$ on the library-origin branch: the same rider the
		// shared settle path applied for the alternative-origin branch above.
		if to == state.ZBattlefield {
			applyStaticEffect(h, c, sa, to, []state.ObjID{id})
		}
	}
	// The search's reveal (hiddenreveal1): Forge's changeHiddenOriginResolve
	// reveals the moved cards when Reveal$ says so, and ALSO by default when
	// the search's ChangeType$ states a quality (anything beyond the bare
	// "Card"), the destination is not the battlefield and no Defined$ fixed
	// the list -- the "reveal it" half of a quality search (Idyllic Tutor,
	// Cultivate, Nissa's Pilgrimage; Demonic Tutor's bare Card ChangeType$
	// stays hidden). A Hidden$ move without an explicit Reveal$ suppresses
	// the default: the change is hidden. The record is the same payload the
	// Reveal primitive emits (a public Note carrying the ids; no Text --
	// view.Describe renders the names), emitted after the moves exactly where
	// Forge's own reveal call sits.
	reveal := strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True") ||
		(to != state.ZBattlefield && spec != "Card" &&
			strings.TrimSpace(sa.Params["Defined"]) == "" &&
			!strings.EqualFold(strings.TrimSpace(sa.Params["NoReveal"]), "True"))
	if strings.EqualFold(strings.TrimSpace(sa.Params["Hidden"]), "True") &&
		!strings.EqualFold(strings.TrimSpace(sa.Params["Reveal"]), "True") {
		reveal = false
	}
	if reveal && len(moved) > 0 {
		h.Emit(events.Event{Kind: events.Note, Player: owner, IDs: moved})
	}

	// AtEOT$ rides the search's moved set too. Scheduled BEFORE the
	// may-shuffle confirm: a searchShuffleTail suspension is a tail-only
	// re-entry (effSearchLibrary's SearchShuffle branch), which would never
	// reach a schedule call placed after it -- the moved cards and their
	// registrations are already game state by then.
	scheduleAtEOT(h, c, sa, moved)
	if searchShuffleTail(h, c, sa, owner, moved, to) {
		return // the may-shuffle confirm suspended the resolution
	}
}

// shuffleLibrary applies the default hidden-library shuffle used by searches
// and direct Defined$ fetches: the CR 701.23d shuffle, unless the SA opts out
// (NoShuffle$ True / Shuffle$ False). A hand put-back is different: it
// shuffles only when its own SA explicitly says Shuffle$ True, and uses
// shuffleLibraryExplicit below. ShuffleNonMandatory$ True -- Forge's "Do you
// want to shuffle the library?" confirm, an information-mercy so a player may
// keep the library order a search just taught them -- is NOT read here: this
// helper is the mandatory path (the fail-to-find shape included, whose
// no-ask silence the AskEmpty pins hold), and the confirm belongs to the
// search's own tail, searchShuffleTail below.
func shuffleLibrary(h Host, sa *cards.SA, owner state.PlayerID) {
	if strings.EqualFold(sa.Params["NoShuffle"], "True") || strings.EqualFold(sa.Params["Shuffle"], "False") {
		return
	}
	shuffleLibraryOrder(h, owner)
}

// searchShuffleTail is a hidden-library search's shuffle-and-place tail, with
// the ShuffleNonMandatory$ read (Path to Exile, Stoneforge Mystic, Squadron
// Hawk, Boggart Harbinger -- 209 exact-Origin$ Library corpus lines carry the
// flag). When the flag is set AND the search moved at least one card, the
// searcher is offered Forge's may-shuffle confirm -- "Shuffle your
// library?" -- instead of the unconditional shuffle: declining keeps the
// library order the search's option list (offered in library order) just
// taught them. A search that moved NOTHING -- the fail-to-find shape -- keeps
// the mandatory shuffle and asks nothing: that shape's no-ask silence is the
// AskEmpty contract's pinned resolution (Squadron Hawk's live soft-lock), and
// nothing was taken from the order the confirm would protect.
//
// The confirm suspends the resolution after the moves: the answer re-enters
// through the "search_mayshuffle" resume arm (rules' resumeResolution), which
// restores the answer into Ctx.SearchShuffle and the moved list into
// Ctx.SearchShuffleMoved (ridden on the ask via Decision.ResumeMoved, the
// same runtime-continuation class as ResumeRemembered -- the re-entry's
// LibraryPosition$ placement needs the list the suspension lost). The
// re-entered effSearchLibrary consumes both at its top and finishes here:
// "yes" emits the same Secret events.Shuffle every library shuffle emits,
// "no" keeps the order, and either way placeLibraryObjects runs after the
// shuffle point exactly as the unconditional path ordered it. A host that
// cannot ask takes the deterministic no-host stand-in (R-9): decline, keep
// the order -- the same stand-in the arrange_mayshuffle confirm falls back
// to. Returns true when the confirm suspended the resolution (the caller
// must stop; the re-entry owns the tail), false when the tail completed.
func searchShuffleTail(h Host, c *Ctx, sa *cards.SA, owner state.PlayerID, moved []state.ObjID, to state.Zone) bool {
	if c.SearchShuffle != "" {
		// Re-entry after the answered confirm: the moves happened in the
		// first pass, so this pass places only. Consume and clear before
		// continuing (fx42 scoping), so a nested search poses its own confirm.
		ans, placed := c.SearchShuffle, c.SearchShuffleMoved
		c.SearchShuffle, c.SearchShuffleMoved = "", nil
		if ans == "yes" {
			shuffleLibraryOrder(h, owner)
		}
		placeLibraryObjects(h, sa, owner, placed, to)
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(sa.Params["ShuffleNonMandatory"]), "True") || len(moved) == 0 {
		shuffleLibrary(h, sa, owner)
		placeLibraryObjects(h, sa, owner, moved, to)
		return false
	}
	d := &decision.Decision{Player: owner, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: c.Source, ResumeKind: "search_mayshuffle", ResumeSA: sa,
		ResumeRemembered: copyTargets(c.Remembered),
		ResumeMoved:      append([]state.ObjID(nil), moved...),
		Prompt:           "Shuffle your library?",
		Options: []decision.Option{
			{Index: 0, Kind: "yes", Label: "Yes — shuffle", Player: owner},
			{Index: 1, Kind: "no", Label: "No — keep the order", Player: owner},
		}}
	if Ask(h, d) == AskAsked {
		return true // suspended; the answer re-enters with Ctx.SearchShuffle set.
	}
	// No-host stand-in (R-9): decline the shuffle, keep the order.
	placeLibraryObjects(h, sa, owner, moved, to)
	return false
}

func shuffleLibraryExplicit(h Host, sa *cards.SA, owner state.PlayerID) {
	if strings.EqualFold(sa.Params["Shuffle"], "True") &&
		!strings.EqualFold(sa.Params["NoShuffle"], "True") {
		shuffleLibraryOrder(h, owner)
	}
}

func shuffleLibraryOrder(h Host, owner state.PlayerID) {
	order := h.ShuffleLibrary(owner, h.Game().Zone(state.ZLibrary, owner))
	h.Emit(events.Event{Kind: events.Shuffle, Player: owner, IDs: order, Secret: true})
}

// placeLibraryObjects implements LibraryPosition$ after its source library
// was shuffled. It is shared by a searched subset and a Defined$ fetch list.
// Reorder$ True (Goblin Recruiter's "put those cards on top in any order",
// Brainstorm's put-back) is the marker that the ANSWER order is the
// placement order: the branch below pins the chosen cards on top in exactly
// the order the player's answer carried them (libraryOrderPlacement), never
// a re-sorted one.
func placeLibraryObjects(h Host, sa *cards.SA, owner state.PlayerID, moved []state.ObjID, to state.Zone) {
	if strings.EqualFold(strings.TrimSpace(sa.Params["Reorder"]), "True") && to == state.ZLibrary {
		position := strings.TrimSpace(sa.Params["LibraryPosition"])
		if len(moved) > 0 && (position == "0" || position == "-1") {
			libraryOrderPlacement(h, owner, moved, position == "-1")
		}
		return
	}
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	if to != state.ZLibrary || len(moved) == 0 || (position != "0" && position != "-1") {
		return
	}
	libraryOrderPlacement(h, owner, moved, position == "-1")
}

// libraryOrderPlacement is the one LibraryPosition$ placement both
// hidden-origin movers (the library search's tutor-back and the hand
// put-back) share. The moved cards are already in the library (Move appended
// them at the bottom, in settle order); this one Secret LibraryOrder makes
// position exact: bottom=false puts the chosen cards on TOP in chosen order,
// bottom=true leaves them at the bottom in that same order, with the rest of
// the library beneath/above them respectively. Secret so the full order is
// visible only to the library's owner (redaction rule (1)).
func libraryOrderPlacement(h Host, owner state.PlayerID, moved []state.ObjID, bottom bool) {
	selected := make(map[state.ObjID]bool, len(moved))
	for _, id := range moved {
		selected[id] = true
	}
	lib := h.Game().Zone(state.ZLibrary, owner)
	rest := make([]state.ObjID, 0, len(lib)-len(moved))
	placed := make([]state.ObjID, 0, len(moved))
	for _, id := range moved {
		if containsID(lib, id) {
			placed = append(placed, id)
		}
	}
	for _, id := range lib {
		if !selected[id] {
			rest = append(rest, id)
		}
	}
	order := make([]state.ObjID, 0, len(lib))
	if !bottom {
		order = append(order, placed...)
		order = append(order, rest...)
	} else {
		order = append(order, rest...)
		order = append(order, placed...)
	}
	h.Emit(events.Event{Kind: events.LibraryOrder, Player: owner, IDs: order, Secret: true})
}

// changeZoneAllPlayers resolves the player scope Forge's ChangeZoneAllEffect
// applies. A card that says "exile all cards from target player's graveyard"
// (Bojuka Bog, Tormod's Crypt, Nihil Spellbomb, ...) names ONE player and must
// move only that player's cards; without this scope the effect swept every
// player's zones. Forge's rule:
//
//	if ((!sa.usesTargeting() && !sa.hasParam("Defined")) || UseAllOriginZones$ True)
//	    -> every player
//	else
//	    -> the chosen target players when the ability uses targeting, else the
//	       Defined$ players (getTargetPlayers; targeting wins when both ride)
//
// Both scope forms resolve through the shared player-target vocabulary
// (`Defined` / `definedSpec`), so `ValidTgts$ Player`, `ValidTgts$ Opponent`,
// `Defined$ You`, `Defined$ TargetedController` and the rest all work without a
// second spelling table. A selector we cannot resolve to a PLAYER (an unknown
// spelling, or an object-only one) keeps the pre-fix all-players sweep rather
// than silently moving nothing, and emits a Note saying so: the unscoped sweep
// is the previous behaviour, so an unmodelled card is never quietly inert, and
// a card we DO understand is correctly restricted. `Origin$` handling is not
// touched -- ParseZones already splits `Hand,Graveyard` into two zones and the
// Any/All wildcard is resolved by the caller before this runs.
func changeZoneAllPlayers(h Host, c *Ctx, sa *cards.SA) []state.PlayerID {
	g := h.Game()
	if strings.EqualFold(strings.TrimSpace(sa.Params["UseAllOriginZones"]), "True") {
		return g.AliveFrom(0)
	}
	_, targeting := sa.Params["ValidTgts"]
	_, defined := sa.Params["Defined"]
	if !targeting && !defined {
		return g.AliveFrom(0)
	}
	var chosen []state.Target
	if targeting {
		// Forge's getTargetPlayers reads the SA's ANSWERED target players when
		// it uses targeting, never Defined$; the pre-ask answered set outranks
		// the resolution list exactly as Defined's own targeting branch does.
		if c.PickedTargets != nil {
			chosen = c.PickedTargets
		} else {
			chosen = c.Targets
		}
	} else {
		chosen = Defined(h, c, sa)
	}
	seen := make(map[state.PlayerID]bool, len(chosen))
	out := make([]state.PlayerID, 0, len(chosen))
	for _, t := range chosen {
		if !t.IsPlayer || seen[t.Player] {
			continue
		}
		seen[t.Player] = true
		out = append(out, t.Player)
	}
	if len(out) == 0 {
		sel := sa.Params["Defined"]
		if targeting {
			sel = sa.Params["ValidTgts"]
		}
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "ChangeZoneAll could not resolve a player scope from " + sel + "; sweeping all players"})
		return g.AliveFrom(0)
	}
	return out
}

func effChangeZoneAll(h Host, c *Ctx, sa *cards.SA) {
	from, all, valid := ParseZones(sa.Params["Origin"])
	if !valid {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognised ChangeZoneAll Origin " + sa.Params["Origin"]})
		return
	}
	if all {
		from = []state.Zone{
			state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard,
			state.ZExile, state.ZStack, state.ZCommand, state.ZCeased,
		}
	}
	to := ParseZone(sa.Params["Destination"])
	spec := sa.Params["ChangeType"]
	if spec == "" {
		spec = "Card"
	}
	g := h.Game()
	// LibraryPosition$ (Terminus' "put all creatures on the bottom of their
	// owners' libraries") and Shuffle$ (Jace, the Mind Sculptor's [-12]
	// "shuffles their hand into their library", Gomazoa's "put on top ... then
	// those players shuffle") both act on the DESTINATION libraries, which are
	// each object's OWNER's library — a battlefield creature controlled by
	// another player (the Gomazoa / Vortex Elemental blocking shapes) still
	// returns to its owner's library, because the MoveZone keeps its owner.
	// The move loop therefore records every destination-library OWNER that had
	// a card moved (read off the object, not the source-zone player), in the
	// loop's own deterministic (zone-major, AliveFrom(0)-minor) order.
	position := strings.TrimSpace(sa.Params["LibraryPosition"])
	shuffle := strings.EqualFold(sa.Params["Shuffle"], "True")
	type ownerMoved struct {
		owner state.PlayerID
		ids   []state.ObjID
	}
	var placements []ownerMoved
	// AtEOT$'s affected set for ChangeZoneAll is the objects the sweep
	// actually moved, collected in move order.
	var moved []state.ObjID
	findOwnerMoved := func(owner state.PlayerID) *ownerMoved {
		for i := range placements {
			if placements[i].owner == owner {
				return &placements[i]
			}
		}
		placements = append(placements, ownerMoved{owner: owner})
		return &placements[len(placements)-1]
	}
	players := changeZoneAllPlayers(h, c, sa)
	// RandomOrder$ True (task mordorparams1, Gríma, Saruman's Footman's
	// "Then that player puts the exiled cards that weren't cast this way on
	// the bottom of their library in a random order"): the destination
	// placement order is a real shuffle, not the engine's scan order. The
	// cards are COLLECTED first (the same zone-major/seat-minor scan, no
	// emission), Fisher-Yates'd per destination-library owner through the
	// seeded engine rng (the randomChoices/Host.Rand precedent — a replay
	// re-derives the identical order), and only then emitted, so the
	// MoveZone appends settle in the shuffled order. The shuffle only sets
	// the MOVE ORDER; the LibraryPosition$/Shuffle$ tail below still applies
	// on top of it (Triumph of Saint Katherine's `LibraryPosition$ 0` after
	// a RandomOrder$ sweep puts the shuffled pile on TOP, not the bottom).
	// The Dig/RestRandomOrder$/RevealRandomOrder$ variants are their own rows
	// and are not touched here.
	randomOrder := strings.EqualFold(strings.TrimSpace(sa.Params["RandomOrder"]), "True")
	emitMove := func(id state.ObjID, z state.Zone, p state.PlayerID) {
		h.Emit(moveZoneEvent(c, id, z, to))
		moved = append(moved, id)
		// Tapped$ True (Splendid Reclamation's "Return all land cards
		// ... tapped"): a battlefield entry is followed by the same
		// "entered tapped" Tap event every other Tapped$ zone-change
		// path emits -- an entry state, not the CR 701.21a event of
		// becoming tapped.
		if to == state.ZBattlefield && strings.EqualFold(strings.TrimSpace(sa.Params["Tapped"]), "True") {
			h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: p, Text: "entered tapped"})
		}
		if to == state.ZExile {
			recordExileReturn(h, c, sa, id, z, to)
		}
		// GainControl$ hands the moved object to the named player
		// (Karn Liberated's ReturnFromExile, Cold Storage, Ghost
		// Vacuum). Only a battlefield entry can carry a control
		// change (CR 701.22a controls permanents), the same rule the
		// ChangeZone path applies; the shared resolver is loud rather
		// than silent on an unresolvable selector.
		if to == state.ZBattlefield {
			applyGainControl(h, c, sa, id)
			// StaticEffect$ (ChangeZoneAll's carriers -- Ghost Vacuum, Grimoire
			// of the Dead, Storm of Souls, Shilgengar): the same per-card rider
			// registration every other ChangeZone mover applies.
			applyStaticEffect(h, c, sa, to, []state.ObjID{id})
		}
		if to == state.ZLibrary {
			owner := p
			if o := g.Obj(id); o != nil {
				owner = o.Owner
			}
			findOwnerMoved(owner).ids = append(findOwnerMoved(owner).ids, id)
		}
		// ChangeZoneAll's remembered movement is needed for the
		// exiled-with-this-source cleanup/tally shape (Valakut
		// Exploration). Other ChangeZoneAll RememberChanged forms
		// remain outside this narrow provenance feature.
		if strings.EqualFold(sa.Params["RememberChanged"], "True") &&
			strings.Contains(sa.Params["ChangeType"], "ExiledWithSource") {
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
	}
	if randomOrder {
		type pendingMove struct {
			id state.ObjID
			z  state.Zone
			p  state.PlayerID
		}
		var owners []state.PlayerID
		byOwner := make(map[state.PlayerID][]pendingMove)
		for _, z := range from {
			for _, p := range players {
				// Snapshot the zone exactly like the emit loop does.
				ids := append([]state.ObjID(nil), g.Zone(z, p)...)
				for _, id := range ids {
					if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
						continue
					}
					owner := p
					if o := g.Obj(id); o != nil {
						owner = o.Owner
					}
					if _, seen := byOwner[owner]; !seen {
						owners = append(owners, owner)
					}
					byOwner[owner] = append(byOwner[owner], pendingMove{id: id, z: z, p: p})
				}
			}
		}
		for _, owner := range owners {
			list := byOwner[owner]
			for i := len(list) - 1; i > 0; i-- {
				j := h.Rand(i + 1)
				list[i], list[j] = list[j], list[i]
			}
			byOwner[owner] = list
		}
		for _, owner := range owners {
			for _, pm := range byOwner[owner] {
				emitMove(pm.id, pm.z, pm.p)
			}
		}
	} else {
		for _, z := range from {
			for _, p := range players {
				// Snapshot the zone: emitting move events mutates it underneath us.
				ids := append([]state.ObjID(nil), g.Zone(z, p)...)
				for _, id := range ids {
					if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
						emitMove(id, z, p)
					}
				}
			}
		}
	}
	// The post-placement tail runs for BOTH branches: the shuffled order is
	// only the ORDER the moves settle in, so `LibraryPosition$ 0` (Triumph of
	// Saint Katherine's "shuffle that pile and put it back on TOP of your
	// library") and `Shuffle$` must still apply after a RandomOrder$ sweep.
	if to == state.ZLibrary && len(placements) > 0 {
		// LibraryPosition$: MoveZone already appends at the bottom of the
		// destination library in settle order, so "-1" (Terminus) is exactly the
		// move order and needs no extra event; "0" pins the moved cards on TOP
		// via the one Secret LibraryOrder placement every library placement
		// shares. Any other value is loud rather than silently inert.
		switch position {
		case "", "-1":
		case "0":
			for _, pm := range placements {
				libraryOrderPlacement(h, pm.owner, pm.ids, false)
			}
		default:
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "LibraryPosition$ " + position + " is not implemented; the cards sit at the BOTTOM of their owners' libraries (the MoveZone append)"})
		}
		// Shuffle$ True shuffles each destination library that received a card,
		// AFTER the placement (Gomazoa's "put on top ..., then those players
		// shuffle" order), through the same Secret events.Shuffle every other
		// library shuffle emits.
		if shuffle {
			for _, pm := range placements {
				shuffleLibraryOrder(h, pm.owner)
			}
		}
	}
	scheduleAtEOT(h, c, sa, moved)
}

// effDestroy is a single-target removal effect: exactly the shape CR 608.2b
// target rechecking exists for. Today the only recheck is "does the target
// still exist, and is it still on the battlefield" -- a target that stayed on
// the battlefield but became newly ineligible some other way (e.g. it gained
// Indestructible in response, or protection from the source) between
// targeting and resolution is not rechecked. See the Task 18 report.
func effDestroy(h Host, c *Ctx, sa *cards.SA) {
	// Same pre-batch discipline as effDestroyAll: the targets Defined
	// resolves are destroyed as one simultaneous batch (a multi-target
	// Destroy over a lifelink Equipment and its bearer must not make the
	// bearer's LKI depend on battlefield order), so the snapshot covers all
	// of them before the first move.
	var victims []state.ObjID
	for _, t := range Defined(h, c, sa) {
		o := h.Game().Obj(t.Obj)
		if t.IsPlayer || o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if h.HasKeyword(o.ID, "Indestructible") {
			continue
		}
		victims = append(victims, o.ID)
	}
	if len(victims) > 0 {
		h.BatchDepartures(victims)
		defer h.EndBatchDepartures()
	}
	for _, id := range victims {
		o := h.Game().Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		// NoRegen$ is compared against "True", not against empty: an explicit
		// NoRegen$ False PERMITS regeneration, and reading it as "set, so
		// suppress" would invert the card. The corpus splits 144 True / 1
		// False (creepy_doll.txt), and that one is unreachable today because
		// cards/link.go auto-links only SubAbility$, not the WinSubAbility$ it
		// hangs off -- so this is correctness insurance for when that changes,
		// not a live fix.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
			continue
		}
		// Umbra armor (CR 702.90) applies even when NoRegen$ suppresses
		// regeneration — it is its own replacement, not a shield. Consuming
		// the Aura leaves it in the graveyard; when the loop reaches the Aura
		// itself (a DestroyAll that named it too) the zone guard above skips it.
		if ReplaceUmbraArmor(h, id) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
}

func effDestroyAll(h Host, c *Ctx, sa *cards.SA) {
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Permanent"
	}
	g := h.Game()
	remember := strings.EqualFold(strings.TrimSpace(sa.Params["RememberDestroyed"]), "True")
	// One pre-batch victim list across every player, then ONE departure
	// snapshot, then the emit loop (CR 704.3 simultaneity, as far as the
	// sequential emit model can express it): the CR 603.10a lifelink LKI a
	// later victim's departure capture reads must be the state from
	// immediately before the FIRST move -- a destroy-all over a
	// lifelink-granting Equipment and its bearer must not make the bearer's
	// own lifelink LKI depend on battlefield order.
	var victims []state.ObjID
	for _, p := range g.AliveFrom(0) {
		ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, p)...)
		for _, id := range ids {
			if h.HasKeyword(id, "Indestructible") {
				continue
			}
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				victims = append(victims, id)
			}
		}
	}
	if len(victims) > 0 {
		h.BatchDepartures(victims)
		defer h.EndBatchDepartures()
	}
	for _, id := range victims {
		if g.Obj(id) == nil || g.Obj(id).Zone != state.ZBattlefield {
			continue
		}
		// NoRegen$ != "True", not == "": see effDestroy's note above.
		if sa.Params["NoRegen"] != "True" && ReplaceDestruction(h, id) {
			continue
		}
		// Umbra armor after the shield: see effDestroy's note.
		if ReplaceUmbraArmor(h, id) {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
		if remember {
			// Forge's RememberDestroyed$ adds each destroyed card to
			// the host's remembered list (Stench of Evil's RepeatEach
			// over DirectRemembered iterates exactly these).
			c.Remembered = append(c.Remembered, state.Target{Obj: id})
		}
	}
}

// effSacrifice moves permanents to the graveyard. Sacrifice ignores
// Indestructible: sacrificing is not destruction (CR 701.16), so no
// HasKeyword/Indestructible gate and no ReplaceDestruction/regeneration
// consultation -- a regenerated creature does not survive being sacrificed.
// Same CR 608.2b caveat as effDestroy: only existence-and-zone is rechecked.
//
// A sacrifice aimed at a PLAYER now asks that player (CR 701.21a: "its
// controller chooses one") through a real KChoose over their matching
// permanents: Amount$ (default 1) sizes the ask, Optional$ True makes it
// "may sacrifice" (Min 0), and a hand of fewer eligible permanents than
// Amount$ sacrifices everything it has without asking (there is no choice
// to record, the effDiscard TgtChoose strict-supersets rule). The answer
// re-enters this effect through ResumeKind "sacrifice" with Ctx.SacPicks
// set, one suspension per Defined$ target (the cursor mirrors effDig's
// per-library asks). An Optional$ ask whose no-host fallback runs takes the
// first Amount$ eligible permanents — the same pick the pre-ask engine made
// — so games that never reach a real player answer replay byte-identically
// up to the pick the answer names.
func effSacrifice(h Host, c *Ctx, sa *cards.SA) {
	// UnlessCost$ is handled by the shared unlessProceed gate in Resolve,
	// exactly as it is for every other API — including the Vexing Devil
	// damage-payment offer (UnlessCost$ DamageYou<N>, UnlessPayer$ Opponent,
	// UnlessSwitched$ True), whose "pay" is taking the damage. When the gate
	// consumed the resolution — an ask was posed (suspended), the answered
	// choice spared the permanent, or every opponent declined the offer —
	// this body does not run at all.
	g := h.Game()
	// SacValid$ narrows WHAT may be sacrificed ("Creature.nonToken",
	// "Artifact"). With no SacValid$ at all the default is "Permanent" (any
	// permanent). The older justification -- that the self-sacrifice and
	// at-end-of-step lines need "Permanent" because the object they sacrifice
	// may be an artifact, a land or a creature -- is empirically false: those
	// lines carry no Defined$ and no ValidTgts$, so Defined() resolves them
	// to the SOURCE object (effects/context.go) and they take the object-target
	// path below, where spec is never consulted at all. Measured at the corpus
	// pin (for the command, see the sc1b report): of 892 Sacrifice SAs, 328
	// carry no SacValid$; 327 of those resolve to an object (or an inherited
	// target) and never reach the default, and exactly one -- Expert-Level
	// Safe's DB$ Sacrifice | Defined$ You | ValidCard$ Card.Self -- reaches it.
	// So "Permanent" is a harmless default rather than a correct reading of
	// the corpus, and no player-targeted line carries SacValid$ Self. (That one
	// reachable line means its controller hands over whichever permanent sits
	// first in zone order -- for Expert-Level Safe, "this artifact" -- instead
	// of the no-op before this fix; see AGENTS.md.)
	spec := sa.Params["SacValid"]
	if spec == "" {
		spec = "Permanent"
	}
	// RememberSacrificed$ True drives the task's effect-driven sacrifice
	// capture: it makes effSacrifice record the LKI snapshot (power,
	// toughness, mana value) of each object it sacrifices, so a SubAbility$
	// chained after it can resolve Sacrificed$CardPower/CardManaCost/Amount
	// against what THIS ability just sacrificed. Without the flag nothing is
	// remembered -- and nothing is, because the flag is read nowhere else in
	// this package (the sacrifice_audit test only counts its occurrence), so
	// the absence is the conservative same-as-before no-op, not a regression.
	remember := sa.Params["RememberSacrificed"] != ""
	// rememberLKICapture captures the sacrificed object's LKI (before the
	// MoveZone resets its counters) into c.Sacrificed, when the flag asks it
	// to. Idempotent per call site; called exactly once per sacrificed object.
	rememberLKICapture := func(id state.ObjID) {
		if remember {
			c.Sacrificed = append(c.Sacrificed, state.SacrificedInfoOf(g, id))
			// Forge's RememberSacrificed$ also remembers the card, which is
			// what a following ConditionDefined$ Remembered, Remembered$Amount
			// or RememberedCard reads (Braids, Scapeshift, Victimize).
			c.Remembered = append(copyTargets(c.Remembered), state.Target{Obj: id})
			eventRemember(h, c, id)
		}
	}
	// fx42 scoping: capture and clear the answered per-player pick BEFORE the
	// target loop, so a nested sacrifice below this walk poses its own ask.
	// SacTarget identifies the exact target that asked: earlier targets
	// completed before suspension and must be skipped, that target consumes
	// the answer, and later targets pose their own asks (Dig's per-library
	// ask shape).
	sacAns := c.SacPicks
	sacDone := c.SacDone
	sacTarget := c.SacTarget
	c.SacPicks, c.SacDone, c.SacTarget = nil, false, 0
	amount := sacrificeAmount(h, c, sa)
	// An Amount$ of zero has no legal sacrifice and, crucially, no meaningful
	// answer. Do not produce a 0..0 KChoose merely because eligible cards
	// happen to exist (an Optional$ Amount$ X trigger with X=0 has this shape).
	if amount <= 0 {
		return
	}
	optional := sa.Params["Optional"] == "True"
	strict := optional && sa.Params["StrictAmount"] == "True"
	// Optional + StrictAmount is not a 0..Amount range: it is specifically
	// "none, or exactly Amount". The KModes answer is consumed below before a
	// possible exact-batch KChoose; keeping it separate prevents a partial
	// sacrifice from taking the card's "if you do" continuation.
	sacOptional, sacOptionalTarget := c.SacOptional, c.SacOptionalTarget
	c.SacOptional, c.SacOptionalTarget = "", 0
	who := Defined(h, c, sa)
	// A Sacrifice that names neither Defined$ nor ValidTgts$ but a SacValid$
	// other than itself is Forge's default Defined$ You: its controller
	// sacrifices a matching permanent (Braids's "you may sacrifice an
	// artifact, creature, ..."). Only a SacValid$ Self/Card.Self line (or no
	// SacValid$ at all) sacrifices the source object itself. Corpus: 66 such
	// lines, which previously sacrificed the source whatever its type.
	if _, targeted := sa.Params["ValidTgts"]; !targeted && strings.TrimSpace(sa.Params["Defined"]) == "" {
		if v := strings.TrimSpace(sa.Params["SacValid"]); v != "" && v != "Self" && v != "Card.Self" {
			who = []state.Target{{Player: c.Controller, IsPlayer: true}}
		}
	}
	for targetIndex, t := range who {
		if sacOptional != "" {
			if targetIndex < sacOptionalTarget {
				continue
			}
			if targetIndex == sacOptionalTarget && sacOptional == "decline" {
				continue
			}
		}
		if sacDone {
			// Re-entry after some target's ask suspended: earlier targets
			// completed on the first pass and must be skipped (re-running
			// them would sacrifice a second batch); the asking target
			// applies its answer; later targets fall through to the normal
			// paths below and pose their own asks (Dig's per-library shape).
			if targetIndex < sacTarget {
				continue
			}
			if targetIndex == sacTarget {
				if t.IsPlayer {
					// Sacrifice exactly the answered cards that still sit on
					// this player's battlefield (a zone check keeps a stray
					// answer from moving an object that left meanwhile), in
					// the player's answer order. One departure snapshot for
					// the whole answered batch (BatchDepartures).
					if len(sacAns) > 0 {
						h.BatchDepartures(sacAns)
						defer h.EndBatchDepartures()
					}
					for _, id := range sacAns {
						if o := g.Obj(id); o == nil || o.Zone != state.ZBattlefield {
							continue
						}
						rememberLKICapture(id)
						h.Emit(events.Sacrifice(id))
					}
				} else if len(sacAns) > 0 {
					// The object-optional ask's sole option was answered
					// "sacrifice it": the object was already zone-checked on
					// the first pass, but re-check here in case it moved.
					if o := g.Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
						rememberLKICapture(o.ID)
						h.Emit(events.Sacrifice(o.ID))
					}
				}
				continue
			}
		}
		if t.IsPlayer {
			// Bounds guard: g.Zone indexes g.zones[zoneIndex(z, p)] and
			// zoneIndex has no bounds check, so an out-of-range target-supplied
			// player id would panic with "index out of range" and halt the
			// table. Player targets normally come from askTarget or AliveFrom
			// and are bounded, but the package's idiom (see cardflow.go and
			// count.go) is not to trust a target blindly.
			if int(t.Player) >= len(g.Players) {
				continue
			}
			// The multi-permanent count defaults to `amount` -- 1 for an
			// ordinary Sacrifice line, or the Amount$ the primitive carries.
			// The Annihilator expansion's generated SA carries its own count
			// in its Annihilator$ marker (cards/keywords.go) and overrides it;
			// the two contexts never coincide in the corpus.
			n := amount
			if ann := sa.Params["Annihilator"]; ann != "" {
				if v, err := strconv.Atoi(ann); err == nil && v >= 0 {
					n = int32(v)
				}
			}
			ids := append([]state.ObjID(nil), g.Zone(state.ZBattlefield, t.Player)...)
			eligible := make([]state.ObjID, 0, len(ids))
			for _, id := range ids {
				if h.SacrificeBlocked(id, false) {
					// A CantSacrifice restriction (Call for Aid) or face static:
					// the permanent is not a sacrifice candidate at all — not
					// offered, never taken (Annihilator rides this same pool).
					continue
				}
				if MatchesSpecCtx(g, spec, id, c.SpecContext(t.Player)) {
					eligible = append(eligible, id)
				}
			}
			minv, maxv := int32(0), int32(0)
			ask := false
			// n is the deterministic/no-host batch. An optional strict batch
			// with too few eligible permanents cannot be paid partially, so it
			// starts at zero rather than falling through to the old first-N path.
			if optional {
				if strict {
					switch {
					case sacOptional == "sacrifice" && targetIndex == sacOptionalTarget:
						// The player accepted the first yes/no step. If there is a
						// genuine identity choice, ask for EXACTLY Amount; when every
						// eligible permanent is required, there is nothing left to ask.
						if int32(len(eligible)) > amount {
							ask = true
							minv, maxv = amount, amount
						}
					case int32(len(eligible)) >= amount:
						// KChoose can express a range but not the disjoint set
						// {0, Amount}, so ask yes/no first and only then (above)
						// choose the exact batch.
						d := &decision.Decision{Player: t.Player, Kind: decision.KModes,
							Min: 1, Max: 1, Source: c.Source, ResumeKind: "sacrifice_optional",
							ResumeSA: sa, ResumeTarget: targetIndex,
							Prompt: "Sacrifice " + strconv.Itoa(int(amount)) + " permanent(s)?",
							Options: []decision.Option{
								{Index: 0, Kind: "mode", Label: "Sacrifice " + strconv.Itoa(int(amount)) + " permanent(s)", Obj: c.Source, Player: t.Player},
								{Index: 1, Kind: "mode", Label: "Don't sacrifice", Obj: c.Source, Player: t.Player},
							}}
						if Ask(h, d) == AskAsked {
							return
						}
						// R-9 no-host fallback: preserve the old deterministic pick,
						// but only as a complete strict batch.
						h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: t.Player,
							Text: "sacrifices the first matching permanent(s) (no engine host to ask)", Secret: true})
					case int32(len(eligible)) < amount:
						n = 0
					}
				} else if len(eligible) > 0 {
					// A non-strict optional sacrifice permits any number through
					// Amount$, including none.
					ask = true
					maxv = amount
					if maxv > int32(len(eligible)) {
						maxv = int32(len(eligible))
					}
				}
			} else if int32(len(eligible)) > n {
				// Mandatory with a choice: exactly the batch count of the
				// eligible — Amount$, or the Annihilator$ marker's count when
				// the generated expansion carries one.
				ask = true
				minv, maxv = n, n
			}
			// (the remaining mandatory shape — eligible <= amount — sacrifices
			// everything eligible without asking: no choice to record, the
			// effDiscard TgtChoose strict-supersets rule.)
			if ask {
				d := &decision.Decision{Player: t.Player, Kind: decision.KChoose,
					Min:          int(minv),
					Max:          int(maxv),
					Source:       c.Source,
					ResumeKind:   "sacrifice",
					ResumeSA:     sa,
					ResumeTarget: targetIndex,
					Prompt:       sacrificePrompt(optional && !strict, maxv)}
				for _, id := range eligible {
					name := "a permanent"
					if o := g.Obj(id); o != nil && o.Face() != nil {
						name = o.Face().Name
					}
					d.Options = append(d.Options, decision.Option{Index: len(d.Options),
						Kind: "sacrifice", Label: name, Obj: id, Player: t.Player})
				}
				if Ask(h, d) == AskAsked {
					return // resolution suspended; the answer re-enters with Ctx.SacPicks set.
				}
				// Fuzz/no-engine host: the deterministic stand-in (R-9) keeps
				// the pre-ask behaviour — the first Amount$ eligible permanents
				// in zone order, so an Optional$ "may sacrifice" plays "do".
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: t.Player,
					Text: "sacrifices the first matching permanent(s) (no engine host to ask)", Secret: true})
				n = maxv
			}
			// One departure snapshot per emitted batch (BatchDepartures): a
			// sacrifice sweep over a lifelink-granting Equipment and its bearer
			// must not make the bearer's CR 603.10a lifelink LKI depend on
			// battlefield order. Asks suspend before any emission, so every
			// suspend-then-resume path still re-collects its batch here.
			batch := make([]state.ObjID, 0, n)
			for i := int32(0); i < n && int(i) < len(eligible); i++ {
				batch = append(batch, eligible[i])
			}
			if len(batch) > 0 {
				h.BatchDepartures(batch)
				defer h.EndBatchDepartures()
			}
			for _, id := range batch {
				rememberLKICapture(id)
				h.Emit(events.Sacrifice(id))
			}
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if h.SacrificeBlocked(o.ID, false) {
			// A CantSacrifice restriction (or face static): this specific
			// object cannot be sacrificed at all — neither offered to its
			// Optional$ ask nor emitted. The targeting already picked it; the
			// restriction is what stops the pick.
			continue
		}
		// A specific object target is sacrificed as-is: the choice of which
		// object was already made by the effect's targeting, so SacValid$'
		// "which one may be sacrificed" step does not re-filter a concrete
		// object (and would misfire on the corpus's SacValid$ Self lines,
		// where "Self" is not a type the filter grammar knows).
		//
		// Optional$ True on an object target is a real yes/no ("you may
		// sacrifice this artifact"): a 0..1 ask over the object, answered
		// through the same "sacrifice" resume. A host that cannot ask keeps
		// the mandatory sacrifice (the pre-ask behaviour).
		if optional {
			// A concrete target cannot satisfy a strict batch greater than one:
			// it may decline, but it must not sacrifice this one object as a
			// partial payment.
			if strict && amount != 1 {
				continue
			}
			d := &decision.Decision{Player: o.Controller, Kind: decision.KChoose,
				Min: 0, Max: 1, Source: c.Source,
				ResumeKind: "sacrifice", ResumeSA: sa, ResumeTarget: targetIndex,
				Prompt: sacrificePrompt(true, 1)}
			name := "a permanent"
			if o.Face() != nil {
				name = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: 0,
				Kind: "sacrifice", Label: name, Obj: o.ID, Player: o.Controller})
			if Ask(h, d) == AskAsked {
				return
			}
			// No-host stand-in: the mandatory sacrifice the pre-ask engine
			// made, with the Note that records why the richer path did not run.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: o.Controller,
				Text: "sacrifices the first matching permanent(s) (no engine host to ask)", Secret: true})
		}
		rememberLKICapture(o.ID)
		h.Emit(events.Sacrifice(o.ID))
	}
}

// ParseDamageUnlessCost reports whether an UnlessCost$ value is the
// damage-payment offer form "DamageYou<N>" and returns N. Recognised: the
// exact spelling (case-insensitive) with a non-negative integer N; anything
// else is not the offer (and falls to the shared gate's pricing).
func ParseDamageUnlessCost(cost string) (int, bool) {
	_, n, ok := strings.Cut(strings.TrimSpace(cost), "DamageYou<")
	if !ok || !strings.HasSuffix(n, ">") {
		return 0, false
	}
	n = strings.TrimSuffix(n, ">")
	v, err := strconv.Atoi(n)
	// N must be a POSITIVE literal: DamageYou<0> (no damage) is not an offer
	// anyone could answer differently, so it fails closed to the ordinary
	// pricing path like every other non-offer spelling.
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// sacrificePrompt renders the player-targeted sacrifice ask's prompt.
func sacrificePrompt(optional bool, n int32) string {
	if optional {
		return "Choose up to " + strconv.Itoa(int(n)) + " permanent(s) to sacrifice, or none"
	}
	return "Choose " + strconv.Itoa(int(n)) + " permanent(s) to sacrifice"
}

// sacrificeAmount resolves Amount$ (default 1). Literals pass through; a
// non-literal resolves through the count evaluator (an SVar name, an inline
// Count$ expression, or {X}). The "X" shape deserves its own arm: on a
// triggered ability Ctx.X is the ability object's own X -- zero, a trigger
// was never paid an X -- so an Amount$ X on a permanent's trigger (Meathook
// Massacre II's "each player sacrifices X creatures") must read the paid X
// off the SOURCE permanent, which CastInfo carried out of the cast onto the
// battlefield object. When that cast X is also zero but an SVar named X
// exists, the evaluator resolves it: Dralnu, Lich Lord's replacement body
// carries SVar:X:ReplaceCount$DamageAmount, naming the replaced event's own
// amount rather than any paid X. An Amount$ that is none of literal, SVar,
// Count$, Sacrificed$ or X is an unknown shape and keeps the
// pre-Amount$-reading behaviour (1) rather than degrading to zero.
func sacrificeAmount(h Host, c *Ctx, sa *cards.SA) int32 {
	raw, ok := sa.Params["Amount"]
	if !ok {
		return 1
	}
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n)
	}
	if raw == "X" {
		if c.X != 0 {
			return c.X
		}
		if o := h.Game().Obj(c.Source); o != nil && o.X != 0 {
			return o.X
		}
		if c.SVars != nil {
			// Dralnu, Lich Lord: Amount$ X with SVar:X:ReplaceCount$DamageAmount
			// — the damage-replacement body's count is the amount of the event
			// being replaced, which main's replacement machinery carries in
			// Ctx.ReplacementAmount and EvalCount's ReplaceCount$ head reads.
			if body, has := c.SVars["X"]; has {
				if v := EvalCount(h, c, body); v != 0 {
					return v
				}
			}
		}
		return 0
	}
	known := strings.HasPrefix(raw, "Count$") || strings.HasPrefix(raw, "Sacrificed$") ||
		strings.HasPrefix(raw, "TriggerCount$") || strings.HasPrefix(raw, "TriggerCountMax$")
	if !known && c.SVars != nil {
		_, known = c.SVars[raw]
	}
	v := Num(h, c, sa, "Amount", 0)
	if !known && v == 0 {
		return 1 // unknown shape: today's fixed-one behaviour, not a silent zero
	}
	return v
}

// changeZoneChosenTargets serves effChangeZone's object path the targets of a
// ValidTgts$-declared targeting when no ask has offered them yet. The ok
// return is NOT "targets were found" -- it is "use the returned set INSTEAD of
// Defined's own fallthrough": ok=true with a nil set means the ask was posed
// and SUSPENDED the resolution (the caller must return before moving
// anything), and the answered re-entry consumes Ctx.Choice here. Every other
// shape returns false and the caller keeps Defined's own behaviour
// (placement-chosen targets, Defined$-named objects, the source default).
//
// The ask never fires when the resolution already carries targets (the
// placement ask's answered set) or when the SA also carries Defined$ (an
// already-named fetch list is Forge's no-ask shape). Bounds come from
// TargetMin$/TargetMax$ through the ordinary Num grammar, clamped to the
// eligible count; Min == Max == 0 or an empty eligible set is no ask and no
// move. A host that cannot ask takes the deterministic first-max stand-in
// (R-9), which is exactly what botpolicy's clamp fallback answers with.
func changeZoneChosenTargets(h Host, c *Ctx, sa *cards.SA) ([]state.Target, bool) {
	if _, targeted := sa.Params["ValidTgts"]; !targeted ||
		strings.TrimSpace(sa.Params["Defined"]) != "" {
		return nil, false
	}
	if c.TargetsOffered {
		// The announcement ask offered THIS SA's targeting (rules sets the
		// marker on the ability/spell branch exactly for the resolving SA);
		// the chosen-zero election must not be re-asked here.
		return nil, false
	}
	if c.ChoiceDone {
		ans := c.Choice
		c.ChoiceDone, c.Choice = false, nil
		return ans, true
	}
	if len(c.Targets) > 0 {
		// The placement ask already offered this targeting; Defined's own
		// fallthrough reads it.
		return nil, false
	}
	chooser := c.Controller
	candidates := h.LegalTargets(chooser, c.Source, sa)
	min := Num(h, c, sa, "TargetMin", 1)
	max := Num(h, c, sa, "TargetMax", 1)
	if max > int32(len(candidates)) {
		max = int32(len(candidates))
	}
	if min > max {
		min = max
	}
	if min < 0 {
		min = 0
	}
	if max <= 0 {
		// Nothing eligible (or an explicitly zero bound): no ask, no move.
		return nil, false
	}
	return poseTargetsAsk(h, c, sa, chooser, candidates, min, max, "choice")
}
