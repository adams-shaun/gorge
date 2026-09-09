// Package view projects one seat's view of a state.Game: a hidden zone
// contributes a count and nothing else unless the viewer owns it, another
// seat's decision is never attached, and everything that can or will hit
// the stack (the user's requirement R3) is described for every seat. It is
// the only package a client-facing layer needs to read game state through —
// nothing here leaks a rules concept the client would have to understand.
//
// RedactEvents (redact.go) does the same job for the event log: it is
// state-aware, not merely Secret-flag-aware, because an event's Player
// field does not always name the seat whose secret its payload is (a
// trigger's controller and the owner of the card it remembered can be two
// different seats) — see RedactEvents' own doc for the three rules. An
// events.Note is always public unless its own emitter marks it Secret
// (Ruling T23-w): it is the engine's explicit "tell everyone" channel.
package view

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Chars is what the view asks the engine for: derived characteristics and
// the triggers that have matched but are not yet on the stack. *rules.Engine
// satisfies it (view_test.go pins that with a compile-time assertion);
// tests here use a flat stand-in instead of importing rules.
//
// Ruling F2: this is named Keywords, not Derived — Engine.Derived already
// returns a struct, and a method of that name could not also satisfy an
// interface expecting a slice.
type Chars interface {
	Power(state.ObjID) int32
	Toughness(state.ObjID) int32
	Keywords(state.ObjID) []string
	// PendingTriggers is R3: everything that WILL hit the stack, once its
	// controller has ordered it or its decider has accepted it, must be
	// observable too — not only what already has.
	PendingTriggers() []state.PendingTrigger
	// AvailableMana is the engine's answer to the seat-box line 3: the mana
	// this player could produce right now by activating the free-to-tap mana
	// abilities of untapped permanents it controls. It is derived from the
	// battlefield (a public zone), so unlike the floating Pool it may be
	// shown for every seat and every visibility, and it is what populates
	// that line in the ordinary case where the floating pool is empty.
	AvailableMana(state.PlayerID) state.Mana
}

// View is one seat's complete picture of the game: everything public, plus
// whatever is theirs alone (their hand, their mana pool, a decision asked of
// them).
type View struct {
	Viewer state.PlayerID `json:"viewer"`
	// Visibility names which rule set built this view: "seat", "public" or
	// "omniscient" (see Visibility).
	Visibility string `json:"visibility"`
	// Turn is the engine's per-player-turn counter (TurnChange events). It
	// increments once per seat's turn, so a four-seat table reads "Turn 22"
	// after five and a half rounds. Round is the exact round-trip count of the
	// players still alive, folded over the ordered event stream (RoundOf) by
	// host, which has the log in hand when it builds this view; a caller with
	// only a snapshot gets the roundOf approximation instead. Both Turn and
	// Round are sent because the raw engine counter is what the transcript
	// ("Turn N: <player>") refers to, and the round is what the board's
	// clock shows.
	Turn     int32          `json:"turn"`
	Round    int32          `json:"round"`
	Step     string         `json:"step"`
	Phase    string         `json:"phase"`
	Active   state.PlayerID `json:"active"`
	Priority state.PlayerID `json:"priority"`
	Over     bool           `json:"over"`
	Draw     bool           `json:"draw"`
	// Winner is nil unless Over && !Draw: PlayerID's zero value is seat 0, a
	// real seat, so a bare PlayerID field could never distinguish "seat 0
	// won" from "the game is still going" or "it was a draw" (Task 22
	// finding 5). A JSON null is unambiguous where a bare 0 would not be.
	Winner *state.PlayerID `json:"winner"`
	// Players, Stack and Pending (below) are every public list this type
	// carries. All of them are built non-nil even when empty (Ruling
	// T23-u): a client should never have to treat a bare JSON `null` and an
	// empty `[]` as the same "nothing here" case for one of these, the way
	// it legitimately must for Hand/Pool below.
	Players []PlayerView `json:"players"`
	// Stack keeps g.Stack's own order: index 0 is the bottom, the last
	// entry is the top. Public for every seat — R3.
	Stack []StackView `json:"stack"`
	// Pending is the trigger queue, in the order it will be placed on the
	// stack. Public for every seat — R3.
	Pending  []PendingView      `json:"pending"`
	Decision *decision.Decision `json:"decision,omitempty"`
}

// commanderViews builds a player's commander roster and its parallel
// command-zone cast counts in one pass, so a commander whose object is
// somehow absent (defensive -- a dangling roster id) is dropped from BOTH
// lists and the pair stays aligned. The roster CardViews project the
// commander's current zone's state like ordinary zone CardViews; the cast
// count is Player.CmdCasts's entry for that commander, whatever zone it
// currently occupies.
func commanderViews(g *state.Game, ch Chars, ids []state.ObjID, casts []int32) ([]CardView, []int32) {
	cmds := make([]CardView, 0, len(ids))
	cs := make([]int32, 0, len(ids))
	for k, id := range ids {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Ephemeral() {
			continue
		}
		cmds = append(cmds, cardView(g, ch, id))
		if k < len(casts) {
			cs = append(cs, casts[k])
		} else {
			cs = append(cs, 0)
		}
	}
	return cmds, cs
}

