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
// different seats) — see RedactEvents' own doc for the three rules.
package view

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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
}

// View is one seat's complete picture of the game: everything public, plus
// whatever is theirs alone (their hand, their mana pool, a decision asked of
// them).
type View struct {
	Viewer   state.PlayerID `json:"viewer"`
	Turn     int32          `json:"turn"`
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
	// Hand and Pool are nil (marshalling to a literal JSON null, not an
	// omitted key -- both deliberately carry no "omitempty" tag) for every
	// seat but the viewer's own, whose Hand/Pool are always non-nil even
	// when empty ("[]"/"{}"). omitempty cannot express "present but
	// possibly empty": with it, the viewer's own EMPTY hand would have
	// marshalled identically to another seat's HIDDEN one (the key simply
	// missing either way), which is exactly the ambiguity this type exists
	// to avoid everywhere else (Winner's own *PlayerID is the same shaped
	// fix). null-vs-[] is what a client checks instead.
	Hand        []CardView       `json:"hand"`
	Battlefield []CardView       `json:"battlefield"`
	Graveyard   []CardView       `json:"graveyard"`
	Exile       []CardView       `json:"exile"`
	Pool        map[string]int32 `json:"pool"`
}

// CardView is one object's public face: printed identity plus its current,
// derived characteristics. Nothing here is read from a hidden zone unless
// the viewer owns it — cardViews is only ever called with a zone list the
// caller has already decided is visible.
type CardView struct {
	ID        state.ObjID      `json:"id"`
	Name      string           `json:"name"`
	Types     string           `json:"types"`
	Tapped    bool             `json:"tapped"`
	Power     int32            `json:"power"`
	Toughness int32            `json:"toughness"`
	Damage    int32            `json:"damage"`
	Attacking bool             `json:"attacking"`
	Counters  map[string]int32 `json:"counters,omitempty"`
	Keywords  []string         `json:"keywords,omitempty"`
	// Controller and Owner can differ (a stolen permanent, a stack object
	// created for someone else's turn); a client needs both. Battlefield
	// zone lists are keyed by controller, every hidden/graveyard/exile list
	// by owner (events/apply.go's zoneOwner), so a CardView carries both
	// regardless of which zone list it came from.
	Controller state.PlayerID `json:"controller"`
	Owner      state.PlayerID `json:"owner"`
	SummonSick bool           `json:"summon_sick"`
}

// StackView is one object on the stack. Kind is "spell" or "ability".
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

// Project builds one seat's view. A hidden zone contributes a count and
// nothing else unless the viewer owns it, and a decision is attached only to
// the player it was asked of. Total: g == nil, ch == nil, and an
// out-of-range viewer all degrade rather than panic (supplement §7).
func Project(g *state.Game, ch Chars, viewer state.PlayerID, d *decision.Decision) View {
	v := View{Viewer: viewer}
	if g == nil {
		return v
	}
	v.Turn = g.Turn
	v.Step = g.Step.String()
	v.Phase = phaseOf(g.Step)
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
	for i := range g.Players {
		p := &g.Players[i]
		pv := PlayerView{
			ID: p.ID, Name: p.Name, Life: p.Life, Lost: p.Lost,
			LibrarySize:   len(g.Zone(state.ZLibrary, p.ID)),
			HandSize:      len(g.Zone(state.ZHand, p.ID)),
			GraveyardSize: len(g.Zone(state.ZGraveyard, p.ID)),
			Battlefield:   cardViews(g, ch, g.Zone(state.ZBattlefield, p.ID)),
			Graveyard:     cardViews(g, ch, g.Zone(state.ZGraveyard, p.ID)),
			Exile:         cardViews(g, ch, g.Zone(state.ZExile, p.ID)),
		}
		if p.ID == viewer {
			pv.Hand = cardViews(g, ch, g.Zone(state.ZHand, p.ID))
			pool := map[string]int32{}
			for idx, sym := range [...]string{"W", "U", "B", "R", "G", "C"} {
				if n := p.Pool[idx]; n > 0 {
					pool[sym] = n
				}
			}
			pv.Pool = pool
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

// phaseOf groups Step into the five phases a client actually cares about
// (supplement §6). An invalid Step (state.Step.Valid reports false) matches
// none of the cases below and falls through to "".
func phaseOf(s state.Step) string {
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
func cardViews(g *state.Game, ch Chars, ids []state.ObjID) []CardView {
	out := make([]CardView, 0, len(ids))
	for _, id := range ids {
		if g.Obj(id) == nil {
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
	}
	if f := o.Face(); f != nil {
		cv.Name = f.Name
		cv.Types = strings.Join(f.Types, " ")
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
			// when that permanent is also gone (supplement §2).
			out = append(out, StackView{
				ID: id, Kind: "ability", Name: abilityName(g, o), Text: abilityText(g, o),
				Controller: o.Controller, Source: o.Source, Targets: targetViews(o.Targets),
			})
			continue
		}
		sv := StackView{ID: id, Kind: "spell", Controller: o.Controller, Targets: targetViews(o.Targets)}
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

// abilityText finds the T: line an ability object was minted from — the
// trigger on the source's ACTIVE face whose Effect pointer is exactly
// o.Ability, which events/apply.go's TriggerPush case sets it to — and
// returns its TriggerDescription$. Falling back to the SA's own
// SpellDescription$/StackDescription$ covers an activated ability (a later
// milestone) or a source that changed face since the trigger matched
// (rules/trigger.go's triggerOf documents the same caveat).
func abilityText(g *state.Game, o *state.Object) string {
	if src := g.Obj(o.Source); src != nil {
		if f := src.Face(); f != nil {
			for _, t := range f.Triggers {
				if t.Effect == o.Ability {
					if d := t.Params["TriggerDescription"]; d != "" {
						return d
					}
					break
				}
			}
		}
	}
	if o.Ability != nil {
		if d := o.Ability.Params["SpellDescription"]; d != "" {
			return d
		}
		if d := o.Ability.Params["StackDescription"]; d != "" {
			return d
		}
	}
	return ""
}

// spellText is SpellDescription$ of the face's own cast ability, falling
// back to the printed Oracle text.
func spellText(f *cards.Face) string {
	if sa := f.SpellAbility(); sa != nil {
		if d := sa.Params["SpellDescription"]; d != "" {
			return d
		}
	}
	return f.Oracle
}

// targetViews copies an object's chosen targets. Object.Remembered is NEVER
// projected here or anywhere else in this package — it can name a
// hidden-zone object (e.g. the card a "whenever you draw" trigger
// remembered) — only Targets, which is a player-visible choice already.
// Always non-nil (Ruling T23-u).
func targetViews(targets []state.Target) []TargetView {
	out := make([]TargetView, 0, len(targets))
	for _, t := range targets {
		out = append(out, TargetView{Obj: t.Obj, Player: t.Player, IsPlayer: t.IsPlayer})
	}
	return out
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
