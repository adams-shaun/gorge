// Event folds for zone movement: MoveZone/Draw/PutOnStack, library order, shuffles, search, explore, attach.
//
// Split out of engine/events/apply.go: code moved verbatim, no behaviour
// change. Each fold function is the body of the matching case in
// Apply (g, e) switch; see apply.go for the dispatch.
package events

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/state"
)

// foldMoveZone folds Kinds MoveZone, Draw, PutOnStack into state.
func foldMoveZone(g *state.Game, e *Event) {
	// CR 733.1 reverses a proposed cast with a real logged stack->origin
	// move. Preserve the entry history that preceded its stack proposal in
	// transient object state: a log-only replay sees the same PutOnStack,
	// captures the same fields and consumes them on the reverse move.
	wasStack := false
	// The object's REAL pre-move zone, not Event.From: Move itself treats
	// From as advisory (a malformed caller-supplied From must not corrupt
	// state), and the Ring-bearer clear below must follow the same rule --
	// otherwise a blob return/re-entry reusing the same ObjID could keep a
	// stale designation.
	wasBattlefield := false
	// The DrawnThisTurn stamp before the move: a CR 733.1 reversal
	// ("reversed" stack->origin move) undoes the proposal, so the card is
	// still the card that was drawn; Move's clear is reverted below.
	drawnBefore := int32(0)
	if o := g.Obj(e.Obj); o != nil {
		drawnBefore = o.DrawnTurn
		wasStack = o.Zone == state.ZStack
		wasBattlefield = o.Zone == state.ZBattlefield
		if e.To == state.ZStack {
			o.PreStackEntryThisTurn = o.EnteredThisTurn
			o.PreStackEntryFrom = o.EnteredFrom
			o.PreStackEnteredLen = len(g.Entered)
			o.PreStackZoneGen = o.ZoneGen
			o.HasPreStackEntry = true
		}
	}
	// A manifest's or cloak's face-down entry (CR 708.5) must be visible
	// INSIDE the Move below: Move's battlefield-entry grants read it (a
	// manifested planeswalker enters as a 2/2 creature with no loyalty
	// grant, a manifested Saga with no lore counter -- while face down it
	// is neither), so the marker folds onto the object before the move and
	// is re-asserted after it. A Counter value on the existing MoveZone
	// decode: no new event kind, no Event field change. The same marker
	// carries a ChangeZone FaceDown$ True entry's folded set type and
	// power/toughness (FaceDownSetType$/FaceDownPower$/FaceDownToughness$)
	// as an optional Counter payload; the bare marker is CR 708.5's plain
	// 2/2 creature. Cloak shares the face-down entry with a second Counter
	// value; only the cloak marker sets Cloaked, the state engine/rules/layers.go
	// and trigger_match.go read for the ward {2}.
	moveCounter, countersRemain := CountersRemainMovePayload(e.Counter)
	if !countersRemain {
		moveCounter = e.Counter
	}
	setType, fdPower, fdTough, fdHasPT, faceDownEntry := "", int32(0), int32(0), false, false
	manifested := false
	if e.Kind == MoveZone && e.To == state.ZBattlefield {
		switch {
		case moveCounter == CloakEntryCounter:
			faceDownEntry = true
		case moveCounter == ManifestEntryCounter:
			faceDownEntry, manifested = true, true
		default:
			setType, fdPower, fdTough, fdHasPT, faceDownEntry = FaceDownEntryFields(moveCounter)
		}
	}
	if faceDownEntry {
		if o := g.Obj(e.Obj); o != nil {
			o.FaceDown = true
			o.Cloaked = e.Counter == CloakEntryCounter
			// Manifested is the manifest procedure's own marker (CR 701.40),
			// never the Secret bit: a generic ChangeZone FaceDown$ True entry
			// from a hand or library is Secret too (Tezzeret, Cruel Machinist;
			// Cybership) but does not grant the CR 701.40b turn-up. This
			// provenance is what lets a manifested creature card be turned face
			// up for its mana cost later.
			o.Manifested = manifested
			o.FaceDownSetType = setType
			o.FaceDownPower = fdPower
			o.FaceDownToughness = fdTough
			o.FaceDownHasPT = fdHasPT
		}
	}
	// The departure's "was a creature" bit, read off the pre-move object
	// before Move resets it (see DepartureCreatureFlip).
	departed, wasCreature := false, false
	if e.Kind == MoveZone && wasBattlefield && e.To != state.ZBattlefield {
		if o := g.Obj(e.Obj); o != nil {
			departed = true
			wasCreature = DepartureDefaultCreature(o) != (e.Step == DepartureCreatureFlip)
		}
	}
	sacrificed := IsSacrifice(*e)
	if e.Kind == MoveZone && countersRemain {
		MoveCountersRemain(g, e.Obj, e.From, e.To)
	} else {
		Move(g, e.Obj, e.From, e.To)
	}
	if e.Kind == Draw && e.To == state.ZHand {
		// The DrawnThisTurn stamp (state.Object.DrawnTurn): the card
		// was drawn on this turn.
		if o := g.Obj(e.Obj); o != nil && o.Zone == state.ZHand {
			o.DrawnTurn = g.Turn
		}
	} else if wasStack && e.Text == "reversed" {
		if o := g.Obj(e.Obj); o != nil {
			o.DrawnTurn = drawnBefore
		}
	}
	if departed && wasCreature {
		for i := len(g.Entered) - 1; i >= 0; i-- {
			if g.Entered[i].Obj == e.Obj {
				if g.Entered[i].From == state.ZBattlefield {
					g.Entered[i].WasCreature = true
				}
				break
			}
		}
	}
	if sacrificed {
		// Stamp the sacrifice onto this move's own zone entry (the
		// latest one naming the object: a mutated pile's under-cards
		// append after it).
		for i := len(g.Entered) - 1; i >= 0; i-- {
			if g.Entered[i].Obj == e.Obj {
				g.Entered[i].Sacrificed = true
				break
			}
		}
	}
	if o := g.Obj(e.Obj); o != nil {
		// A WithMayLook$ look permission is a property of ONE face-down
		// exile, so every later move clears it: the "exiled_with_face_down_
		// maylook" branch below re-sets it for the exile that grants it.
		// Without this reset a card exiled face down a second time (or
		// otherwise moved) would keep a stale looker and leak its face.
		o.HasMayLook = false
		o.MayLookPlayer = 0
		// CR 702.104: the tribute designation (kw_tribute-expansion's
		// AlterAttribute "Tributed" fold) belongs to one battlefield stint.
		// Clear it on every move that is not a battlefield entry -- not just
		// on a departure -- so an entry a later replacement redirects off the
		// battlefield cannot leave a stale flag that suppresses the unpaid
		// trigger on a future entry.
		if e.To != state.ZBattlefield {
			o.Tributed = false
		}
		o.HasExiledBy = false
		o.ExiledBy = 0
		if e.Kind == MoveZone && e.To == state.ZBattlefield && !wasBattlefield {
			applyEntryCounterPairs(o, e.Pairs)
		}
		if e.To == state.ZStack && o.Face() != nil {
			o.StackKind, o.StackKindKnown = state.StackKindSpell, true
		}
		if e.Kind == PutOnStack {
			// Record the zone and caster of this cast on the object, so
			// rules' latestCastOrigin reads the field instead of scanning
			// the whole log backwards on every call. A later PutOnStack
			// overwrites it (latest cast wins). The fold is the one writer,
			// so a log-only replay derives the same pair.
			o.LatestCastFrom, o.LatestCastBy, o.HasLatestCast = e.From, e.Player, true
		} else if e.From != state.ZStack || e.To != state.ZBattlefield {
			// Any other zone change after the cast than the resolution's own
			// stack -> battlefield entry (a counter, a CR 733.1 reversal, a
			// blink, a death, a draw) makes a new object that was never cast
			// (CR 400.7), so its cast origin ends here -- the same rule
			// rules' castProvenanceEndedBy applies to the log reads.
			o.HasLatestCast = false
		}

		if e.To == state.ZStack && IsFaceDownEntry(moveCounter) {
			// CR 708.4: a face-down CAST's spell sits on the stack with no
			// name, no types and no abilities. The face-down entry marker
			// rides the PutOnStack (engine/rules/cast.go's pushCast), and this fold
			// keeps Object.FaceDown on the stack object so the view redacts
			// its printed identity from everyone but its controller, and so
			// the resolution entry (moveResolvedOffStack's re-carried marker)
			// can tell a face-down spell from an ordinary one. The cloak
			// marker is the Disguise entry's carrier (the ward {2} a
			// disguised creature has while face down rides the same state
			// bit the Cloak machinery reads). No ordinary PutOnStack or
			// stack-bound MoveZone carries an entry marker today, so every
			// unrelated cast folds exactly as before.
			o.ExiledWith = 0
			o.FaceDown = true
			o.Cloaked = moveCounter == CloakEntryCounter
			// A face-down CAST is the morph/megamorph/disguise procedure, not
			// a manifest: its turn-face-up cost is the keyword cost, so the
			// manifest provenance must not be set here.
			o.Manifested = false
			o.FaceDownSetType, o.FaceDownPower, o.FaceDownToughness, o.FaceDownHasPT = "", 0, 0, false
		} else if e.To == state.ZExile {
			foldExileExiler(g, o, e)
			switch moveCounter {
			case "exiled_with_face_down", "exiled_with_face_down_foretold":
				// A blind face-down exile (Necropotence's top-card exile; a
				// pre-cr-g64 Hideaway/foretold log): the exiling source rides
				// in Amount and no looker is recorded, so FaceDown is state a
				// later projection reads to withhold the face from everyone
				// (CR 406.3). The foretold variant also records the
				// designation after Move has reset a battlefield object's
				// cast flags.
				o.ExiledWith = state.ObjID(e.Amount)
				o.FaceDown = true
				if moveCounter == "exiled_with_face_down_foretold" {
					o.CastFlags |= state.FlagForetold
				}
			case "exiled_with_face_down_maylook", "exiled_with_face_down_maylook_foretold":
				// A may-look face-down exile (WithMayLook$, a Foretold$
				// designation, the Foretell action, Hideaway, a Dig take the
				// chooser looked at, a library search's find): FaceDown plus
				// the look permission that seat holds. This marker's own
				// layout puts the LOOKER in Amount and the exiling source in
				// IDs, so both survive replay with no new event kind and no
				// Event field change. MayLookPlayer replaces Object.Controller
				// as the privileged viewer (view/cardViews), so the card's
				// owner cannot read a face the owner never had the right to
				// look at.
				o.FaceDown = true
				o.MayLookPlayer = state.PlayerID(e.Amount)
				o.HasMayLook = true
				if len(e.IDs) > 0 {
					o.ExiledWith = e.IDs[0]
				} else {
					o.ExiledWith = 0
				}
				if moveCounter == "exiled_with_face_down_maylook_foretold" {
					o.CastFlags |= state.FlagForetold
				}
			case "exiled_with_face_down_exiler", "exiled_with_face_down_exiler_foretold":
				// A DefinedExiler$ face-down exile (Shared Fate): same layout
				// as the may-look marker -- the exiling player in Amount, the
				// exiling source in IDs[0] -- and the exiler is also the
				// looker, so the view admits them before any static is read.
				o.FaceDown = true
				o.ExiledBy = state.PlayerID(e.Amount)
				o.HasExiledBy = true
				o.MayLookPlayer = state.PlayerID(e.Amount)
				o.HasMayLook = true
				if len(e.IDs) > 0 {
					o.ExiledWith = e.IDs[0]
				} else {
					o.ExiledWith = 0
				}
				if moveCounter == "exiled_with_face_down_exiler_foretold" {
					o.CastFlags |= state.FlagForetold
				}
			case "face_down":
				// A bare ChangeZone FaceDown$ True exile (Tezzeret's
				// Reckoning): the card is put into exile face down WITHOUT
				// claiming an ExiledWith association -- the line never named
				// an exiling source, so none is invented. The default branch's
				// own ExiledWith handling below is deliberately bypassed.
				o.FaceDown = true
			case "exiled_with":
				o.ExiledWith = state.ObjID(e.Amount)
				o.FaceDown = false
			default:
				// MoveZone reserves its otherwise-unused IDs payload for the
				// source when an effect exiles a card (moveZoneEvent). This
				// provenance is event-derived, hence survives replay, and
				// clears as soon as the card leaves exile.
				if len(e.IDs) > 0 {
					o.ExiledWith = e.IDs[0]
				} else {
					o.ExiledWith = 0
				}
				o.FaceDown = false
			}
		} else if faceDownEntry {
			// CR 708.5: a face-down card stays state-face-down while it is on
			// the battlefield (the view redacts it to everyone but its
			// controller); leaving the battlefield clears it (CR 708.9)
			// through Move's own leave reset and the default branch.
			o.ExiledWith = 0
			o.FaceDown = true
			o.FaceDownSetType = setType
			o.FaceDownPower = fdPower
			o.FaceDownToughness = fdTough
			o.FaceDownHasPT = fdHasPT
			o.Cloaked = e.Counter == CloakEntryCounter
			o.Manifested = manifested
		} else {
			o.ExiledWith = 0
			o.FaceDown = false
		}
		// CR 400.7: remember which incarnation of the source exiled this card.
		o.ExiledWithInc = 0
		if o.ExiledWith != 0 {
			if src := g.Obj(o.ExiledWith); src != nil {
				o.ExiledWithInc = src.Incarnation + 1
			}
		}
		if e.Text == "reversed" && o.HasPreStackEntry {
			// The reversal undoes the cast proposal (CR 733.1): the card
			// keeps the identity it had before the push.
			o.ZoneGen = o.PreStackZoneGen
			o.EnteredThisTurn = o.PreStackEntryThisTurn
			o.EnteredFrom = o.PreStackEntryFrom
			if o.PreStackEnteredLen <= len(g.Entered) {
				g.Entered = g.Entered[:o.PreStackEnteredLen]
			}
			o.PreStackEntryThisTurn = false
			o.PreStackEntryFrom = state.ZLibrary
			o.PreStackEnteredLen = 0
			o.PreStackZoneGen = 0
			o.HasPreStackEntry = false
		} else if wasStack && e.To != state.ZStack {
			o.PreStackEntryThisTurn = false
			o.PreStackEntryFrom = state.ZLibrary
			o.PreStackEnteredLen = 0
			o.PreStackZoneGen = 0
			o.HasPreStackEntry = false
		}
	}
	// Source-dependent goads end as soon as their source leaves play.
	pruneGoads(g)
	// CR 400.7 / 701.54e: a Ring-bearer designation lives on a permanent
	// and requires the battlefield — the moment the object leaves, every
	// seat's designation naming it is gone (the next battlefield entry is
	// a new object and never inherits one).
	if wasBattlefield && e.To != state.ZBattlefield {
		clearRingBearers(g, e.Obj)
		// CR 702.157b: the suspected designation has the same shape -- it
		// ends the moment the permanent leaves the battlefield; a later
		// battlefield entry never inherits one. The Plotted designation
		// (CR 701.34) has the same end condition on a permanent (the
		// exile case is Move's own leaving-exile clear, the plot ACTION's
		// path).
		if o := g.Obj(e.Obj); o != nil {
			o.Suspected = false
			o.SaddledTurn = 0
			o.Monstrous = false
			o.Solved = false
			o.Renowned = false
			o.PlottedTurn = 0
			// CR 722.3a: the prepared designation lives on a battlefield
			// permanent; the exempted exile copy (CR 722.3c) ceases to be
			// castable because its PreparedSource no longer answers true.
			o.Prepared = false
			o.Harnessed = false
		}
		// The prepared exile copy's cessation exemption is linked to this
		// battlefield permanent. Once it leaves, retire that provenance in
		// the same replayed zone-change fold so the orphaned copy ceases.
		if g.PreparedSourcesLive() {
			for i := range g.Objs {
				cp := &g.Objs[i]
				if cp.IsCopy && cp.PreparedSource == e.Obj {
					cp.PreparedSource = 0
					g.ClearPreparedSource()
				}
			}
		}
	}
}