// PlayerView is one seat's own public state, plus (only when this is the
// viewer's own seat) the private parts.
//
// ID's tag is "seat", not "id": state.PlayerID and state.ObjID are both
// small integers, and this package's own leak test proves the collision is
// real -- a four-seat board's low ObjIDs (the very first cards dealt) land
// in the same 0-3 range as every PlayerID, so an "id" tag here would make a
// CardView's object id and a PlayerView's seat number indistinguishable by
// key name alone.
type PlayerView struct {
	ID            state.PlayerID `json:"seat"`
	Name          string         `json:"name"`
	Life          int32          `json:"life"`
	Lost          bool           `json:"lost"`
	LibrarySize   int            `json:"library_size"`
	HandSize      int            `json:"hand_size"`
	GraveyardSize int            `json:"graveyard_size"`
	// Hand is nil (marshalling to a literal JSON null, not an omitted key --
	// it deliberately carries no "omitempty" tag) for every seat but the
	// viewer's own, whose Hand is always non-nil even when empty ("[]").
	// omitempty cannot express "present but possibly empty": with it, the
	// viewer's own EMPTY hand would have marshalled identically to another
	// seat's HIDDEN one (the key simply missing either way), which is exactly
	// the ambiguity this type exists to avoid everywhere else (Winner's own
	// *PlayerID is the same shaped fix). null-vs-[] is what a client checks
	// instead. Hand is a hidden zone under CR 400.2 and stays gated on "is
	// this the viewer's own seat", unlike Pool (next field), which is public.
	Hand        []CardView `json:"hand"`
	Battlefield []CardView `json:"battlefield"`
	Graveyard   []CardView `json:"graveyard"`
	Exile       []CardView `json:"exile"`
	// Pool is the mana currently floating in this player's pool. It is
	// PUBLIC information under the CR: a mana pool is not one of the seven
	// zones in CR 400.1 and holds no cards, so CR 400.2's hidden-zone
	// framework (library and hand) has no purchase on it; instead CR 106.4a,
	// 106.4b, 117.3d and 118.3a all require a player to ANNOUNCE what is in
	// their pool, an obligation that is incoherent for information meant to
	// be hidden. So Pool is projected for every seat under every visibility
	// (seat, public, omniscient), like Available. It is always non-nil, even
	// when empty ("{}"), because there is no longer a hidden state to
	// distinguish: an empty pool is simply "{}". The field carries no
	// omitempty, so it is always present even when zero -- the one wire
	// distinction it keeps against Available (which carries omitempty and is
	// absent when nothing is available).
	Pool map[string]int32 `json:"pool"`
	// Available is what this player could produce right now by tapping
	// untapped permanents' free-to-tap mana abilities — the "free mana one
	// gains by tapping lands or other effects" half of the seat box's line
	// 3. It is derived from the battlefield (public), so it is filled for
	// every seat under every visibility (seat, public, omniscient), never
	// gated on "is this the viewer's own seat". It is always non-nil, even
	// when empty ("{}"), like Pool. It is separate from Pool on purpose — a
	// reader must never mistake mana that could be tapped for mana already
	// floating.
	// It carries omitempty (so an empty availability is absent, never a JSON
	// null): Available is a public quantity that is only ever present-when-
	// nonzero. Pool, in contrast, carries no omitempty and is always present
	// as an object. This mirrors the sibling CmdDamage field, another public
	// per-player map that is omitted when zero rather than sent as {}.
	Available map[string]int32 `json:"available,omitempty"`
	// Command is the command zone (CR 903.6): the player's commanders
	// currently sitting there, in zone order. A commander leaves it when it
	// is cast (the object itself moves; its id is stable), so Command is the
	// ever-shrinking subset of Commanders that can still be cast from the
	// command zone under the CR 903.8 tax. Public for every seat -- ZCommand
	// is not a hidden zone, and commander identity is open information.
	Command []CardView `json:"command"`
	// Commanders is the player's full commander roster -- the same list
	// m30's genesis built, in the same order, never shrunk as commanders
	// are cast or die. A roster CardView projects the commander's CURRENT
	// zone's state (a cast commander is a battlefield object, a dead one a
	// graveyard object), so the roster is what lets a client -- and the bot
	// policy's view-shaped half -- tell that a battlefield creature is a
	// commander (the CR 903.10 clock's subject) even when it has left the
	// command zone and not yet dealt damage. Public for every seat: the
	// identity of a player's commanders is the premise of the format.
	Commanders []CardView `json:"commanders"`
	// CommanderCasts runs parallel to Commanders: entry k is how many times
	// Commanders[k] has been cast from the command zone, the CR 903.8 tax
	// base for its next command-zone cast (an additional {2} per prior
	// cast). Public for every seat -- the count is derived from public
	// events.
	CommanderCasts []int32 `json:"commander_casts"`
	// CmdDamage is the commander damage this player has taken (CR 903.10),
	// keyed by each commander's object id -- the 21-damage clock, public
	// for every seat like a life total. nil/absent when the player has
	// taken no commander damage (omitempty: absence is zero), so a
	// Constructed game never pays for a per-player empty map.
	CmdDamage map[state.ObjID]int32 `json:"cmd_damage,omitempty"`
}

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
	ID    state.ObjID `json:"id"`
	Name  string      `json:"name"`
	Types string      `json:"types"`
	// ManaCost is the printed cost in Forge's notation ("1 W", "R", "X G").
	// Hand lists render it as symbols.
	ManaCost string `json:"mana_cost,omitempty"`
	// Printing is what an image lookup keys on; Token ("#12") tells two
	// copies of one card apart in the stack, the log and an arrow.
	Printing Printing `json:"printing"`
	Token    string   `json:"token"`
	// AttackingPlayer is the seat this creature is attacking while
	// Attacking is true, nil otherwise; BlockedBy lists the creatures
	// blocking it. Both exist for the arrow overlay (PL-17) and come
	// straight from the object's combat fields, which EndCombatReset clears.
	AttackingPlayer *state.PlayerID  `json:"attacking_player,omitempty"`
	BlockedBy       []state.ObjID    `json:"blocked_by,omitempty"`
	Tapped          bool             `json:"tapped"`
	Power           int32            `json:"power"`
	Toughness       int32            `json:"toughness"`
	Damage          int32            `json:"damage"`
	Attacking       bool             `json:"attacking"`
	Counters        map[string]int32 `json:"counters,omitempty"`
	Keywords        []string         `json:"keywords,omitempty"`
	// Controller and Owner can differ (a stolen permanent, a stack object
	// created for someone else's turn); a client needs both. Battlefield
	// zone lists are keyed by controller, every hidden/graveyard/exile list
	// by owner (events/apply.go's zoneOwner), so a CardView carries both
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
	// Produces is what this card's mana abilities add to the pool when a
	// tap-for-mana activation runs them, derived from the compiled abilities
	// (cards.Face.ManaProduction) rather than land subtypes: a basic land's
	// intrinsic {W}, a dual's {W}{U}, an "add any colour" source's resolved
	// colourless, a colourless rock's {C}{C}. nil when the card has no mana
	// ability at all, so a creature or a spell never pays for the six-entry
	// array on the wire. It is a projected characteristic like ManaCost and
	// Keywords -- a mana ability's production is a card fact every seat sees,
	// and it is projected for the same zones the card itself is visible in.
	// This is what the bot policy's colour-aware tap and land heuristics read
	// (carried into botpolicy.Card as plain data, never the view type).
	Produces *cards.ManaProduction `json:"produces,omitempty"`
}

