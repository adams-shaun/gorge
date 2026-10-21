package view

import (
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Printing is the identity a client resolves an image by: the exact face
// name today. Set and Number stay empty until a printing table exists
// (roadmap open question 1); the fields are here so the wire shape does
// not change when it does.
type Printing struct {
	Name   string `json:"name"`
	Set    string `json:"set,omitempty"`
	Number string `json:"number,omitempty"`
}

// CardView is one object's public face: printed identity plus its current,
// derived characteristics. Nothing here is read from a hidden zone unless
// the viewer owns it — cardViews is only ever called with a zone list the
// caller has already decided is visible.
type CardView struct {
	ID   state.ObjID `json:"id"`
	Name string      `json:"name"`
	// CardName is the object's PRINTED card name -- its card's front face,
	// the name its genesis deck list (deck.Manifest) records -- when that
	// differs from Name: a transformed or back-face-played double-faced
	// card, a split or adventure half on the stack, a permanent a copy
	// effect (CopyFace) or a SetName$ renamed. Empty when it equals Name,
	// and on every redacted face-down CardView (the redaction rebuilds the
	// view from a fresh literal). The physical card's identity is exactly as
	// visible as the object carrying it: a copy or a transformed permanent
	// is a public card whose printed front every player can read. The
	// honest own-library composition (view.OwnLibrary) keys on it, so a
	// Clone copying one of the seat's own cards is counted as the Clone it
	// is, not as a second copy of what it copies.
	CardName string `json:"card_name,omitempty"`
	FaceDown bool   `json:"face_down,omitempty"`
	Types    string `json:"types"`
	// ManaCost is the printed cost in Forge's notation ("1 W", "R", "X G").
	// Hand lists render it as symbols.
	ManaCost string `json:"mana_cost,omitempty"`
	// Facts are Types' and ManaCost's compiled facts (cards.PrintedFacts),
	// set by the projection from the face it read them from, so an
	// in-process reader tests bits instead of re-parsing the text. Never on
	// the wire: a decoded or hand-built CardView has Facts.OK false and its
	// reader parses the strings.
	Facts CardFacts `json:"-"`
	// EffectiveManaCost is the offer-time cost to cast this card when the
	// engine can determine a non-X own-cost composition that differs from
	// the printed ManaCost. Empty means use ManaCost.
	EffectiveManaCost string `json:"effective_mana_cost,omitempty"`
	// SpellAPI is the API of the card's primary cast-shape ability (its
	// SP$ line -- "Counter" for Counterspell, "DealDamage" for Lightning
	// Bolt, "" for a card with no spell ability, which is every creature
	// and every activated-ability permanent). It is a printed card fact,
	// projected for exactly the cards the projection already carries the
	// printed ManaCost and Types of -- a visible card's own text is open
	// information -- so a seat that can read a card's cost can read what
	// the cast does at the same grain. The bot policy's casting rule reads
	// it (carried into botpolicy.Card.Counter) to tell a counter spell
	// from any other cast. A hidden hand's cards are never projected as
	// CardViews at all, so this never leaks hidden information.
	SpellAPI string `json:"spell_api,omitempty"`
	// RemovalKind names the one-shot removal shape of SpellAPI: "destroy",
	// "damage", or "bounce". Empty means the card is not one of the shapes
	// the cast policy holds. It is derived from the same printed ability
	// SpellAPI names, so it is a visible card fact, never hidden information.
	RemovalKind string `json:"removal_kind,omitempty"`
	// RemovalTargets is the coarse target scope of that same removal
	// ability, read off its ValidTgts$ alternatives' base tokens:
	// "creature" when they name creatures and no players, "player" when
	// they name players and no creatures, "any" when they name both, and
	// "" when they name neither (an artifact or land destruction spell, a
	// graveyard move). It is derived from the same printed ability as
	// RemovalKind and never set without it, so it is a visible card fact.
	// The bot policy's removal-now hold rule reads it to know whether a
	// spell could even aim at a creature (the threat gate) or at a player
	// (the lethal-face shortcut).
	RemovalTargets string `json:"removal_targets,omitempty"`
	// RemovalDamage is the literal NumDmg$ of that same removal ability
	// when it is a plain integer, 0 when it is absent, dynamic ({X}, an
	// SVar) or not positive. Like RemovalKind it is a printed card fact of
	// a visible card. The hold rule reads it as the spell's damage against
	// an opponent's projected life: a literal damage figure at or above a
	// player's life total is a lethal face target, so the removal is worth
	// firing now. A 0 answers "unknown", never "no damage" -- the policy
	// treats an unknown figure as no face candidate at all.
	RemovalDamage int32 `json:"removal_damage,omitempty"`
	// PumpPower and PumpToughness are the literal, non-negative bonus of
	// the card's primary Pump ability (effects.CastPumpBonus), both 0 for
	// every shape the cast policy does not read: a curse, a mixed-sign or
	// dynamic amount, a non-creature target, an opponent-only target
	// scope, or a non-Pump spell. They are printed facts of a visible
	// card, like RemovalDamage, and the opt-in A6 pump timing rule reads
	// them to decide whether a combat trick can change a combat. A 0
	// answers "not a literal creature pump", never "pumps for zero".
	PumpPower     int32 `json:"pump_power,omitempty"`
	PumpToughness int32 `json:"pump_toughness,omitempty"`
	// PumpOwn reports whether that Pump ability's target filter
	// can name both the caster's own attacking and blocking creatures
	// (effects.CastPumpOwnCombatants). It is true for an unrestricted
	// creature filter and false for a narrowed one -- attacking-only,
	// blocking-only, a colour/type/state restriction, or an opponent-only
	// scope. The A6 pump timing rule holds a literal pump it cannot aim at
	// its own combatants rather than casting it into the ordinary target
	// ranking. Only meaningful when PumpPower or PumpToughness is
	// positive; a false answer there means "not a pump the rule governs".
	PumpOwn bool `json:"pump_own_combatants,omitempty"`
	// CombatTrick reports whether the primary spell ability is a combat
	// trick (effects.CastCombatTrick).
	CombatTrick bool `json:"combat_trick,omitempty"`
	// PlayMain1 is the canonical Forge `SVar:PlayMain1` timing hint:
	// "ALWAYS", "TRUE" (cast in the first main while the caster controls a
	// creature) or "OPPONENTCREATURES" (while an opponent does), and "" for
	// no hint. It is a printed deck-builder hint of a visible card, never
	// hidden information. The opt-in AIHints cast rule reads it; the default
	// bot ignores it.
	PlayMain1 string `json:"play_main1,omitempty"`
	// RemoveDeck reports that the card carries an `AI:RemoveDeck` deck-builder
	// hint (Forge's "the AI does not know how to play this card"). It is a
	// printed fact of a visible card, projected so the opt-in AIHints cast
	// policy can veto it; the default bot ignores it.
	RemoveDeck bool `json:"remove_deck,omitempty"`
	// AuraLogic is the AttachAILogic hint ("Pump", "Curse", ...) of a card
	// whose spell is an Aura aimed at a creature, "" for every other card.
	// It is a printed deck-builder hint of a visible card. The opt-in A5
	// attach rules read it to hold a boon Aura while the caster controls no
	// creature that could use it.
	AuraLogic string `json:"aura_logic,omitempty"`
	// ColourIdentity is the card's colour IDENTITY mask (cards.ColourMask:
	// the printed cost's pips, the colour indicator and every colour-naming
	// mana symbol in the rules text -- CR 903.4's definition, the same one
	// cards.Face.ColourIdentity derives). It is a printed fact of a visible
	// card, like Types and ManaCost, and it is projected so a bot can read a
	// deck's colour splash from the lands it plays without re-parsing text.
	// Carried into botpolicy.Card.Colour; a face-down or unreadable card
	// projects zero, which is "identity unknown", never "colourless".
	ColourIdentity cards.ColourMask `json:"colour_identity,omitempty"`
	// Printing is what an image lookup keys on; Token ("#12") tells two
	// copies of one card apart in the stack, the log and an arrow.
	Printing Printing `json:"printing"`
	Token    string   `json:"token"`
	// IsToken and IsCopy are the object's own state.Object flags projected
	// for every visible card: a battlefield token (CR 111.7) and a copy
	// (CR 707.10; a battlefield token copy carries BOTH) are public identity
	// facts like the name they sit beside, and a client wire that mints its
	// own identity DTO reads them rather than re-deriving one from Token --
	// which is the display tag every card carries, never a token flag.
	// cardViews' face-down replacement rebuilds the CardView from a fresh
	// literal without them, so a card hidden behind its face reveals no more
	// than that projection already did; and since a token exists only on the
	// battlefield and a copy only on the stack or the battlefield, neither
	// flag can name a card in a hidden zone.
	IsToken bool `json:"is_token,omitempty"`
	IsCopy  bool `json:"is_copy,omitempty"`
	// AttackingPlayer is the seat this creature is attacking while
	// Attacking is true, nil otherwise; BlockedBy lists the creatures
	// blocking it. Both exist for the arrow overlay (PL-17) and come
	// straight from the object's combat fields, which EndCombatReset clears.
	AttackingPlayer *state.PlayerID `json:"attacking_player,omitempty"`
	BlockedBy       []state.ObjID   `json:"blocked_by,omitempty"`
	Tapped          bool            `json:"tapped"`
	Power           int32           `json:"power"`
	Toughness       int32           `json:"toughness"`
	// PrintedPT is the face's PRINTED power/toughness as text ("1/1", "*/*"),
	// the value the physical card or token is printed with. Power/Toughness
	// above are the CURRENT layer-7 values a counter or an anthem moves; a
	// token whose current P/T differs from its printed P/T (a 1/1 Soldier
	// with a +1/+1 counter reads "2/2") must still resolve to the same art as
	// its un-pumped sibling, and only the printed value is stable across
	// pumping. Empty for a face with no printed P/T (a noncreature) and on
	// every redacted face-down CardView, which is rebuilt from a fresh
	// literal.
	PrintedPT string           `json:"printed_pt,omitempty"`
	Damage    int32            `json:"damage"`
	Attacking bool             `json:"attacking"`
	Counters  map[string]int32 `json:"counters,omitempty"`
	Keywords  []string         `json:"keywords,omitempty"`
	// Controller and Owner can differ (a stolen permanent, a stack object
	// created for someone else's turn); a client needs both. Battlefield
	// zone lists are keyed by controller, every hidden/graveyard/exile list
	// by owner (engine/events/apply.go's zoneOwner), so a CardView carries both
	// regardless of which zone list it came from.
	Controller state.PlayerID `json:"controller"`
	Owner      state.PlayerID `json:"owner"`
	SummonSick bool           `json:"summon_sick"`
	// AttachedTo is the permanent this Aura or Equipment is currently
	// attached to, 0 meaning unattached -- the same zero-value convention
	// Obj uses, so omitempty keeps an unattached permanent (or a non-
	// permanent: a spell, a card in a hand) from emitting a field. It comes
	// straight from state.Object.AttachedTo, which the engine's Attach event
	// sets and clears; without it a client could not render an attachment
	// beneath the permanent it modifies -- could not tell what is attached
	// to what at all.
	AttachedTo state.ObjID `json:"attached_to,omitempty"`
	// ActivatedThisTurn is how many non-mana activated abilities of this
	// object were activated this turn (events.Apply's AbilityPush census on
	// state.Object.ActivatedThisTurn). Public fact, like the tap state it
	// rides beside; the bot policy's repeatable-ability budget (A5) reads
	// it to bound its own loop-shaped activations (Basalt Monolith's untap
	// re-enabling its own tap) without ever gating a human seat, for whom
	// unlimited activations stay legal and offered.
	ActivatedThisTurn int32 `json:"activated_this_turn,omitempty"`
	// AbilityCosts is the current offer-time Forge-notation cost of each
	// non-mana activated ability, in face ability order. Applicable
	// RaiseCost/ReduceCost statics have already been composed exactly as the
	// engine's legal-action gate composes them; this is deliberately not just
	// the printed Cost$. It is projected only for battlefield cards and the
	// viewer's own hand: those are the cards an interactive seat can use to
	// decide whether floating mana would unlock an ability. Other zones leave
	// it nil, rather than turning this into a general rules-text projection.
	AbilityCosts []string `json:"ability_costs,omitempty"`
	// Produces is what this card's mana abilities add to the pool when a
	// tap-for-mana activation runs them, derived from the compiled abilities
	// (cards.Face.ManaProduction) rather than land subtypes: a basic land's
	// intrinsic {W}, a dual's {W}{U}, an "add any colour" source's five
	// colour alternatives (one unit each, flagged Any -- the colour is chosen
	// when it is tapped, CR 106.1b), a colourless rock's {C}{C}. nil when the card has no mana
	// ability at all, so a creature or a spell never pays for the six-entry
	// array on the wire. It is a projected characteristic like ManaCost and
	// Keywords -- a mana ability's production is a card fact every seat sees,
	// and it is projected for the same zones the card itself is visible in.
	// This is what the bot policy's colour-aware tap and land heuristics read
	// (carried into botpolicy.Card as plain data, never the view type).
	Produces *cards.ManaProduction `json:"produces,omitempty"`
	// Flags are the permanent's status designations (CardFlag bits), straight
	// from the state.Object fields the engine folds from events: suspected,
	// renowned, monstrous, harnessed, solved, cloaked, the Ring-bearer, a
	// morph/disguise cast and the unlocked Room doors. Public facts like the
	// tap state; zero (omitted) on every ordinary card, so a view without any
	// of them serialises byte-identically to before the field existed.
	Flags CardFlag `json:"flags,omitempty"`
	// Imprinted lists the cards this permanent imprinted (state.Object.Imprinted,
	// CR 607.2a), Paired the creature it is soulbond-paired with (0 = none), and
	// ExiledCards the cards it exiled that are still in exile (the Oblivion
	// Ring link). They are bare object ids: the cards themselves are the
	// CardViews of whichever zone list they sit in, so a reader resolves them
	// there and a hidden card stays hidden. All omitted when empty.
	Imprinted   []state.ObjID `json:"imprinted,omitempty"`
	Paired      state.ObjID   `json:"paired,omitempty"`
	ExiledCards []state.ObjID `json:"exiled_cards,omitempty"`
	// AttachedToPlayer is true when this Aura/Curse is attached to the player
	// AttachedPlayer rather than to a permanent (AttachedTo stays 0); the
	// bool disambiguates seat 0 from "no player", the omitted zero.
	AttachedToPlayer bool           `json:"attached_to_player,omitempty"`
	AttachedPlayer   state.PlayerID `json:"attached_player,omitempty"`
}

// CardFacts are a CardView's compiled text facts. OK reports that the
// projection set them: PrintedFacts is then exactly
// cards.TypeLineMasks(Types) and cards.ManaCostPips(ManaCost).
type CardFacts struct {
	cards.PrintedFacts
	OK bool
}

// CardFlag is the CardView.Flags bit set.
type CardFlag uint16

const (
	FlagSuspected CardFlag = 1 << iota
	FlagRenowned
	FlagMonstrous
	FlagHarnessed
	FlagSolved
	FlagCloaked
	FlagRingBearer
	FlagMorphed
	FlagDisguised
	FlagLeftDoor
	FlagRightDoor
)

// Has reports whether every bit of m is set.
func (f CardFlag) Has(m CardFlag) bool { return f&m == m }

// cardFlags folds the object's status designations into a CardFlag set.
func cardFlags(g *state.Game, o *state.Object) CardFlag {
	var f CardFlag
	set := func(on bool, b CardFlag) {
		if on {
			f |= b
		}
	}
	set(o.Suspected, FlagSuspected)
	set(o.Renowned, FlagRenowned)
	set(o.Monstrous, FlagMonstrous)
	set(o.Harnessed, FlagHarnessed)
	set(o.Solved, FlagSolved)
	set(o.Cloaked, FlagCloaked)
	if int(o.Controller) < len(g.Players) {
		set(o.ID != 0 && g.Players[o.Controller].RingBearer == o.ID, FlagRingBearer)
	}
	set(o.CastFlags&(state.FlagMorphed|state.FlagMegamorphed) != 0, FlagMorphed)
	set(o.CastFlags&state.FlagDisguised != 0, FlagDisguised)
	if o.Zone == state.ZBattlefield {
		set(o.DoorUnlocked(0), FlagLeftDoor)
		set(o.DoorUnlocked(1), FlagRightDoor)
	}
	return f
}

// projector is one projection's read-only context: the game, its Chars, the
// viewer, and the optional Chars capabilities, probed once per projection
// instead of once per card. It lives on the projecting call's stack.
type projector struct {
	g      *state.Game
	ch     Chars
	viewer state.PlayerID
	// effective and namer are ch's optional SpellEffectiveCost and layer-3
	// Name capabilities (nil when ch lacks them); noTokens is a
	// tokenSuppressor's answer. namer is consulted only when vchars is
	// absent, and its empty string keeps the printed name: a bound-empty
	// name (a both-locked Room) needs vchars' flag.
	effective interface {
		SpellEffectiveCost(state.PlayerID, state.ObjID) string
	}
	namer interface{ Name(state.ObjID) string }
	// vchars is ch's optional one-call Name/Keywords/Power/Toughness
	// (rules.Engine.ViewCharacteristics), nil when ch lacks it. The second
	// return is the name's bound flag: when true, the (possibly empty) name
	// is authoritative and the printed face is not a fallback.
	vchars interface {
		ViewCharacteristics(state.ObjID) (string, bool, []string, int32, int32)
	}
	// vtypes is ch's optional derived type line (rules.Engine.ViewTypes):
	// ok only when a layer-4 effect changed the object's card types, nil
	// when ch lacks it (the printed line then stands).
	vtypes interface {
		ViewTypes(state.ObjID) ([]string, bool)
	}
	noTokens bool
	// hideHand is the CR 103.5 blind: true while the CR 103.1 play/draw
	// choice is unresolved, when no player may see any hand. It is probed
	// once from ch's optional StartingChoicePending capability
	// (rules.Engine); a Chars without it (a hand-built snapshot, a test
	// stand-in) never blinds, matching the R-9 no-ask host.
	hideHand bool
	// noDerived leaves every CardView's Chars-derived facts (layer-3 name,
	// keywords, P/T) unasked: a lean projection's OmitDerivedChars.
	noDerived bool
	// types and text are the projection's type-line cache and display-text
	// memo (projScratch).
	types *typeCache
	text  *textMemo
	// looker is ch's optional MayLookAt$ read over a face-down exiled card
	// (rules.Engine.MayLookAtFaceDown: a live Continuous MayLookAt$ static
	// whose Affected$ spec matches the card names that seat), nil when ch
	// lacks it -- the face is then shown only to the object's own looker.
	looker interface {
		MayLookAtFaceDown(state.PlayerID, state.ObjID) bool
	}
	// revealer is ch's optional "play with the top card revealed" read
	// (rules.Engine.LibraryTopRevealed, CR 401.5), nil when ch lacks it --
	// the top card is then shown only through MayLookAtLibraryTop.
	revealer interface {
		LibraryTopRevealed(state.PlayerID) bool
	}
}

func newProjector(g *state.Game, ch Chars, viewer state.PlayerID, sc *projScratch) projector {
	p := projector{g: g, ch: ch, viewer: viewer, types: &sc.types, text: &sc.text}
	p.effective, _ = ch.(interface {
		SpellEffectiveCost(state.PlayerID, state.ObjID) string
	})
	p.namer, _ = ch.(interface{ Name(state.ObjID) string })
	p.looker, _ = ch.(interface {
		MayLookAtFaceDown(state.PlayerID, state.ObjID) bool
	})
	p.revealer, _ = ch.(interface {
		LibraryTopRevealed(state.PlayerID) bool
	})
	p.vchars, _ = ch.(interface {
		ViewCharacteristics(state.ObjID) (string, bool, []string, int32, int32)
	})
	p.vtypes, _ = ch.(interface {
		ViewTypes(state.ObjID) ([]string, bool)
	})
	if s, ok := ch.(tokenSuppressor); ok && s.SuppressCardTokens() {
		p.noTokens = true
	}
	if s, ok := ch.(interface{ StartingChoicePending() bool }); ok && s.StartingChoicePending() {
		p.hideHand = true
	}
	return p
}

// seatSet is a set of seats (CR 723.4's alsoVisible widening), a bitset over
// every PlayerID value so a membership test allocates nothing.
type seatSet [4]uint64

// noSeats is the empty set, shared read-only.
var noSeats seatSet

func (s *seatSet) add(p state.PlayerID)      { s[p>>6] |= 1 << (p & 63) }
func (s *seatSet) has(p state.PlayerID) bool { return s[p>>6]&(1<<(p&63)) != 0 }

// refill returns buf emptied for refilling with up to n elements, reusing its
// backing array -- and, through the elements' own reusable fields (cardView),
// the storage each slot still holds -- when it is large enough. A short buf
// is replaced by one exactly n long when it had no storage (a fresh
// projection allocates what Project always has), or by one with doubled
// capacity whose slots carry buf's old ones. Never nil, even for n == 0, so a
// refilled list marshals "[]" the way a fresh one does (Ruling T23-u).
func refill[T any](buf []T, n int) []T {
	if cap(buf) >= n && buf != nil {
		return buf[:0]
	}
	c := n
	if cap(buf) > 0 {
		c = max(n, 2*cap(buf))
	}
	grown := make([]T, cap(buf), c)
	copy(grown, buf[:cap(buf)])
	return grown[:0]
}

// cardViews maps a zone's object ids to CardViews, in zone order, into buf's
// storage (refill). An id whose object no longer exists (a dangling entry, or
// a defensively tampered list) is skipped rather than producing a zero
// CardView or panicking (supplement §7). Always non-nil (Ruling T23-u), even
// for an empty or all-dangling ids: this is what lets the viewer's own
// genuinely empty Hand marshal "[]" rather than the same "null" a hidden hand
// would.
//
// Ephemeral objects (copies, tokens off the battlefield) have ceased to
// exist -- state.Object.Ephemeral is the single definition of that, consulted
// here rather than re-spelled inline so it cannot drift from any other call
// site -- and an ability object (Card == nil, so Face() == nil too) never
// legitimately sits in a card zone at all. Both are parked in exile by the
// engine and are skipped here (Task 4).
func (p *projector) cardViews(buf []CardView, ids []state.ObjID, includeAbilityCosts bool, abilityPlayer state.PlayerID, revealFaceDown bool, alsoVisible *seatSet) []CardView {
	out := refill(buf, len(ids))
	for _, id := range ids {
		n := len(out)
		out = out[:n+1]
		if !p.zoneCard(&out[n], id, includeAbilityCosts, abilityPlayer, revealFaceDown, alsoVisible) {
			out = out[:n]
		}
	}
	return out
}

// listViews is cardViews (never revealing face-down cards), or nil when skip
// (a lean projection's OmitCardLists).
func (p *projector) listViews(skip bool, buf []CardView, ids []state.ObjID, includeAbilityCosts bool, abilityPlayer state.PlayerID, alsoVisible *seatSet) []CardView {
	if skip {
		return nil
	}
	return p.cardViews(buf, ids, includeAbilityCosts, abilityPlayer, false, alsoVisible)
}

// zoneCard writes id's zone CardView into cv (cardViews' per-card body) and
// reports whether id projects at all; a skipped id leaves cv untouched.
func (p *projector) zoneCard(cv *CardView, id state.ObjID, includeAbilityCosts bool, abilityPlayer state.PlayerID, revealFaceDown bool, alsoVisible *seatSet) bool {
	o := p.g.Obj(id)
	// An ability object (no Face) is engine bookkeeping, not a card in
	// this zone; Ephemeral covers copies and tokens off the battlefield.
	if o == nil || o.Face() == nil || o.Ephemeral() {
		return false
	}
	if o.PhasedOut {
		// CR 702.25b: a phased-out permanent is treated as though it does
		// not exist, so it is absent from every projection of its zone.
		// PhasedOut is only ever set on a battlefield permanent (the
		// PhaseOut fold gates on it and the Move fold clears it), so this
		// cannot hide a card in a hidden zone.
		return false
	}
	// A face-down card is public as a distinct object but its face is
	// redacted from every viewer who may not look at it (p.mayLookFaceDown).
	// Secret on the original MoveZone was only event redaction and cannot
	// carry this lasting fact. A face-down planar-deck card is unknown to
	// every seat, including its owner; only the face-up current plane is
	// public.
	//
	// The redacted card is decided first and never built: none of the
	// derived facts the full card would read survives the blanking, so asking
	// Chars for them would only spend storage the blank card then drops.
	hidden := o.FaceDown && (o.Zone == state.ZPlanarDeck || (!revealFaceDown && !p.mayLookFaceDown(o, alsoVisible)))
	if hidden {
		*cv = CardView{ID: id, FaceDown: true, Token: p.token(id),
			Controller: o.Controller, Owner: o.Owner}
	} else {
		p.cardView(cv, id)
		if p.effective != nil {
			if cost := p.effective.SpellEffectiveCost(abilityPlayer, id); cost != "" && cost != cv.ManaCost {
				cv.EffectiveManaCost = cost
			}
		}
		cv.FaceDown = o.FaceDown
	}
	if includeAbilityCosts {
		if p.ch != nil {
			cv.AbilityCosts = p.ch.AbilityCosts(abilityPlayer, id)
		} else {
			cv.AbilityCosts = printedNonManaAbilityCosts(o.Face())
		}
	}
	return true
}

// mayLookFaceDown reports whether the viewer may see face-down o's face.
//
// On the battlefield and the stack the face is its controller's (CR 708.5,
// 708.4). In exile (CR 406.3: a card "exiled face down" can't be examined by
// any player except when instructions allow it) the face belongs to exactly
// the seats that may look -- never its owner by default (Necropotence's
// blind exile shows it to nobody):
//   - o.MayLookPlayer, the one looker the exiling instruction named
//     (WithMayLook$, a Foretold$ designation and the Foretell action -- CR
//     702.143a -- Hideaway, CR 702.75a, a looked-at Dig take and a library
//     search's find; events.Apply folds it from the
//     "exiled_with_face_down_maylook" marker);
//   - a seat a live MayLookAt$ static names (Praetor's Grasp's "you may look
//     at that card"), through ch's optional MayLookAtFaceDown.
//
// Either way CR 723.4 widens it to a seat controlling that looker (the
// alsoVisible set or a live control, controlsSeat). Forge's
// CardView.canBeShownTo is the same rule (cr-g64).
func (p *projector) mayLookFaceDown(o *state.Object, alsoVisible *seatSet) bool {
	if o.Zone != state.ZExile {
		return p.seesFor(o.Controller, alsoVisible)
	}
	if o.HasMayLook && p.seesFor(o.MayLookPlayer, alsoVisible) {
		return true
	}
	if p.looker == nil {
		return false
	}
	for s := range p.g.Players {
		if seat := state.PlayerID(s); p.seesFor(seat, alsoVisible) && p.looker.MayLookAtFaceDown(seat, o.ID) {
			return true
		}
	}
	return false
}

// seesFor reports whether the viewer sees what seat sees: it is seat, or
// controls seat (CR 723.4: the alsoVisible set, or a live control).
func (p *projector) seesFor(seat state.PlayerID, alsoVisible *seatSet) bool {
	return p.viewer == seat || alsoVisible.has(seat) || controlsSeat(p.g, p.viewer, seat)
}

// tokenSuppressor is a Chars whose caller discards every CardView.Token it
// projects (searchprobe's observation collector blanks them all before it
// compares or hashes a board), so cardView leaves the field empty instead of
// formatting a string per card. A Chars that does not implement it -- every
// ordinary caller, rules.Engine included -- gets the token.
type tokenSuppressor interface{ SuppressCardTokens() bool }

// token is id's display token ("#12"), or "" for a tokenSuppressor.
func (p *projector) token(id state.ObjID) string {
	if p.noTokens {
		return ""
	}
	return cardToken(id)
}

// tokenTableSize bounds the display tokens precomputed at init: "#0" through
// "#8191" live in one immutable string (tokenText, tokenEnds[i] the end of
// "#i"), so a projection slices its tokens out of it instead of formatting
// one string per card. Larger ids are formatted.
const tokenTableSize = 1 << 13

var tokenText, tokenEnds = buildTokenTable()

func buildTokenTable() (string, []uint32) {
	b := make([]byte, 0, tokenTableSize*6)
	ends := make([]uint32, tokenTableSize)
	for i := range ends {
		b = append(b, '#')
		b = strconv.AppendUint(b, uint64(i), 10)
		ends[i] = uint32(len(b))
	}
	return string(b), ends
}

// cardToken is id's display token, "#" and its decimal id.
func cardToken(id state.ObjID) string {
	if id < tokenTableSize {
		start := uint32(0)
		if id > 0 {
			start = tokenEnds[id-1]
		}
		return tokenText[start:tokenEnds[id]]
	}
	return "#" + strconv.FormatUint(uint64(id), 10)
}

// printedNonManaAbilityCosts is the no-rules fallback: it preserves the
// face's ability order while exposing only activated abilities that are not
// mana abilities. An absent Cost$ remains an empty string. A real Engine Chars
// supplies effective costs instead, including live cost modifiers.
func printedNonManaAbilityCosts(f *cards.Face) []string {
	var out []string
	for _, a := range f.Abilities {
		if a.Kind == "AB" && a.API != "Mana" {
			out = append(out, a.Params["Cost"])
		}
	}
	return out
}

// cardView writes one object's public face into cv. ch is read through
// Power/Toughness/Keywords rather than any printed field directly — the view
// asks the engine for derived characteristics, never the card's own text —
// and a nil ch (supplement §7) degrades to the zero P/T with no keywords.
//
// Every field of cv is rewritten. The storage cv already holds -- its
// Produces and AttackingPlayer structs, its BlockedBy and Keywords arrays,
// its Counters map -- is reused for the new values (ProjectInto's reuse), so
// it must be storage a previous projection made, never a slice or pointer
// anything else holds. A fresh (zero) cv allocates exactly what a new
// CardView needs.
func (p *projector) cardView(cv *CardView, id state.ObjID) {
	o := p.g.Obj(id)
	prod, atk, ctr, types := cv.Produces, cv.AttackingPlayer, cv.Counters, cv.Types
	blk, kws := cv.BlockedBy[:0], cv.Keywords[:0]
	*cv = CardView{
		ID: id, Tapped: o.Tapped, Damage: o.Damage, Attacking: o.IsAttacking,
		Controller: o.Controller, Owner: o.Owner, SummonSick: o.SummonSick,
		AttachedTo: o.AttachedTo, ActivatedThisTurn: o.ActivatedThisTurn,
	}
	cv.Token = p.token(id)
	cv.IsToken = o.IsToken
	cv.IsCopy = o.IsCopy
	// vchars answers the four derived facts below in one call; without it
	// each is asked for separately.
	var dName string
	var dNameSet bool
	var dKw []string
	if p.vchars != nil && !p.noDerived {
		dName, dNameSet, dKw, cv.Power, cv.Toughness = p.vchars.ViewCharacteristics(id)
	}
	if f := o.Face(); f != nil {
		cv.Name = f.Name
		// Name is a layer-3 characteristic. Keep the optional method so
		// lightweight Chars test doubles remain source-compatible while the
		// real rules engine exposes SetName$ results to clients. A bound
		// name is authoritative even when empty: a both-locked Room has no
		// name (CR 709.5) and must not fall back to its printed front face.
		if p.noDerived {
			// a lean projection's printed name
		} else if p.vchars != nil {
			if dNameSet || dName != "" {
				cv.Name = dName
			}
		} else if p.namer != nil {
			if name := p.namer.Name(id); name != "" {
				cv.Name = name
			}
		}
		if o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0] != nil {
			if printed := o.Card.Faces[0].Name; printed != cv.Name {
				cv.CardName = printed
			}
		}
		// Types is a layer-4 characteristic (CR 613.1d): a permanent whose
		// card types an effect changed (Imprisoned in the Moon makes a
		// creature a land and nothing else) shows the derived line, not its
		// printed one. Facts stay exactly TypeLineMasks(Types) and
		// ManaCostPips(ManaCost): the derived line's masks are recomputed,
		// the printed cost's pips kept.
		var derived []string
		hasDerived := false
		if !p.noDerived && p.vtypes != nil {
			derived, hasDerived = p.vtypes.ViewTypes(id)
		}
		if pf, line, ok := f.PrintedFacts(); ok {
			if hasDerived {
				cv.Types = p.typeLine(types, derived)
				pf.TypeFold, pf.TypeExact = cards.TypeLineMasks(cv.Types)
			} else {
				cv.Types = line
			}
			cv.Facts = CardFacts{PrintedFacts: pf, OK: true}
		} else if hasDerived {
			cv.Types = p.typeLine(types, derived)
		} else {
			cv.Types = p.typeLine(types, f.Types)
		}
		cv.ManaCost = f.ManaCost
		cv.PrintedPT = f.PT
		cv.Printing = Printing{Name: f.Name}
		// The colour identity (CR 903.4), a printed fact of the face. A
		// colourless face's zero mask is indistinguishable from an
		// unreadable card's, which is why the field is documented as
		// "identity unknown or colourless" -- the colour fold treats an
		// empty identity as no evidence, never as colourless evidence.
		cv.ColourIdentity = cards.ColourMask(f.ColourIdentity())
		cv.PlayMain1 = f.PlayMain1Hint()
		cv.RemoveDeck = f.RemoveDeck != ""
		// The spell-ability API projection: the card's own SP$ line, the
		// same fact boardFromGame reads for the casting census. Empty when
		// the card has no spell ability (a creature, a land, a permanent
		// with only activated abilities).
		if sa := f.SpellAbility(); sa != nil {
			cv.SpellAPI = sa.API
			cv.RemovalKind = effects.CastRemovalKind(sa)
			cv.RemovalTargets = effects.CastRemovalTargets(sa)
			cv.RemovalDamage = effects.CastRemovalDamage(sa)
			cv.PumpPower, cv.PumpToughness = effects.CastPumpBonus(sa)
			cv.PumpOwn = effects.CastPumpOwnCombatants(sa)
			cv.CombatTrick = effects.CastCombatTrick(sa)
			cv.AuraLogic = effects.CastAuraLogic(f)
		}
		// The mana-production projection (Task dp2): what tapping this card
		// puts in the pool, from its own abilities. A nil pointer keeps a card
		// with no mana ability off the wire; the struct is this CardView's own
		// (fresh, or the one it held before), so the address is never aliased
		// across object views.
		if mp := f.ManaProduction(); !mp.IsZero() {
			if prod == nil {
				prod = new(cards.ManaProduction)
			}
			*prod = mp
			cv.Produces = prod
		}
	}
	if o.IsAttacking {
		if atk == nil {
			atk = new(state.PlayerID)
		}
		*atk = o.Attacking
		cv.AttackingPlayer = atk
	}
	if len(o.BlockedBy) > 0 {
		cv.BlockedBy = append(blk, o.BlockedBy...)
	}
	if p.noDerived {
		// no keywords, zero P/T
	} else if p.vchars != nil {
		if len(dKw) > 0 {
			cv.Keywords = append(kws, dKw...)
		}
	} else if p.ch != nil {
		cv.Power = p.ch.Power(id)
		cv.Toughness = p.ch.Toughness(id)
		// A copy: Chars is an interface, and nothing guarantees an
		// implementation hands back a slice nobody else holds a reference
		// to (supplement §10's no-aliasing rule).
		if kw := p.ch.Keywords(id); len(kw) > 0 {
			cv.Keywords = append(kws, kw...)
		}
	}
	cv.Flags = cardFlags(p.g, o)
	if len(o.Imprinted) > 0 {
		cv.Imprinted = append([]state.ObjID(nil), o.Imprinted...)
	}
	cv.Paired = o.Paired
	if len(o.ExiledCards) > 0 {
		cv.ExiledCards = append([]state.ObjID(nil), o.ExiledCards...)
	}
	if o.HasAttachedPlayer {
		cv.AttachedToPlayer, cv.AttachedPlayer = true, o.AttachedPlayer
	}
	if len(o.Counters) > 0 {
		if ctr == nil {
			ctr = make(map[string]int32, len(o.Counters))
		} else {
			clear(ctr)
		}
		for _, c := range o.Counters {
			ctr[c.Kind] = c.N
		}
		cv.Counters = ctr
	}
}