// foldLibraryOrder folds Kind LibraryOrder into state.
func foldLibraryOrder(g *state.Game, e *Event) {
	// A library-arranging effect (Ponder, later Scry/Surveil) set a
	// complete new order on a player's library. Mechanically identical to
	// Shuffle's SetZone (see the same defensive copy below -- never alias
	// the event's slice into game state), but a separate Kind on purpose
	// (Ruling J1): Shuffle means "randomised, the old order is gone" to
	// a decoder, while LibraryOrder means "the player chose a new order"
	// -- a scry is not a shuffle, and a log reader must be able to tell
	// them apart. Same totality stance as every case here: an invalid
	// Player is a no-op, never a panic.
	if validPlayer(g, e.Player) {
		g.SetZone(state.ZLibrary, e.Player, append([]state.ObjID(nil), e.IDs...))
	}
}

// foldShuffle folds Kind Shuffle into state.
func foldShuffle(g *state.Game, e *Event) {
	if validPlayer(g, e.Player) {
		g.SetZone(state.ZLibrary, e.Player, append([]state.ObjID(nil), e.IDs...))
	}
}

// foldAttach folds Kind Attach into state.
func foldAttach(g *state.Game, e *Event) {
	if o := g.Obj(e.Obj); o != nil {
		// Any Attach fold supersedes a queued bearer-departure detach: the
		// object is re-attached here (or detached by this fold's own
		// empty-IDs arm), so the pending publication must not fire.
		o.UnattachPending = false
		switch {
		case e.Text == "attach to player" && validPlayer(g, e.Player):
			moved := o.AttachedTo != 0 || !o.HasAttachedPlayer || o.AttachedPlayer != e.Player
			o.AttachedTo = 0
			o.AttachedPlayer, o.HasAttachedPlayer = e.Player, true
			if moved {
				restampLayer(g, o)
			}
		case len(e.IDs) == 0:
			o.AttachedTo, o.HasAttachedPlayer = 0, false
		case g.Obj(e.IDs[0]) != nil:
			moved := o.AttachedTo != e.IDs[0] || o.HasAttachedPlayer
			o.AttachedTo, o.HasAttachedPlayer = e.IDs[0], false
			if moved {
				restampLayer(g, o)
			}
			// A re-attach supersedes any earlier bearer: the object is
			// now "attached to" the new one, so a later "was attached
			// to X" read must not still name the old X (state.Object.
			// LastBearer's contract).
			o.LastBearer = 0
		}
	}
}