// StackView is one object on the stack. Kind is "spell", "trigger" (an
// object minted by a TriggerPush) or "ability" (any other ability object).
type StackView struct {
	ID         state.ObjID    `json:"id"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"` // card name; for an ability, its source's name
	Text       string         `json:"text"` // what it does, in the card's own words, when known
	Controller state.PlayerID `json:"controller"`
	Source     state.ObjID    `json:"source,omitempty"` // ability only: the permanent it came from
	// Targets is a public list (Ruling T23-u): non-nil, "[]" not "null",
	// even when nothing has been targeted yet.
	Targets []TargetView `json:"targets"`
	Card    *CardView    `json:"card,omitempty"` // spell only
}

// TargetView is one chosen target: exactly one of Obj and Player means
// anything, discriminated by IsPlayer — the same shape as state.Target.
type TargetView struct {
	Obj      state.ObjID    `json:"obj,omitempty"`
	Player   state.PlayerID `json:"player"`
	IsPlayer bool           `json:"is_player"`
	// Label is what the object was allowed to target, in the card's own
	// words: its TgtPrompt$ when it has one ("Select any target"), else its
	// ValidTgts$ ("Creature"). Empty when the object declares neither.
	Label string `json:"label,omitempty"`
}

// PendingView is a trigger that will hit the stack once its controller has
// ordered it / its decider has accepted it. R3.
type PendingView struct {
	Source     state.ObjID     `json:"source"`
	Controller state.PlayerID  `json:"controller"`
	Label      string          `json:"label"`
	Optional   bool            `json:"optional"`
	Decider    *state.PlayerID `json:"decider,omitempty"` // nil unless Optional
}

func displayName(p *state.Player) string {
	if p.PlayerName != "" {
		return p.PlayerName
	}
	return p.Name
}

// Project builds one seat's view. A hidden zone contributes a count and
// nothing else unless the viewer owns it, and a decision is attached only to
// the player it was asked of. Total: g == nil, ch == nil, and an
// out-of-range viewer all degrade rather than panic (supplement §7).
// Project is ProjectFor with Seat visibility.
func Project(g *state.Game, ch Chars, viewer state.PlayerID, d *decision.Decision) View {
	return ProjectFor(g, ch, viewer, Seat, d)
}