// typeLine is strings.Join(parts, " "), the CardView.Types line, without a
// fresh string per card: prev (the slot's previous line) when it already
// reads the same, else the projection's type-line cache.
func (p *projector) typeLine(prev string, parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	if joinedIs(prev, parts) {
		return prev
	}
	if p.types != nil {
		return p.types.join(parts)
	}
	return strings.Join(parts, " ")
}

// joinedIs reports whether s == strings.Join(parts, " "), allocation-free.
func joinedIs(s string, parts []string) bool {
	for i, part := range parts {
		if i > 0 {
			if len(s) == 0 || s[0] != ' ' {
				return false
			}
			s = s[1:]
		}
		if !strings.HasPrefix(s, part) {
			return false
		}
		s = s[len(part):]
	}
	return len(s) == 0
}

// typeCache is a small direct-mapped cache of joined type lines, keyed by a
// hash of the parts and verified against them on every hit, so a hit is
// always exactly strings.Join(parts, " ").
type typeCache struct{ slots [256]string }

func (c *typeCache) join(parts []string) string {
	h := uint32(2166136261)
	for _, s := range parts {
		for i := 0; i < len(s); i++ {
			h = (h ^ uint32(s[i])) * 16777619
		}
		h = (h ^ ' ') * 16777619
	}
	slot := &c.slots[(h^h>>16)&255]
	if !joinedIs(*slot, parts) {
		*slot = strings.Join(parts, " ")
	}
	return *slot
}