// restampLayer gives battlefield object o a new CR 613.7 layer timestamp: one
// Game.Clock tick, as an entry takes, written to LayerStamp only (Timestamp
// is the entry identity and stays). The callers are the three CR 613.7e-g
// events -- an attach to a different object or player (613.7e/701.3c), a
// turn face up or down (613.7f) and a transform (613.7g) -- each folded
// AFTER its own state change, so the new stamp is later than every effect
// that existed when the event happened. An object off the battlefield (a
// face chosen on the stack or in hand) keeps its stamp: its entry stamps it
// anyway. An Aura entering attached (rules' settleAuraEntry emits its Attach
// right after the entry fold) takes one more tick than its entry, as Forge's
// attachToEntity does.
func restampLayer(g *state.Game, o *state.Object) {
	if o.Zone != state.ZBattlefield {
		return
	}
	g.Clock++
	o.LayerStamp = g.Clock
}

// foldUnattached folds Kind Unattached into state.
func foldUnattached(g *state.Game, e *Event) {
	// CR 701.3b: Obj became unattached from the bearer on a path where Obj
	// itself stays on the battlefield (the attachmentSBAs detach arms and
	// the bestowed type switch). The fold is the same AttachedTo clear an
	// empty-IDs Attach makes; the Kind is distinct so Mode$ Attached keeps
	// ignoring a detach while Mode$ Unattached fires. IDs[0] is the former
	// bearer, which only the trigger matcher reads -- nothing about the
	// state fold depends on it.
	if o := g.Obj(e.Obj); o != nil {
		o.AttachedTo, o.HasAttachedPlayer = 0, false
		// An Unattached fold CONSUMES a queued bearer-departure detach
		// (attachmentSBAs' pending arm publishes exactly this kind for it)
		// or supersedes one another writer detached; either way the queue
		// is spent.
		o.UnattachPending = false
		// IDs[0] is the former bearer: the attachmentSBAs detach arms
		// carry it so a later trigger can still resolve "attached to
		// that creature" after the sweep cleared AttachedTo
		// (state.Object.LastBearer's contract). A zero carrier leaves
		// any earlier LastBearer standing -- the object was not
		// attached to a named permanent.
		if len(e.IDs) > 0 && e.IDs[0] != 0 {
			o.LastBearer = e.IDs[0]
		}
	}
}

// foldLandPlayed folds Kind LandPlayed into state.
func foldLandPlayed(g *state.Game, e *Event) {
	if validPlayer(g, e.Player) {
		g.Players[e.Player].LandsPlayed++
	}
}

// zone's order. That is deterministic and matches every other move.
func Move(g *state.Game, id state.ObjID, from, to state.Zone) {
	move(g, id, from, to, false)
}

// MoveCountersRemain folds a move whose departing permanent has the
// CountersRemain static. The marker is carried by the logged MoveZone event.
func MoveCountersRemain(g *state.Game, id state.ObjID, from, to state.Zone) {
	move(g, id, from, to, true)
}

// detachAttachmentsOnDeparture queues the CR 400.7 / CR 704.5m-n break of
// every attachment to the permanent id that is leaving the battlefield
// (called only from move's wasBattlefield-and-leaving arm, so the
// index-ordered g.Objs walk keeps the fold deterministic). It records the
// departed bearer as LastBearer -- the contract LKI readers of "objects that
// were attached to it" fall back to -- and marks each attachment
// UnattachPending for engine/rules/attach.go's attachmentSBAs to publish,
// but deliberately leaves the live AttachedTo link alone: the mid-resolution
// window between this move and the next SBA checkpoint must still show the
// attachment fastened (Murderous Spoils' StealEquip resolves the Equipment
// of the creature its Destroy leg just killed), and a stale link cannot leak
// across zones because every cross-zone reader zone-gates. Only
// object-attached links queue; an attachment to a PLAYER
// (HasAttachedPlayer) is untouched, and a re-Attach or the attachment's own
// Unattached fold supersedes or spends the queue.
func detachAttachmentsOnDeparture(g *state.Game, id state.ObjID) {
	for i := range g.Objs {
		other := &g.Objs[i]
		if other.AttachedTo == id {
			other.LastBearer = id
			other.UnattachPending = true
		}
	}
}

// breakQueuedAttachmentsOnEntry completes the CR 400.7 attachment break when
// the departed bearer RE-ENTERS the battlefield (a flicker's return leg, the
// entry arm of move). While the bearer was off the battlefield the queued
// attachment's live link was deliberately left standing (the Murderous
// Spoils / Witness Protection mid-chain window); once the bearer is back, the
// returned permanent is a NEW object (CR 400.7) and nothing is attached to
// it, so the link must break NOW rather than at the next SBA checkpoint:
// checkStateBased runs destroyLethalDamage before attachmentSBAs in a pass,
// and CR 704.3's SBAs are simultaneous -- a toughness-changing Aura whose
// stale link survived the re-entry would kill the returned creature it was
// never attached to. The walk is the same index-ordered g.Objs fold as the
// departure arm (deterministic, replay-safe), and it clears ONLY AttachedTo:
// UnattachPending and LastBearer stay standing so engine/rules/attach.go's
// pending arm -- which reads LastBearer, not AttachedTo -- still publishes
// the queued detach with its usual Unattached / graveyard MoveZone.
func breakQueuedAttachmentsOnEntry(g *state.Game, id state.ObjID) {
	for i := range g.Objs {
		other := &g.Objs[i]
		if other.UnattachPending && other.AttachedTo == id {
			other.AttachedTo = 0
		}
	}
}