// project is Project's body, shared by every Visibility in ProjectFor.
func project(g *state.Game, ch Chars, viewer state.PlayerID, d *decision.Decision) View {
	v := View{Viewer: viewer}
	if g == nil {
		return v
	}
	v.Turn = g.Turn
	v.Round = roundOf(g)
	v.Step = g.Step.String()
	v.Phase = PhaseOf(g.Step)
	v.Active = g.Active
	v.Priority = g.Priority
	v.Over = g.Over
	v.Draw = g.Draw
	if g.Over && !g.Draw && int(g.Winner) < len(g.Players) {
		w := g.Winner
		v.Winner = &w
	}
	v.Stack = stackViews(g, ch, g.Stack)
	// Default to the non-nil empty shape (Ruling T23-u) whether or not ch
	// is nil; a real Chars overwrites it below.
	v.Pending = pendingViews(nil)
	if ch != nil {
		v.Pending = pendingViews(ch.PendingTriggers())
	}

	// A viewer index that names no real seat is a spectator: everything
	// below that gates on "is this the viewer's own seat" naturally stays
	// closed for them, since no real p.ID will ever equal an out-of-range
	// viewer — but the decision check does not have that same natural
	// guard (a malformed Decision.Player could coincide with a malformed
	// viewer), so it is checked explicitly.
	spectator := int(viewer) >= len(g.Players)

	// Non-nil even when g.Players is empty (Ruling T23-u): an empty match
	// still marshals "players":[], never "players":null.
	v.Players = make([]PlayerView, 0, len(g.Players))
	// denseCmd is every commander object in the match, in the match-wide
	// dense order rules.New assigns at genesis (player order, then each
	// player's Commanders order) -- the index Player.CmdDamage is keyed by,
	// so each player's tally slice can be re-keyed by object identity for
	// the wire (CmdDamage map). Nil when the match has no commanders; the
	// single small slice is shared by every player's clock below.
	var denseCmd []state.ObjID
	for i := range g.Players {
		denseCmd = append(denseCmd, g.Players[i].Commanders...)
	}
	for i := range g.Players {
		p := &g.Players[i]
		roster, casts := commanderViews(g, ch, p.Commanders, p.CmdCasts)
		pv := PlayerView{
			ID: p.ID, Name: displayName(p), Life: p.Life, Lost: p.Lost,
			LibrarySize:    len(g.Zone(state.ZLibrary, p.ID)),
			HandSize:       len(g.Zone(state.ZHand, p.ID)),
			GraveyardSize:  len(g.Zone(state.ZGraveyard, p.ID)),
			Battlefield:    cardViews(g, ch, g.Zone(state.ZBattlefield, p.ID)),
			Graveyard:      cardViews(g, ch, g.Zone(state.ZGraveyard, p.ID)),
			Exile:          cardViews(g, ch, g.Zone(state.ZExile, p.ID)),
			Command:        cardViews(g, ch, g.Zone(state.ZCommand, p.ID)),
			Commanders:     roster,
			CommanderCasts: casts,
		}
		// Available is public (battlefield-derived) and so projected for
		// every seat under every visibility, like Pool; only Hand (a CR 400.2
		// hidden zone) is filled for the viewer's own seat. A nil ch
		// (supplement §7) degrades to an empty (zero) availability, the same
		// way it degrades every other derived characteristic.
		var avail state.Mana
		if ch != nil {
			avail = ch.AvailableMana(p.ID)
		}
		pv.Available = poolView(avail)
		// The 21-damage clock: this player's cumulative commander damage,
		// keyed by the commander that dealt it (re-keyed off the dense
		// slice CmdDamage is indexed by). Only built when any tally is
		// nonzero -- nil/absent means zero -- so no map is allocated for a
		// player (or a game) with no commander damage.
		if len(p.CmdDamage) > 0 {
			for j, id := range denseCmd {
				if j < len(p.CmdDamage) && p.CmdDamage[j] != 0 {
					if pv.CmdDamage == nil {
						pv.CmdDamage = make(map[state.ObjID]int32, len(denseCmd))
					}
					pv.CmdDamage[id] = p.CmdDamage[j]
				}
			}
		}
		// Pool is public (CR 106.4a/106.4b): a mana pool is not one of the
		// seven zones in CR 400.1 and holds no cards, so CR 400.2's hidden-
		// zone framework has no purchase on it; instead 106.4a, 106.4b,
		// 117.3d and 118.3a all require a player to ANNOUNCE what is in their
		// pool, an obligation incoherent for hidden information. So Pool is
		// projected for every seat under every visibility, like Available;
		// only Hand stays gated on "is this the viewer's own seat" (CR 400.2
		// names hand as a hidden zone).
		pv.Pool = poolView(p.Pool)
		if p.ID == viewer {
			pv.Hand = cardViews(g, ch, g.Zone(state.ZHand, p.ID))
		}
		v.Players = append(v.Players, pv)
	}

	if !spectator && d != nil && d.Player == viewer {
		// A copy, never the engine's own pending pointer (supplement §10):
		// a Seat (Task 25) holds this View in-process and must not be able
		// to corrupt the live decision through it.
		cp := *d
		cp.Options = append([]decision.Option(nil), d.Options...)
		v.Decision = &cp
	}
	return v
}