// projScratch is the per-projection scratch that is not part of any View:
// only caches whose hits are verified, so which scratch a projection draws
// changes no output. Pooled, so a steady stream of projections reuses it.
type projScratch struct {
	types typeCache
	text  textMemo
}

// textMemo is a small direct-mapped memo of derived display text (a
// placeholder substitution, a restriction sentence), keyed by the two
// strings the text is a pure function of and verified against both on every
// hit, so a hit is always exactly what computing it would return.
type textMemo struct{ slots [64]textMemoEntry }

type textMemoEntry struct {
	k1, k2, out string
	kind        memoKind
}

// memoKind names which derivation an entry memoises; the zero value is an
// empty slot.
type memoKind uint8

const (
	memoSubstitute memoKind = iota + 1 // substitutePlaceholders(k1, k2)
	memoLabel                          // optionLabelText(k1)
	memoRestrict                       // the restriction sentence of Valid$ k1, chosen type k2
)

// lookup returns the memoised kind text for (k1, k2) and the slot it lives
// in; ok is false on a miss, and the caller stores the computed text through
// the slot's store.
func (m *textMemo) lookup(kind memoKind, k1, k2 string) (slot *textMemoEntry, out string, ok bool) {
	h := uint32(2166136261) ^ uint32(kind)
	for i := 0; i < len(k1); i++ {
		h = (h ^ uint32(k1[i])) * 16777619
	}
	h = (h ^ 0xff) * 16777619
	for i := 0; i < len(k2); i++ {
		h = (h ^ uint32(k2[i])) * 16777619
	}
	slot = &m.slots[(h^h>>16)&63]
	if slot.kind == kind && slot.k1 == k1 && slot.k2 == k2 {
		return slot, slot.out, true
	}
	return slot, "", false
}

func (e *textMemoEntry) store(kind memoKind, k1, k2, out string) string {
	*e = textMemoEntry{k1: k1, k2: k2, out: out, kind: kind}
	return out
}

// substitute is substitutePlaceholders(text, name), memoised in m when m is
// non-nil.
func (m *textMemo) substitute(text, name string) string {
	if m == nil || !hasPlaceholder(text) || name == "" {
		return substitutePlaceholders(text, name)
	}
	slot, out, ok := m.lookup(memoSubstitute, text, name)
	if ok {
		return out
	}
	return slot.store(memoSubstitute, text, name, substitutePlaceholders(text, name))
}

// labelText is optionLabelText(label), memoised in m when m is non-nil.
func (m *textMemo) labelText(label string) string {
	if m == nil || !hasPlaceholder(label) {
		return optionLabelText(label)
	}
	slot, out, ok := m.lookup(memoLabel, label, "")
	if ok {
		return out
	}
	return slot.store(memoLabel, label, "", optionLabelText(label))
}

var scratchPool = sync.Pool{New: func() any { return new(projScratch) }}