func move(g *state.Game, id state.ObjID, from, to state.Zone, countersRemain bool) {
	o := g.Obj(id)
	if o == nil || !o.Zone.Valid() || !to.Valid() {
		return
	}
	// The object's real (pre-move) zone, captured before o.Zone is
	// overwritten below -- Task 4's X/CastFlags/Chosen* reset needs to know
	// whether this object is actually LEAVING the battlefield, not merely
	// where the caller-supplied from claims it came from (the same
	// real-zone-over-claimed-zone rule this function already applies to the
	// removal itself, a few lines below).
	enteredFrom := o.Zone
	leftController := o.Controller
	wasBattlefield := enteredFrom == state.ZBattlefield
	wasStack := enteredFrom == state.ZStack
	o.SplitFrom, o.SplitDep = 0, 0
	// The battlefield-departure ordinal: the split stamps below (a mutated
	// pile's under-card, a meld partner) record THIS departure's number, and
	// rules' CR 903.9a scan compares stamps against its watermark, so a
	// departure an earlier state-based check already consumed is never
	// re-offered. A RETURN to the battlefield is not a departure and leaves
	// the ordinal alone, so a blink (exile and back in one resolution) does
	// not invalidate the card's arrival in exile.
	if wasBattlefield && to != state.ZBattlefield {
		g.DepSeq++
	}
	// Forge's drawnThisTurn survives only the move onto the stack (a cast of
	// the drawn card); any other move makes the card a new object that was
	// not drawn. The Draw fold re-stamps it after this Move.
	// Stamp the remembered-list boundary for the CR 400.7 clear below: a
	// spell pushed onto the stack (MoveZone's Spell entry, a PutOnStack)
	// notes how much of its Remembered predates the push. Entries written
	// after the push are the cast's own memory (a craft material paid to
	// cast the spell, an exile its own cost performed); entries written
	// before it belong to an earlier stint of the same ObjID (a card
	// bounced and recast). Overwrites on every push, so an aborted cast
	// re-proposed after a CR 733.1 reversal stamps its own fresh
	// boundary. The battlefield-entry clear consumes it; the moves below
	// that end the cast without an entry (a counter, a reversal) clear it.
	if to == state.ZStack && enteredFrom != state.ZStack {
		o.RememberedStackBase = uint32(len(o.Remembered))
	}
	if to != state.ZStack {
		o.DrawnTurn = 0
		// The as-cast battlefield snapshot belongs to the spell on the stack:
		// any move off it (resolution, counter, fizzle, an aborted cast's
		// reversal) ends it, so a stable ObjID never reuses a stale freeze.
		o.CastBattlefield = nil
	}
	if enteredFrom == state.ZExile && to != state.ZExile {
		// Forge's exiledCards association is a zone relationship, not an
		// imprint. Once this object leaves exile it is a new object for that
		// association, even if a later effect exiles the same engine ObjID.
		// Do this inside Apply's Move fold so live play and log replay prune
		// every source's list identically. The ExileReturn list (ChangeZone's
		// Duration$ UntilHostLeavesPlay) prunes identically: a card that left
		// exile by any other path is no longer its exiler's business to return.
		for i := range g.Objs {
			g.Objs[i].ExiledCards = withoutObjID(g.Objs[i].ExiledCards, id)
			g.Objs[i].ExileReturn = withoutExileReturnObj(g.Objs[i].ExileReturn, id)
			// Cipher (CR 702.99): the encoded link is a zone relationship too.
			// Once the encoded card leaves exile -- cast, blinked, moved by any
			// effect -- it is no longer that creature's encoded card, so prune
			// it exactly as ExiledCards prunes, inside the event fold so live
			// play and log replay agree.
			g.Objs[i].EncodedCards = withoutObjID(g.Objs[i].EncodedCards, id)
		}
		// CR 701.34c: a plotted card is no longer plotted once it leaves
		// exile (cast from exile to the stack, or moved on by any effect), so
		// the free-cast permission cannot revive on a later return to exile.
		// The exile-departure clear is the one home for this: every leaving
		// path (MoveZone, Draw and PutOnStack all call Move) runs it.
		if o := g.Obj(id); o != nil {
			o.PlottedTurn = 0
			// A granted suspend keyword is scoped to the exiled object; once it
			// leaves exile it is a new object for the grant's purposes.
			o.SuspendGranted = false
		}
	}
	if wasBattlefield && to != state.ZBattlefield && (g.BlockersLive() || ArenaSkipVerify) {
		// Leaving combat removes this permanent as a blocker, but does not
		// make creatures it blocked unblocked (CR 506.4, 509.1h). Preserve
		// each attacker's blocker-list length with the same zero tombstone
		// EndCombatReset uses for regeneration. Dense arena order keeps this
		// deterministic, and the departing object's own state is cleared by
		// the zone reset below. With no blocker recorded since the last
		// whole-combat reset (state.Game.BlockersLive) every list is empty
		// and the arena walk is skipped: without it a mass departure of N
		// permanents walked the arena N times.
		live := g.BlockersLive()
		for i := range g.Objs {
			other := &g.Objs[i]
			// The empty-list test first: it is the common case, and it reads
			// only the BlockedBy header instead of also touching the ID at
			// the other end of the ~800-byte object on every arena entry.
			if len(other.BlockedBy) == 0 || other.ID == id {
				continue
			}
			if !live {
				panic(fmt.Sprintf("events: obj %d has BlockedBy %v while no blocker is live (a BlockedBy write skipped state.Game.NoteBlockers)", other.ID, other.BlockedBy))
			}
			for j, blocker := range other.BlockedBy {
				if blocker == id {
					other.BlockedBy[j] = 0
				}
			}
		}
	}
	remove(g, id, o.Zone, zoneOwner(o, o.Zone))
	// CR 702.140e: when a mutated permanent leaves the battlefield, each card
	// merged beneath its top card moves to the same zone -- the pile is one
	// permanent, not a top plus orphans. The under-cards are parked in ZCeased
	// (no membership list) each with its own object, and this is the one site
	// that relocates them; a log-only replay runs the same Move. The pile
	// marker clears with the departure so a permanent that returns later
	// (CR 400.7) is a fresh, unmutated object. Recurse only into this
	// battlefield-boundary arm: a parked object's own Move has
	// wasBattlefield == false, so the walk cannot nest.
	if wasBattlefield && to != state.ZBattlefield && len(o.MergedCards) > 0 {
		merged := o.MergedCards
		o.MergedCards = nil
		o.TimesMutated = 0
		for _, mc := range merged {
			if mc.Obj != 0 {
				Move(g, mc.Obj, state.ZCeased, to)
				if m := g.Obj(mc.Obj); m != nil {
					m.SplitFrom, m.SplitDep = id, g.DepSeq
				}
			}
		}
	}
	// CR 400.7 / 704.5m-n: a permanent that leaves the battlefield is a new
	// object in its next zone, so nothing stays attached ACROSS the move.
	// The departure itself only QUEUES the break: each attachment to the
	// departing permanent is marked UnattachPending with the departed bearer
	// recorded as LastBearer, while its live AttachedTo link stays exactly
	// as it was. Keeping the link preserves the mid-resolution window
	// (before any SBA checkpoint) sub-abilities and "enchanted"-style
	// readers rely on: Murderous Spoils' StealEquip still resolves the
	// Equipment attached to the creature its Destroy leg just killed, and a
	// stale AttachedTo cannot leak across zones because every such reader
	// zone-gates (CR 303.4/400.7: an off-battlefield card is not enchanted).
	// The queue, not the clear, is what makes a flicker lose its attachments:
	// a flicker (exile, then return under the same ObjID) puts the bearer
	// back on the battlefield before the next SBA checkpoint runs, so
	// attachmentSBAs' bearer-left arm -- which reads the LIVE link -- could
	// never see the departure; the queued mark survives the round trip, the
	// bearer's re-entry breaks the live link (breakQueuedAttachmentsOnEntry:
	// the returned permanent is a new object nothing is attached to, and the
	// link must not survive to the SBA pass where destroyLethalDamage runs
	// before attachmentSBAs), and the pending arm publishes each queued
	// detach with the SAME Unattached
	// ("bearer left the battlefield" for an Equipment/Fortification,
	// "bestowed attachment ended" for a bestowed card) or the same graveyard
	// MoveZone ("attached to an object that left the battlefield" for an
	// Aura) the bearer-left arm emits, so Mode$ Unattached triggers (Grafted
	// Exoskeleton) and the CR 702.114b bestowed detach log keep firing
	// exactly as before.
	if wasBattlefield && to != state.ZBattlefield {
		detachAttachmentsOnDeparture(g, id)
	}
	// CR 712.4: a melded permanent that goes anywhere but the battlefield
	// splits back into its two cards (engine/events/apply_meld.go).
	if to != state.ZBattlefield && o.MeldedWith != 0 {
		unmeld(g, o, enteredFrom, to)
	}
	// CR 400.7: leaving the battlefield makes the object a new object in
	// its next zone, so control-changing effects do not follow it. Reset
	// before choosing the destination's zone owner: a later graveyard/hand
	// re-entry must be placed under its owner, not its former controller.
	// CR 702.25e: a phased-out permanent that leaves the battlefield phases
	// in as it does so -- the phased-out status is a property of that
	// battlefield object, and a later entry is a fresh, phased-in permanent.
	if wasBattlefield && to != state.ZBattlefield {
		o.Controller = o.Owner
		o.PhasedOut = false
		o.PhasedOutIndirect = false
	}
	if wasBattlefield && to != state.ZBattlefield {
		// CR 702.99: a creature's encoded cards are battlefield-stint state.
		// A creature that leaves the battlefield is a new object (CR 400.7),
		// and its encoded cards stay in exile un-encoded; the association is
		// dropped here so no later damage trigger reads a stale link.
		o.EncodedCards = nil
	}
	// CR 113.7a: an ability on the stack is not a card, and once it leaves
	// the stack it ceases to exist. The resolved/countered ability's move is
	// logged as stack->exile and o.Zone says exile (a historical shape every
	// golden replay and many pins carry: "the CR 608.2m exile parking"), but
	// the Face-less object never joins a zone's MEMBERSHIP list: an exile
	// walk (Oracle of Dust's "put a card an opponent owns from exile into
	// that player's graveyard" cost) moved such an object into a graveyard,
	// where Delve and an ExileFromGrave cost offered it as a card and
	// dereferenced its nil Face.
	ceasedAbility := to != state.ZStack && o.Card == nil && o.Ability != nil
	if to != state.ZCeased && !ceasedAbility {
		dst := zoneOwner(o, to)
		g.SetZone(to, dst, append(g.Zone(to, dst), id))
	}

	o.Zone = to
	// CR 709.4: a non-Room split card has no persistent "current half" once it
	// leaves the stack -- both halves are printed on the same physical card and
	// either is castable again from whatever zone it lands in. A split_alt (or
	// aftermath) cast flips the object to face 1 for the cast transaction; this
	// normalization resets it, or a card returned to hand would offer only the
	// half it was last cast as. Rooms are excluded (their face IS persistent
	// battlefield state) and so is a battlefield destination. Done inside
	// Apply's Move fold so live play and log replay normalize identically.
	if wasStack && to != state.ZBattlefield && o.Card != nil &&
		o.Card.AlternateMode == "Split" && len(o.Card.Faces) == 2 && int(o.FaceIdx) != 0 {
		room := false
		for _, f := range o.Card.Faces {
			if f != nil && f.IsRoom() {
				room = true
				break
			}
		}
		if !room {
			o.SetFaceIdx(0)
		}
	}
	// CR 715.4: off the stack an adventurer card has only its normal (main
	// face) characteristics, so an Adventure spell leaving the stack --
	// resolved into the adventure zone, countered, or reversed to hand -- is
	// its main face again. The adventure-zone recast reads the log, not the face.
	// CR 720.4: the same rule for an Omen card -- in every zone except the
	// stack, and while on the stack not as an Omen, it has only its normal
	// characteristics. So a resolved (now in the library), countered or
	// reversed Omen is its creature front again.
	if wasStack && to != state.ZBattlefield && o.Card != nil &&
		(o.Card.AlternateMode == "Adventure" || o.Card.AlternateMode == "Omen") && o.FaceIdx != 0 {
		o.SetFaceIdx(0)
	}
	// CR 712.4d: a Modal DFC is front-face up in every non-battlefield
	// zone. Its back face remains active while it is a permanent, but leaving
	// the battlefield creates a new object whose characteristics are the
	// front face. CR 712.8a: likewise a back-face spell leaving the stack
	// (resolved, countered or reversed) is the front face again, unless it
	// becomes a permanent. Keep this in the event fold so replay and live
	// play agree.
	if (wasBattlefield || wasStack) && to != state.ZBattlefield && o.Card != nil &&
		o.Card.AlternateMode == "Modal" && len(o.Card.Faces) == 2 &&
		o.Card.Faces[0] != nil && o.Card.Faces[1] != nil {
		o.SetFaceIdx(0)
	}
	// CR 712.8a / CR 712.14: a transforming double-faced card outside the
	// battlefield has only its front face's characteristics, and one put onto
	// the battlefield from a zone other than the stack enters front face up
	// unless the effect says "transformed" (effects.applyTransformed flips it
	// after this reset, before the entering move). So a DoubleFaced permanent
	// leaving the battlefield is front face up again; without this a Galian
	// Beast died into the graveyard and came back as Galian Beast. Same fold
	// site as the Modal reset above so replay and live play agree. Meld cards
	// reset in unmeld, not here.
	if wasBattlefield && to != state.ZBattlefield && o.Card != nil &&
		o.Card.IsTransformingDFC() && o.FaceIdx != 0 {
		o.SetFaceIdx(0)
	}
	// The incarnation stamp is used by promises tied to a particular
	// permanent (evoke/dash/warp), so only crossing the battlefield
	// boundary advances it. A provisional hand->stack->hand CR 733 reversal
	// must restore byte-identical state and is not a permanent incarnation.
	if enteredFrom != to && (enteredFrom == state.ZBattlefield || to == state.ZBattlefield) {
		o.Incarnation++
	}
	// CR 400.7: every REAL zone change makes a new object, whatever zones it
	// passed through (a CR 733.1 reversal below restores PreStackZoneGen: the
	// reversal undoes the proposal, it does not make a new object).
	if enteredFrom != to {
		o.ZoneGen++
	}
	// CR 400.7: a text change ends with the object it changed -- except
	// CR 400.7a, a spell's text change carries onto the permanent it becomes.
	if enteredFrom != to && len(o.TextChanges) > 0 && !(enteredFrom == state.ZStack && to == state.ZBattlefield) {
		o.SetTextChanges(nil)
	}
	// A new object in a new zone has its owner's default control. The old
	// controller is needed above to remove it from the battlefield/stack, so
	// reset only after removal and placement have used that zone ownership.
	if to != state.ZBattlefield && to != state.ZStack {
		o.Controller = o.Owner
	}
	// The stamped stack kind (state.StackKindKnown) is a property of STACK
	// MEMBERSHIP, not of the card: every mint stamps it when its event mints
	// the stack object (MoveZone's Spell entry, TriggerPush/AbilityPush and
	// their siblings), so an ability object keeps its kind after its source
	// has left (CR 113.7a) -- and leaving the stack (a CR 733.1 reversal's
	// MoveZone, a resolution, a counter) un-stamps it, so the restored object
	// is byte-identical with its pre-push state and the legacy re-derivation
	// in state.StackKindOf applies again. Inside the fold for the same reason
	// the Controller reset above is: no engine/rules/ or engine/effects/ caller can mint a
	// leaving-the-stack move that skips it.
	if wasStack && to != state.ZStack {
		o.StackKind, o.StackKindKnown = state.StackKindSpell, false
		// The cast ended without a battlefield entry (a counter, a CR 733.1
		// reversal), so the remembered-list boundary it stamped at the push
		// has nothing left to split. The resolution's own stack -> battlefield
		// entry keeps it until the CR 400.7 clear below consumes it.
		if to != state.ZBattlefield {
			o.RememberedStackBase = 0
		}
	}
	// A zone change is the single source of zone-entry provenance. Capture
	// the actual old zone (not Event.From, which Move deliberately treats as
	// advisory) so replay and a live game derive identical ThisTurnEntered*
	// state even from a malformed caller-supplied From.
	o.EnteredThisTurn = true
	o.EnteredFrom = enteredFrom
	if to == state.ZBattlefield && enteredFrom != state.ZBattlefield && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].IsRoom() {
		// A stack copy is put onto the stack, not cast (CR 707.10). It
		// resolves from ZStack too, but does not designate a cast door.
		o.CastDoor = enteredFrom == state.ZStack && !o.IsCopy
	}
	// Record the per-add entry the Count$ThisTurnEntered_* heads and the
	// ThisTurnEntered* filter predicates read (Forge's per-zone
	// getCardsAddedThisTurn lists, one append per add). Every Move routes
	// through events.Apply, so this stays inside the Apply-only mutation
	// discipline; the TurnChange case clears the list with the rest of the
	// per-turn state. A reversed CR 733.1 cast proposal keeps its entries:
	// the proposal's PutOnStack entry and the reverse move's entry both
	// land, and no corpus head reads the zones that pair touches
	// (Hand_from_Stack does, and the double entry it sees is the honest
	// record of the two moves).
	permanentCard := !o.IsToken && !o.IsCopy && o.Card != nil && o.Face() != nil && o.Face().IsPermanent()
	if !ceasedAbility {
		// A ceased ability is no zone entry: a Count$ThisTurnEntered_Exile
		// head counts cards put into exile, never retired abilities.
		g.Entered = append(g.Entered, state.ZoneEntry{Obj: id, To: to, From: enteredFrom,
			Owner: o.Owner, PermanentCard: permanentCard, Controller: leftController})
	}
	switch to {
	case state.ZBattlefield:
		o.SummonSick = true
		o.Damage = 0
		g.Clock++
		o.Timestamp = g.Clock
		o.LayerStamp = g.Clock
		// CR 707.10g: a copy of a permanent spell becomes a token. The pair
		// enteredFrom == ZStack + o.IsCopy uniquely identifies a StackCopy
		// mint resolving onto the battlefield (every other token mint routes
		// through a token script from a non-stack zone), and folding the flag
		// here inside Apply keeps a log-only replay byte-identical with no
		// new event. IsCopy is cleared at the same point: a resolved
		// permanent copy is a token, not a CR 707.10h "copy that left the
		// stack" (the clear also keeps the cast-provenance readers, which
		// gate on !IsCopy, reading a battlefield copy as never-cast). Note
		// this clear is NOT what makes the resolved copy visible any more:
		// state.Object.Ephemeral's IsCopy half is zone-aware and would show
		// a battlefield copy regardless, and engine/effects/filter.go's zone-aware
		// CR 707.10h guard already matched it as a real permanent. A copy of
		// an instant/sorcery never enters the battlefield, so its IsCopy and
		// its exile rest zone are untouched.
		if enteredFrom == state.ZStack && o.IsCopy {
			o.IsToken = true
			o.IsCopy = false
		}

		// CR 702.145b: a daybound permanent represented by a transforming
		// DFC that enters while it is night enters with its back face up.
		// Not a transform (no FlipFace, no "transforms into" trigger), so it
		// folds here as a pure function of the event-folded Game.DayNight.
		if !wasBattlefield && g.DayNight == state.DayNightNight && entersTransformedAtNight(o) {
			o.SetFaceIdx(1)
		}

		// CR 400.7: a battlefield entry from another zone is a new object and
		// a new control acquisition — kw:Echo's gate stamp (the entry already
		// carries the entering controller). A battlefield→battlefield stay is
		// not a new acquisition and must not re-stamp.
		//
		// The entry-characteristic COUNTERS (CR 306.5b starting loyalty, Riot's
		// and Unleash's +1/+1 election, a Saga's lore counter, a Battle's
		// defense counters) are deliberately NOT folded here. events.Move is a
		// pure state fold with no way to emit, so a counter folded here is
		// invisible to the CR 614 replacement pipeline and to the CantPutCounter
		// prohibition. rules snapshots events.EntryCounterGrants just before
		// this move folds and places each grant through a real CounterChange
		// event, so replacements and prohibitions see an entry counter exactly
		// like any other placement and a log-only replay re-derives it from
		// those logged events (task addcounter1/2). What stays here is only the
		// Riot "haste" election (a keyword grant, not a counter) and the
		// one-shot consumption of both elections.
		if !wasBattlefield {
			// The re-entry half of the CR 400.7/704.5m-n attachment break: a
			// bearer the departure arm queued against is back on the
			// battlefield as a new object, so its old attachments' links
			// break here, before the next SBA pass (see
			// breakQueuedAttachmentsOnEntry). First entries have nothing
			// queued against them and the walk finds no match.
			breakQueuedAttachmentsOnEntry(g, id)
			o.AcqTurn = g.Turn
			o.AcqStep = g.Step
			// CR 400.7: a battlefield entry from another zone is a new
			// object, so it inherits no attachment history. Without this a
			// permanent that left the battlefield and returned (blink,
			// reanimation, a bounced-and-replayed Equipment) would keep the
			// LastBearer of its prior incarnation, and a Defined$
			// Equipped|Enchanted read on the now-unattached object would
			// resolve to a creature it was never attached to. The leaving
			// arm above already preserved the old bearer as LastBearer for
			// LKI; this is its reset on the next entry.
			o.LastBearer = 0
			// A queued detach cannot survive an entry either: the queue was
			// either consumed before the departure or spent by the
			// departure's own fold.
			o.UnattachPending = false
			// Riot's choice is made before this entry. The "haste" half grants
			// a keyword (the "counter" half is placed by the engine's
			// EntryCounterGrants path); consuming the election here makes every
			// entry path obey the same logged choice.
			switch o.RiotChoice {
			case "haste":
				o.IntrinsicKeywords = append(o.IntrinsicKeywords, "Haste")
			}
			o.RiotChoice = ""
			// kw:Unleash's choice rides the same logged-then-consumed shape;
			// the "counter" half is placed through the engine's CounterChange
			// path, so only the consumption is left here.
			o.UnleashChoice = ""
			o.ReadAheadChapter = 0
			// CR 400.7: a battlefield RE-entry from another zone is a new
			// object with no memory of its former existence, so it carries no
			// remembered list from its previous incarnation. Without this, a
			// permanent that left the battlefield and returned (a
			// bounced-and-replayed or reanimated Curator of Destinies) would
			// answer a later `Card.IsRemembered` read with what an earlier
			// INCARNATION's resolution recorded, and that stale half of the
			// IsRemembered union would move cards the new object never
			// remembered. The leaving arm above deliberately keeps the list
			// for the old incarnation's pending referents (the O-Ring /
			// UntilHostLeavesPlay premise); this is its reset on the next
			// entry.
			//
			// Two entries are deliberately NOT the reset:
			//   - a first entry (Incarnation 1): an object that has never
			//     been a permanent has no former existence to forget, and the
			//     memory a spell or copy carried onto the battlefield is
			//     deliberate -- a spell's cast-time Craft material
			//     (rememberCraftMaterials, written before the resolving
			//     spell's entry) and a CopyPermanent token's inherited
			//     remembered set (TokenRemembered$, the Hofri Spirit) both
			//     enter through this arm and must survive it.
			//   - a Transformed$ True entry (CR 711.10a: the object enters
			//     back face up): Craft's `Transform this permanent` is
			//     CR 701.27, not a zone change, and this build renders that
			//     in-place transform as a self-exile plus a Transformed$
			//     re-entry (kwCraft's ChangeZone body). The transformed face
			//     is the SAME object continuing (CR 712.4), so the material
			//     its cast remembered is still its own. The leave arm above
			//     resets the face for every battlefield departure, so a back
			//     face at this point can only be a Transformed$ entry, never
			//     a prior incarnation's.
			// The boundary RememberedStackBase was stamped with at the
			// split's push splits a recast spell's list: entries the new
			// cast wrote after the push (its own paid cost's material) are
			// the new object's memory and survive, entries predating the
			// push belong to an earlier incarnation of the same ObjID and
			// die with the entry exactly as this block clears them for a
			// spell with no cast-time writes. A list shorter than the
			// boundary (a mid-cast-window clear of the old entries) has no
			// pre-push tail left to identify, so it falls to the full wipe
			// like every other entry. Any entry from a non-stack zone has
			// no boundary at all and wipes whole.
			if o.Incarnation > 1 && !(o.Card != nil && o.Card.IsTransformingDFC() && o.FaceIdx != 0) {
				if enteredFrom == state.ZStack && o.RememberedStackBase <= uint32(len(o.Remembered)) {
					o.Remembered = append(o.Remembered[:0], o.Remembered[o.RememberedStackBase:]...)
				} else {
					o.Remembered = nil
				}
			}
			o.RememberedStackBase = 0
			// CR 400.7: a battlefield entry is a new object, so the cards
			// the previous incarnation exiled are no longer its own -- except
			// where the object continues: a spell becoming its permanent, and
			// Craft's in-place transform (CR 702.167a), rendered as a
			// self-exile plus a Transformed$ re-entry. Those carry the links.
			craft := o.Card != nil && o.Card.IsTransformingDFC() && o.FaceIdx != 0
			if enteredFrom == state.ZStack || craft {
				prev := o.Incarnation - 1
				if craft && enteredFrom != state.ZStack {
					prev = o.Incarnation - 2
				}
				for i := range g.Objs {
					m := &g.Objs[i]
					if m.ExiledWith == id && m.ExiledWithInc == prev+1 {
						m.ExiledWithInc = o.Incarnation + 1
					}
				}
			} else {
				o.ExiledCards = nil
			}
		}
		// A face-down entry (a manifest) is a 2/2 creature with no abilities
		// (CR 708.5), so it grants none of the entry counters above; the
		// engine's EntryCounterGrants gate reads the same MoveZone face-down
		// marker this fold applies.
	default:
		// CR 702.103: a Soulbond pair ends when either member leaves the
		// battlefield. Move itself is the complete logged state transition, so
		// clear the remaining member here too; replay derives the same break
		// without a second event.
		if wasBattlefield && o.Paired != 0 {
			if partner := g.Obj(o.Paired); partner != nil && partner.Paired == o.ID {
				partner.Paired = 0
			}
		}
		// CR 400.7 / CR 702.122: the crew pairing the Creature.CrewedThisTurn
		// filter reads is battlefield-stint state on BOTH ends, and the crewing
		// creature's list is keyed by the Vehicle's stable ObjID. A Vehicle that
		// leaves the battlefield and returns in the same turn (blink, bounce) is
		// a NEW object (CR 400.7), but the old pairing would still match its id:
		// a trigger would target or affect a creature that never crewed THIS
		// object. Clear the departing permanent's id from every battlefield
		// object's CrewedVehicles -- the same derive-without-a-second-event sweep
		// the Soulbond break above does for its pairing. (The departing object's
		// OWN list is cleared below with the rest of its leaving-the-battlefield
		// state.) Totality: the id can appear at most once (the Crew case folds a
		// set), so the first hit is removed and the loop stops.
		if wasBattlefield && g.CrewedObjectsLive() {
			for i := range g.Objs {
				cr := &g.Objs[i] // a read: never copy the ~1 KB Object per arena slot
				if cr.ID == id || cr.Zone != state.ZBattlefield || len(cr.CrewedVehicles) == 0 {
					continue
				}
				for j, v := range cr.CrewedVehicles {
					if v == id {
						cr.CrewedVehicles = append(cr.CrewedVehicles[:j], cr.CrewedVehicles[j+1:]...)
						if len(cr.CrewedVehicles) == 0 {
							g.ClearCrewedObject()
						}
						break
					}
				}
			}
		}
		// Leaving the battlefield or the stack resets everything that only
		// exists while a permanent or spell is in play.
		o.Tapped = false
		o.Damage = 0
		o.ClassLevelValue = 0
		o.IsAttacking = false
		o.IsBlocking = false
		o.AttackingBattle = 0
		o.Band = 0
		o.BlockedBy = nil
		// CR 400.7: the this-turn block history is battlefield-stint state
		// too, so a creature that leaves and returns is a new object that
		// blocked/blocked-by nothing this turn.
		o.BlockedByThisTurn = nil
		o.BlockedThisTurn = nil
		// CR 400.7: likewise the records of who dealt THIS object damage
		// (the DamagedBy / wasDealtDamageByThisGame predicates' recipient
		// side). A creature dealt damage by Kumano's Pupils, bounced and
		// recast is a new object the Pupils never damaged. The source-side
		// DamageDealtThisTurn ledger stays: the damage was still dealt this
		// turn, and the turn-wide damage counts read it after the source
		// is gone.
		o.DamageTakenThisTurnBy = nil
		o.DamageTakenByGame = nil
		if !countersRemain || to == state.ZHand || to == state.ZLibrary {
			o.Counters = nil
		}
		o.IntrinsicKeywords = nil
		o.ExiledWith = 0
		o.HasExiledBy = false
		o.ExiledBy = 0
		o.FaceDown = false
		o.FaceDownSetType = ""
		o.FaceDownPower = 0
		o.FaceDownToughness = 0
		o.FaceDownHasPT = false
		o.Cloaked = false
		o.Manifested = false
		o.RiotChoice = ""
		o.UntapChoice = ""
		o.UnleashChoice = ""
		o.ReadAheadChapter = 0
		o.IsMyriad = false
		// CR 400.7: leaving the battlefield makes the object a new object, so
		// a layer-1 copy effect does not follow it. The ClonePermanent basis
		// is battlefield-only state and is cleared here (its continuous-effect
		// bookkeeping is dropped by active()/cleanup, since the effect's
		// source -- this same object -- is no longer on the battlefield).
		if o.CopyFace != nil {
			o.LeftCopyFace = o.CopyFace
		}
		o.SetCopyFace(nil)
		o.CopyGainThisAbility = false
		o.Paired = 0
		o.Targets = nil
		o.SubTargets = nil
		o.TargetStamps = nil
		o.Division = nil
		// o.Remembered is deliberately NOT reset on the DEPARTURE: a pending
		// referent read belongs to the object that queued it (CR 400.7), and
		// a leave trigger's body resolves only after the fold -- the source
		// has already left when the trigger is even matched, so the reset has
		// to wait for the next entry (below). Forge's host card list also
		// spans zone changes, which is the O-Ring premise and the reason its
		// ForgetOtherTargets$ exists at all (the deliberate clear on a
		// re-exile). A trigger body that resolves its remembered set through
		// this list does not read it live, though: its fire-time queue
		// (rules' triggerReferents) snapshots the list with an Incarnation
		// stamp, so a source that re-enters the battlefield before the
		// trigger resolves answers the pending read with the OLD object's
		// list (effects' sourceRemembered). The list stays event-backed
		// (Choose "remembered"/"clear-remembered"), so live play and replay
		// derive it identically either way.
		if wasBattlefield {
			// Room door designations belong to this battlefield incarnation.
			o.Unlocked = false
			o.CastDoor = false
			o.LockedDoors = 0
			o.Imprinted = nil
			o.ImprintTokens = nil
			o.SeekFound = nil
		}
		// X/CastFlags/Chosen* carry cast-time and choose-time information
		// forward from the stack onto the permanent it resolves into (an
		// ETB "if it was kicked" trigger needs to read X/CastFlags off the
		// permanent, not just the spell) -- so hand/stack -> battlefield
		// must NOT reset them, and they only reset once the permanent
		// genuinely leaves the battlefield again. AttachedTo has no legal
		// life off the battlefield at all (an Aura/Equipment that isn't a
		// permanent cannot be "attached"), so it always resets here
		// regardless of where the object came from.
		if wasBattlefield {
			o.X, o.CastFlags = 0, 0
			o.ReplicateTimes = 0
			o.SquadPaid = 0
			o.OffspringPaid = false
			o.OptionalCostPaid = false
			o.ConvergeColours = 0
			o.TimesKicked = 0
			o.Conspired = false
			o.TeamworkPaid = false
			o.Convoked = nil
			o.ManaSpentSources = nil
			o.ManaAddsCounterGrants = nil
			o.ManaAddsKeywordGrants = nil
			o.ManaSpent = 0
			o.ManaSnowSpent = 0
			o.ManaTreasureSpent = 0
			o.ManaCaveSpent = 0
			o.ManaDesertSpent = 0
			o.ManaArtifactSpent = 0
			o.ManaColorSpent = state.Mana{}
			o.CompleatedLifePaid = 0
			o.NotedNumber = 0
			// CR 400.7: the runtime SVar store is the old permanent's, not the
			// new object's -- a blunk/reanimated StoreSVar carrier starts with
			// no stored value (the printed default stands).
			o.RuntimeSVars = nil
			// CR 702.168: the gift promise's receiver is cast-time
			// provenance, not a battlefield characteristic -- a re-entering
			// permanent carries no promise from its old cast. The
			// FlagPromisedGift bit is cleared with CastFlags just above.
			o.GiftPromisedTo = 0
			o.ChosenName, o.ChosenType, o.ChosenNumber, o.ChosenColor = "", "", 0, ""
			o.ETBCloneChoice, o.ETBCloneChoiceValid = 0, false
			o.Protector, o.ProtectorValid = 0, false
			o.LastNotedMana = ""
			o.Chosen = nil
			// CR 400.7: leaving the battlefield makes the object a new object,
			// so a Charm's ChoiceRestriction$ ThisTurn picks -- battlefield-
			// stint state the source's own Charm reads -- do not follow it. A
			// permanent that leaves and returns (blink, reanimation) starts
			// with an empty log, even in the same turn.
			o.ModeChoices = nil
			o.CurCombatTurn, o.CurCombatCombat = 0, 0
			// Exert state is the old permanent's, not the new object's
			// (CR 400.7): a re-entering Combat Celebrant may exert again
			// this turn and carries no untap-skip window. The CR 611.2b
			// next-untap-step restriction (Frost Lynx's runtime keyword
			// grant) is the same kind of battlefield-stint state and is
			// cleared with it.
			o.ExertedThisTurn, o.ExertSkipUntap = false, false
			o.CantUntapNextStep = false
			// CR 702.160: enlist is the old permanent's fact, not the new
			// object's -- a re-entering creature carries no enlist stamp.
			o.EnlistedTurn, o.EnlistedCombat = 0, 0
			// A re-entering creature is a new object: it has not attacked this
			// combat (CR 400.7).
			o.AttackedTurn, o.AttackedCombat = 0, 0
			// CR 400.7 / 702.122: crew status is the old permanent's, not the
			// new object's -- a re-entering creature carries no crew stamp.
			if len(o.CrewedVehicles) != 0 {
				g.ClearCrewedObject()
			}
			o.CrewedVehicles, o.CrewedTurn = nil, 0
		}
		// CR 107.3m: the paid X belongs to the spell on the stack and to the
		// permanent the spell becomes, and to nothing else. An object leaving
		// the stack for a zone OTHER than the battlefield -- a countered or
		// fizzled spell into the graveyard, a resolving instant/sorcery -- is
		// a card in a non-battlefield zone, where X in its text is 0. Without
		// this the stale paid X rides along: a countered Genesis Hydra
		// reanimated later would resolve its ETB trigger with the dead cast's
		// X instead of 0. (A stack->battlefield move keeps X/CastFlags -- the
		// battlefield case above deliberately does not reset them, which is
		// what lets an ETB trigger read them off the permanent.)
		if wasStack {
			o.X, o.CastFlags = 0, 0
			o.ReplicateTimes = 0
			o.SquadPaid = 0
			o.OffspringPaid = false
			o.OptionalCostPaid = false
			o.ConvergeColours = 0
			o.TimesKicked = 0
			o.Conspired = false
			o.TeamworkPaid = false
			o.Convoked = nil
			o.ManaSpentSources = nil
			o.ManaAddsCounterGrants = nil
			o.ManaAddsKeywordGrants = nil
			o.ManaSpent = 0
			o.ManaSnowSpent = 0
			o.ManaTreasureSpent = 0
			o.ManaCaveSpent = 0
			o.ManaDesertSpent = 0
			o.ManaArtifactSpent = 0
			o.ManaColorSpent = state.Mana{}
			o.CompleatedLifePaid = 0
			o.NotedNumber = 0
			// CR 702.168: a spell leaving the stack for a non-battlefield zone
			// (a resolving instant/sorcery, a countered spell) names no gift
			// receiver further. The FlagPromisedGift bit is cleared with
			// CastFlags just above.
			o.GiftPromisedTo = 0
		}
		// ChosenModes is needed while a modal spell/ability resolves or for
		// the lifetime of a mode-chosen permanent (SetChosenMode$). A fresh
		// battlefield stint must choose again. An aborted cast restores its
		// captured prior value after the reverse stack move.
		if wasStack || wasBattlefield {
			o.ChosenModes = nil
		}
		// AttachedTo has no legal life off the battlefield at all (an Aura/
		// Equipment that isn't a permanent cannot be "attached"), so it always
		// resets here. The pre-clear bearer is preserved as LastBearer so a
		// trigger that resolves after the sweep can still resolve "objects
		// that were attached to it" (state.Object.LastBearer's contract).
		// When AttachedTo is already 0 (an earlier Unattached set it) the
		// existing LastBearer stands.
		if o.AttachedTo != 0 {
			o.LastBearer = o.AttachedTo
		}
		// Its own departure spends any queued bearer-departure detach: the
		// look-back arm of Mode$ Unattached (engine/rules/
		// trigmatch/unattached.go) already read this move's LKI snapshot,
		// which carried the queue, so nothing is left to publish.
		o.UnattachPending = false
		o.AttachedTo, o.HasAttachedPlayer = 0, false
	}
}