// RoundOf is the EXACT round-trip count, folded over the ordered event
// stream rather than projected from a single state.Game snapshot. This is
// the honest fix for the round number: the log knows the order things
// happened in, which a snapshot cannot. It is a pure function of the game
// and the event log -- it reads g.Players (the seat count) and the
// events.PlayerLost / events.TurnChange stream, writes nothing, and cannot
// change a chain head -- so it is identical across a live game and a
// log-only reconstruction of the same game (replay reproduces the same
// events, and this fold is deterministic on them).
//
// The rule, stated precisely: a round boundary is a TurnChange that hands
// the turn to the FIRST STILL-LIVING SEAT in turn order (the lowest-index
// survivor, seat order 0, 1, ..., N-1 being the cyclic turn order the
// engine's NextAlive follows) -- i.e. play has returned to the seat that
// opened the current pass. Round 1 is the initial pass, so the very first
// TurnChange never counts as a boundary (the fold keeps a seen flag and
// skips the increment on it); each later return to the then-first survivor
// opens a new round. The fold tracks the alive set itself by folding
// PlayerLost events, so it knows the first survivor at every moment rather
// than only at the end -- the thing a snapshot cannot recover.
//
// The three edge cases fall out of the one rule. First round before any
// turn: no TurnChange has been seen, so RoundOf is 1; the first TurnChange
// never increments. A seat eliminated MID-ROUND (not the anchor): the first
// survivor is unchanged, so the survivors after the death simply get a
// shorter cycle and no boundary fires until play genuinely returns to that
// (unchanged) first survivor. A seat eliminated who WAS the anchor (the
// first survivor): the first survivor becomes the next-lowest living seat,
// and the next turn to reach that new first survivor -- which the old
// anchor's death immediately makes imminent -- opens a new round. This is
// the case that breaks a naive "the active index fell" wrap rule: when the
// anchor (seat 0) dies during its own turn, the turn passes 0 -> 1 (an
// index RISE), yet play has returned to the first still-living seat, so it
// IS a round boundary. The count is monotonic non-decreasing: it only ever
// increments, never repeats a value for a later state and never jumps
// backwards.
//
// Extra turns (a card granting a seat two turns inside one round-trip) are
// out of scope for this build: beginTurn is reached only from genesis and
// from rules/turn.go's NextAlive advance, and the effects registry
// registers no AddTurn primitive, so the engine never produces a same-seat
// repeat turn for the fold to misread; if AddTurn ever lands, this fold
// must be revisited.
//
// Guards: a nil game, an empty log (or one with no TurnChange) and a
// table whose seats have all been eliminated all fold to round 1.
//
// This is what the board clock should use wherever the ordered event stream
// is in hand (host builds views from the log); roundOf below is the
// snapshot-only approximation and the fallback for a consumer that has only
// a state.Game.
func RoundOf(g *state.Game, evs []events.Event) int32 {
	n := len(g.Players)
	if n <= 0 {
		return 1
	}
	alive := make([]bool, n)
	for i := range alive {
		alive[i] = true
	}
	round := int32(1)
	seen := false
	for _, ev := range evs {
		switch ev.Kind {
		case events.PlayerLost:
			if int(ev.Player) < n {
				alive[ev.Player] = false
			}
		case events.TurnChange:
			cur := int(ev.Player)
			// The first still-living seat in turn order: the lowest index
			// among the survivors. If no seat is alive (defensive -- a
			// finished game whose winner has not been flagged) there is no
			// first survivor to return to, so no boundary can fire.
			first := 0
			for first < n && !alive[first] {
				first++
			}
			if seen && first < n && cur == first {
				round++
			}
			seen = true
		}
	}
	return round
}

// roundOf projects the engine's per-player-turn counter (g.Turn) onto the
// number of round-trips the table has made. The rule, stated once: a round
// is one full pass around the table among the players still alive, and the
// projected number is 1 + (Turn-1)/AliveCount -- so round 1 spans the first
// complete pass of every surviving seat, round 2 the second, and the round
// length (AliveCount) shrinks as seats are eliminated.
//
// This is the SNAPSHOT-ONLY APPROXIMATION, kept as the fallback for a
// consumer that has only a state.Game (the cmd/* tools and this package's
// own tests are the ones that genuinely lack the log). It is exact before
// the first elimination and, after one, runs AHEAD of the true round-trip
// count -- the exact one is view.RoundOf, folded over the ordered event
// stream, which is what host uses to build the board clock and what the
// AGENTS.md row names as the honest fix. Do not route a log-bearing caller
// through this function; it is here so a snapshot-only caller still gets a
// never-wrong-direction round rather than a panic, and because the log is
// genuinely unavailable in that shape.
//
// This is a projection of View's existing state and nothing else: it reads
// g.Turn and each seat's Lost flag, writes no event, and cannot change a
// chain head. It is computed from the single snapshot because neither view
// nor the client keeps a per-turn history; the consequence is that it
// cannot reconstruct WHEN a seat died, only the current alive set. Two
// truths follow and are worth writing down. First, before the first
// elimination it is exactly the round-trip count. Second, after an
// elimination it stays monotonic -- 1+(Turn-1)/AliveCount only ever grows
// as Turn grows and only grows faster when AliveCount shrinks -- so it
// never repeats a value for a later game state and never jumps backwards,
// which is the failure the board's clock must not exhibit; it may run ahead
// of the literal "when play returned to the first surviving seat" count,
// because that count needs the death times this snapshot does not carry.
//
// Extra turns (a card granting a seat two turns inside one round-trip) are
// taken care of by the engine's shape, not by this function: beginTurn is
// reached only from genesis and from rules/turn.go's NextAlive advance, and
// the effects registry registers no AddTurn primitive, so this build never
// produces a same-seat repeat turn for the projection to account for.
//
// Guards: a nil game, a Turn still at its pre-genesis 0, or a table with no
// surviving seats all project to round 1 rather than dividing by zero.
func roundOf(g *state.Game) int32 {
	if g == nil || g.Turn <= 0 {
		return 1
	}
	alive := int32(0)
	for _, p := range g.Players {
		if !p.Lost {
			alive++
		}
	}
	if alive <= 0 {
		return 1
	}
	return (g.Turn-1)/alive + 1
}

// PhaseOf groups a Step into the five phases a client shows: beginning,
// main1, combat, main2, ending; "" for an invalid Step.
func PhaseOf(s state.Step) string {
	switch s {
	case state.StepUntap, state.StepUpkeep, state.StepDraw:
		return "beginning"
	case state.StepMain1:
		return "main1"
	case state.StepBeginCombat, state.StepDeclareAttackers, state.StepDeclareBlockers,
		state.StepCombatDamage, state.StepEndCombat:
		return "combat"
	case state.StepMain2:
		return "main2"
	case state.StepEnd, state.StepCleanup:
		return "ending"
	default:
		return ""
	}
}

// cardViews maps a zone's object ids to CardViews, in zone order. An id
// whose object no longer exists (a dangling entry, or a defensively
// tampered list) is skipped rather than producing a zero CardView or
// panicking (supplement §7). Always non-nil (Ruling T23-u), even for an
// empty or all-dangling ids: this is what lets the viewer's own genuinely
// empty Hand marshal "[]" rather than the same "null" a hidden hand would.
//
// Ephemeral objects (copies, tokens off the battlefield) have ceased to
// exist -- state.Object.Ephemeral is the single definition of that, consulted
// here rather than re-spelled inline so it cannot drift from any other call
// site -- and an ability object (Card == nil, so Face() == nil too) never
// legitimately sits in a card zone at all. Both are parked in exile by the
// engine and are skipped here (Task 4).
func cardViews(g *state.Game, ch Chars, ids []state.ObjID) []CardView {
	out := make([]CardView, 0, len(ids))
	for _, id := range ids {
		o := g.Obj(id)
		// An ability object (no Face) is engine bookkeeping, not a card in
		// this zone; Ephemeral covers copies and tokens off the battlefield.
		if o == nil || o.Face() == nil || o.Ephemeral() {
			continue
		}
		out = append(out, cardView(g, ch, id))
	}
	return out
}

// cardView builds one object's public face. ch is read through Power/
// Toughness/Keywords rather than any printed field directly — the view asks
// the engine for derived characteristics, never the card's own text — and a
// nil ch (supplement §7) degrades to the zero P/T with no keywords.
func cardView(g *state.Game, ch Chars, id state.ObjID) CardView {
	o := g.Obj(id)
	cv := CardView{
		ID: id, Tapped: o.Tapped, Damage: o.Damage, Attacking: o.IsAttacking,
		Controller: o.Controller, Owner: o.Owner, SummonSick: o.SummonSick,
		AttachedTo: o.AttachedTo,
	}
	cv.Token = "#" + strconv.FormatUint(uint64(id), 10)
	if f := o.Face(); f != nil {
		cv.Name = f.Name
		cv.Types = strings.Join(f.Types, " ")
		cv.ManaCost = f.ManaCost
		cv.Printing = Printing{Name: f.Name}
		// The mana-production projection (Task dp2): what tapping this card
		// puts in the pool, from its own abilities. A nil pointer keeps a card
		// with no mana ability off the wire; p is a fresh value per call, so
		// the address is never aliased across object views.
		if p := f.ManaProduction(); !p.IsZero() {
			cv.Produces = &p
		}
	}
	if o.IsAttacking {
		p := o.Attacking
		cv.AttackingPlayer = &p
	}
	if len(o.BlockedBy) > 0 {
		cv.BlockedBy = append([]state.ObjID(nil), o.BlockedBy...)
	}
	if ch != nil {
		cv.Power = ch.Power(id)
		cv.Toughness = ch.Toughness(id)
		// A defensive copy: Chars is an interface, and nothing guarantees
		// an implementation hands back a slice nobody else holds a
		// reference to (supplement §10's no-aliasing rule).
		if kw := ch.Keywords(id); len(kw) > 0 {
			cv.Keywords = append([]string(nil), kw...)
		}
	}
	if len(o.Counters) > 0 {
		cv.Counters = make(map[string]int32, len(o.Counters))
		for _, c := range o.Counters {
			cv.Counters[c.Kind] = c.N
		}
	}
	return cv
}