// changeControl gives o to controller p. The battlefield is keyed by
// controller (zoneOwner), so a permanent moves from its old controller's list
// to the new one's, the way Forge's controllerChangeZoneCorrection does;
// every per-controller reader (untap step, attackers, blockers, mana and
// activation offers, statics, projections) then sees it under its controller.
// The stack is one shared list, so a spell only changes its Controller.
//
// On the battlefield a control change also:
//   - removes the permanent from combat (CR 506.4): it stops attacking, loses
//     its blockers, and attackers it blocked keep a zero tombstone so they stay
//     blocked (CR 509.1h), exactly as a departing blocker does in Move;
//   - makes it summoning sick (CR 302.6): its new controller has not controlled
//     it continuously since their most recent turn began. TurnChange clears it
//     from the active player's list, i.e. at its new controller's next turn.
//
// planarWalkToOrder is the Defined$ planeswalk fold's destination move: each
// named destination plane is moved to the front of ids in the order named,
// and every plane not named keeps its relative order after them. A
// destination not present in the zone is skipped (a stale remembered card),
// and duplicates are placed once. The result is a permutation of the input,
// so the fold can never lose or duplicate a plane.
func planarWalkToOrder(ids, dests []state.ObjID) []state.ObjID {
	inZone := make(map[state.ObjID]bool, len(ids))
	for _, id := range ids {
		inZone[id] = true
	}
	out := make([]state.ObjID, 0, len(ids))
	placed := make(map[state.ObjID]bool, len(dests))
	for _, d := range dests {
		if !inZone[d] || placed[d] {
			continue
		}
		placed[d] = true
		out = append(out, d)
	}
	for _, id := range ids {
		if !placed[id] {
			out = append(out, id)
		}
	}
	return out
}