// stackViews maps the stack's own object ids to StackViews, bottom to top.
// Always non-nil (Ruling T23-u).
func stackViews(g *state.Game, ch Chars, ids []state.ObjID) []StackView {
	out := make([]StackView, 0, len(ids))
	for _, id := range ids {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		if o.Ability != nil {
			// An ability object has no Face (Ruling F3): Card == nil, set
			// by events/apply.go's TriggerPush case. Its display name is
			// the face name of the permanent it came from, or "Ability"
			// when that permanent is also gone (supplement §2). Kind
			// distinguishes a TriggerPush object from any other ability
			// object (an activated ability, once the engine enumerates
			// them) so a client can render them differently.
			kind := "ability"
			if _, ok := triggerLine(g, o); ok {
				kind = "trigger"
			}
			sv := StackView{
				ID: id, Kind: kind, Name: abilityName(g, o), Text: abilityText(g, o),
				Controller: o.Controller, Source: o.Source, Targets: targetViews(o.Targets, targetLabel(o)),
			}
			// The ability object has no face of its own (Ruling F3), so the
			// artwork a client shows for a trigger/ability band has to come
			// from the permanent it was minted from -- the same cardView the
			// spell branch uses for its own id. Fill Card only while that
			// source is a visible object: the source may have left the
			// battlefield to a hidden zone (a "leaves the battlefield"
			// trigger whose card is now in its owner's hand) or be gone
			// entirely, and neither may be exposed here. This is the same
			// projection everything else uses, never a hand-built view.
			if src := g.Obj(o.Source); src != nil && src.Face() != nil && !src.Zone.Hidden() && !src.Ephemeral() {
				cv := cardView(g, ch, o.Source)
				sv.Card = &cv
			}
			out = append(out, sv)
			continue
		}
		sv := StackView{ID: id, Kind: "spell", Controller: o.Controller, Targets: targetViews(o.Targets, targetLabel(o))}
		if f := o.Face(); f != nil {
			sv.Name = f.Name
			sv.Text = spellText(f)
			cv := cardView(g, ch, id)
			sv.Card = &cv
		}
		out = append(out, sv)
	}
	return out
}

// abilityName is the face name of the permanent an ability object came
// from, or "Ability" when that permanent is gone too (supplement §2).
func abilityName(g *state.Game, o *state.Object) string {
	if src := g.Obj(o.Source); src != nil {
		if f := src.Face(); f != nil && f.Name != "" {
			return f.Name
		}
	}
	return "Ability"
}

// triggerLine finds the T: line an ability object was minted from: the
// trigger on its source's active face whose Effect is exactly o.Ability
// (events/apply.go's TriggerPush case sets it so). ok is false for an
// ability object that is not a trigger, or whose source has changed face.
func triggerLine(g *state.Game, o *state.Object) (cards.Trigger, bool) {
	src := g.Obj(o.Source)
	if src == nil {
		return cards.Trigger{}, false
	}
	f := src.Face()
	if f == nil {
		return cards.Trigger{}, false
	}
	for _, t := range f.Triggers {
		if t.Effect == o.Ability {
			return t, true
		}
	}
	return cards.Trigger{}, false
}

// abilityText finds the T: line an ability object was minted from (see
// triggerLine) and returns its TriggerDescription$. Falling back to the
// SA's own SpellDescription$/StackDescription$ covers an activated ability
// (a later milestone) or a source that changed face since the trigger
// matched (rules/trigger.go's triggerOf documents the same caveat). The
// text is then placeholder-substituted with the same display name the
// StackView.Name uses -- the source's face name, or "Ability" when the
// source is gone (see abilityName and substitutePlaceholders).
func abilityText(g *state.Game, o *state.Object) string {
	text := ""
	if t, ok := triggerLine(g, o); ok {
		if d := t.Params["TriggerDescription"]; d != "" {
			text = d
		}
	}
	if text == "" && o.Ability != nil {
		if d := o.Ability.Params["SpellDescription"]; d != "" {
			text = d
		}
		if text == "" {
			if d := o.Ability.Params["StackDescription"]; d != "" {
				text = d
			}
		}
	}
	return substitutePlaceholders(text, abilityName(g, o))
}