// withoutObjID returns ids without id, retaining its order and avoiding an
// allocation when no entry matches. ExiledCards is a short insertion-ordered
// relation, so an ordered slice preserves deterministic selector results.
func withoutObjID(ids []state.ObjID, id state.ObjID) []state.ObjID {
	for i, got := range ids {
		if got != id {
			continue
		}
		out := append([]state.ObjID(nil), ids[:i]...)
		for _, got := range ids[i:] {
			if got != id {
				out = append(out, got)
			}
		}
		return out
	}
	return ids
}

// withoutExileReturnObj drops every ExileReturn entry naming id, preserving
// the order of the survivors (the same withoutObjID contract for the
// entry-valued list).
func withoutExileReturnObj(entries []state.ExileReturnEntry, id state.ObjID) []state.ExileReturnEntry {
	for i, got := range entries {
		if got.Obj != id {
			continue
		}
		out := append([]state.ExileReturnEntry(nil), entries[:i]...)
		for _, got := range entries[i:] {
			if got.Obj != id {
				out = append(out, got)
			}
		}
		return out
	}
	return entries
}

// foldExileExiler folds the MarkExileExiler payload. The face-down exile markers
// override Counter and keep their own Amount layouts, so this no-ops for them;
// the DefinedExiler$ arm below still owns ExiledBy for those.
func foldExileExiler(g *state.Game, o *state.Object, e *Event) {
	if o == nil || e.Kind != MoveZone || e.To != state.ZExile || e.Counter != ExileExilerCounter {
		return
	}
	p := state.PlayerID(e.Amount)
	if !validPlayer(g, p) {
		return
	}
	o.ExiledBy, o.HasExiledBy = p, true
}