// spellText is SpellDescription$ of the face's own cast ability, falling
// back to the printed Oracle text, with Forge's self-reference placeholders
// substituted by the card's own name (see substitutePlaceholders).
func spellText(f *cards.Face) string {
	text := f.Oracle
	if sa := f.SpellAbility(); sa != nil {
		if d := sa.Params["SpellDescription"]; d != "" {
			text = d
		}
	}
	return substitutePlaceholders(text, f.Name)
}

// substitutePlaceholders replaces Forge's self-reference placeholders in a
// rules-text string with the card's own display name. CARDNAME is Forge's
// token for the card's full name; NICKNAME is the same token narrowed to the
// name's first word (Forge's default nickname -- the corpus carries no
// Nickname$ or Nickname: line and the parser has no field for one, so the
// first word is the whole of what the engine can know). Both are matched on
// a word boundary -- a letter or digit on either side is not a match -- so a
// token inside a larger word ("CARDNAMES") is left alone, and the substituted
// name is written out and never re-scanned, so a name that legitimately
// contains the letters ("Forked Bolt") is inserted whole and a name that
// happened to be a placeholder token as a whole word would not be rewritten
// a second time.
//
// The set is what the corpus actually uses in the text this package renders:
// CARDNAME is pervasive in SpellDescription$/TriggerDescription$/
// StackDescription$ (and, like NICKNAME, is always the uppercase token in
// those fields), NICKNAME occurs in about a thousand of them, and the "~"
// shorthand does not occur in any of them (it appears only in Forge SVar
// arithmetic, which is never rendered) -- see the task report for the grep.
func substitutePlaceholders(text, name string) string {
	if text == "" || name == "" {
		return text
	}
	nick := firstWord(name)
	var b strings.Builder
	b.Grow(len(text))
	i := 0
	for i < len(text) {
		if !isWordByte(text[i]) {
			b.WriteByte(text[i])
			i++
			continue
		}
		j := i
		for j < len(text) && isWordByte(text[j]) {
			j++
		}
		tok := text[i:j]
		switch tok {
		case "CARDNAME":
			b.WriteString(name)
		case "NICKNAME":
			b.WriteString(nick)
		default:
			b.WriteString(tok)
		}
		i = j
	}
	return b.String()
}

// firstWord is the first whitespace-delimited word of a name -- Forge's
// NICKNAME default ("Forked Bolt" -> "Forked"). A trailing comma, the
// separator between a legendary title and its epithet ("Ambergris, Agent of
// Destruction" -> "Ambergris"), is dropped so the substitution does not
// render "Ambergris, deals ..."; a hyphen inside the first word is kept
// ("A-Alrund, God of the Cosmos" -> "A-Alrund").
func firstWord(s string) string {
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		i = len(s)
	}
	return strings.TrimRight(s[:i], ",")
}

// isWordByte is the word-character edge the placeholder scan matches on:
// letters and digits only. A placeholder token at either end of the string,
// or adjacent to punctuation (an apostrophe, a comma, a period), is still a
// whole word; a placeholder inside a larger word is not.
func isWordByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

// targetViews copies an object's chosen targets. Object.Remembered is NEVER
// projected here or anywhere else in this package — it can name a
// hidden-zone object (e.g. the card a "whenever you draw" trigger
// remembered) — only Targets, which is a player-visible choice already.
// label (from targetLabel) is stamped on every entry: the object declared
// one set of legal targets, not one per chosen target. Always non-nil
// (Ruling T23-u).
func targetViews(targets []state.Target, label string) []TargetView {
	out := make([]TargetView, 0, len(targets))
	for _, t := range targets {
		out = append(out, TargetView{Obj: t.Obj, Player: t.Player, IsPlayer: t.IsPlayer, Label: label})
	}
	return out
}

// targetLabel is the object's own description of what it targets, from the
// first SA in its chain (the SA itself, then SubAbility$ links) that
// declares TgtPrompt$, else the first that declares ValidTgts$.
func targetLabel(o *state.Object) string {
	var sa *cards.SA
	if o.Ability != nil {
		sa = o.Ability
	} else if f := o.Face(); f != nil {
		sa = f.SpellAbility()
	}
	valid := ""
	for s := sa; s != nil; s = s.Sub {
		if p := s.Params["TgtPrompt"]; p != "" {
			return p
		}
		if valid == "" {
			valid = s.Params["ValidTgts"]
		}
	}
	return valid
}

// pendingViews copies the engine's pending-trigger queue into the wire
// shape, in the same order: R3. Always non-nil (Ruling T23-u), including
// when called with nil (Project's own default before a real Chars, if any,
// overwrites it).
func pendingViews(pts []state.PendingTrigger) []PendingView {
	out := make([]PendingView, 0, len(pts))
	for _, pt := range pts {
		pv := PendingView{Source: pt.Source, Controller: pt.Controller, Label: pt.Label, Optional: pt.Optional}
		if pt.Optional {
			who := pt.Decider
			pv.Decider = &who
		}
		out = append(out, pv)
	}
	return out
}
