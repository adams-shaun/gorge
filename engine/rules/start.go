package rules

import (
	"errors"
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// StartSpec is a scripted start: the position a game opens in, applied
// inside genesis (Config.Start) so a replay from (Config, intents) rebuilds
// it and the setup events sit in the hash chain. A nil Config.Start is the
// ordinary genesis, byte for byte.
//
// Every card is an index into Config.Decks[seat]. The slices are ordered and
// map-free so the setup is deterministic. The deck is dealt and shuffled as
// usual, then applyStart rearranges it.
type StartSpec struct {
	Seats []StartSeat
}

// StartSeat is one seat's scripted position. A card listed in several of
// Hand, Battlefield, Graveyard, Exile and Library is placed once, by the
// first of them in that order; an index out of range, or one naming a card
// not in the seat's dealt library and hand (a commander), is skipped.
type StartSeat struct {
	// Hand is the opening hand, in hand order.
	Hand []int
	// Battlefield lists the permanents, in the order they enter (timestamp
	// order). An attached Aura (Attach) enters after every unattached
	// permanent of every seat.
	Battlefield []int
	// Graveyard is the graveyard, in the order cards enter it (first is the
	// oldest, bottom).
	Graveyard []int
	// Exile is the exile zone, in the order cards enter it.
	Exile []int
	// Library is the top of the library, top first.
	Library []int
	// Tapped and BackFace name battlefield cards that enter tapped / on
	// their back face.
	Tapped, BackFace []int
	// Counters are placed on battlefield cards, applied in the order given
	// (the caller sorts by (Card, Kind)).
	Counters []StartCounter
	// Attach binds a battlefield Aura of this seat to its bearer.
	Attach []StartAttach
	// Tokens are minted onto this seat's battlefield, in the order given,
	// after the seat's unattached Battlefield cards and before any attached
	// Aura. A token is not a deck card: Script names a Config.Tokens script.
	Tokens []StartToken
	// Life, when non-nil, is the seat's life total.
	Life *int32
}

// StartCounter puts N counters of Kind on the battlefield card Card.
type StartCounter struct {
	Card int
	Kind string
	N    int32
}

// StartToken is one token a scripted start places: the Config.Tokens
// script Script (an unknown script places nothing), entering tapped when
// Tapped, with Counters applied in the order given.
type StartToken struct {
	Script   string
	Tapped   bool
	Counters []StartTokenCounter
}

// StartTokenCounter puts N counters of Kind on a StartToken.
type StartTokenCounter struct {
	Kind string
	N    int32
}

// StartAttach enters the battlefield card Card (also listed in Battlefield)
// attached to the card To of seat ToSeat. The bearer must be a placed
// battlefield card, else the Aura enters unattached.
type StartAttach struct {
	Card   int
	ToSeat int
	To     int
}

// SeedForStarter returns the first seed >= seed whose CR 103.1 toss names
// seat want in a game of seats seats. The toss is the first rng draw of
// genesis, IntN(seats), so it depends on the seed and the seat count only.
// A want outside [0, seats) returns seed unchanged.
func SeedForStarter(seed uint64, seats, want int) uint64 {
	if seats <= 0 || want < 0 || want >= seats {
		return seed
	}
	for s := seed; ; s++ {
		if newRNG(s).IntN(seats) == want {
			return s
		}
	}
}

// startHost is the narrow view of the engine applyStart needs: the game,
// the one emit path, the replacement-pass guard and the end-of-setup
// settle. It holds no *Engine, so the engine's reach (engineSurface) does
// not grow.
type startHost struct {
	G         *state.Game
	Emit      func(events.Event) events.Event
	Replacing func(on bool) (prior bool) // sets applyingReplacement, returns the old value
	Settle    func()                     // drops the setup's queued triggers
}

// startBuilder carries the host through applyStart's passes; its methods
// are not Engine methods (the engine method count only shrinks).
type startBuilder struct{ h startHost }

// startSeatPlan is one seat's resolved placements.
type startSeatPlan struct {
	seat     state.PlayerID
	spec     *StartSeat
	ids      []state.ObjID
	rest     []state.ObjID // the library after the deal, shuffled order
	hand     []state.ObjID // the dealt hand, hand order
	placed   map[state.ObjID]bool
	hands    []state.ObjID
	field    []state.ObjID
	grave    []state.ObjID
	exile    []state.ObjID
	top      []state.ObjID
	attached map[state.ObjID]bool // Auras entering attached, placed after every seat
}

// applyStart rearranges the freshly dealt game into spec. ids[p] are seat
// p's deck objects in Config.Decks order. It emits, in three passes (all
// seats each): the placements, the attached Auras (a bearer may sit in
// another seat), then each library's complete order and each life total. Every
// trigger the setup queued is then dropped: a placed permanent counts as
// having been there since before the game (CR 302.6), and so the setup
// leaves nothing on the stack. Library order, per seat: the Library cards
// top first, then the shuffled remainder minus every placed card, then the
// unplaced opening-hand cards in hand order.
func applyStart(h startHost, spec *StartSpec, ids [][]state.ObjID) {
	b := startBuilder{h: h}
	plans := make([]startSeatPlan, 0, len(spec.Seats))
	for p := range spec.Seats {
		if p >= len(ids) || p >= len(h.G.Players) {
			break
		}
		plans = append(plans, b.planStartSeat(state.PlayerID(p), &spec.Seats[p], ids[p]))
	}
	for i := range plans {
		b.placeStartSeat(&plans[i])
	}
	for i := range plans {
		b.placeStartAttached(&plans[i], ids)
	}
	for i := range plans {
		b.finishStartSeat(&plans[i])
	}
	h.Settle()
}

// planStartSeat resolves a seat's index lists to objects, each card claimed
// once.
func (b startBuilder) planStartSeat(p state.PlayerID, s *StartSeat, ids []state.ObjID) startSeatPlan {
	h := b.h
	pl := startSeatPlan{seat: p, spec: s, ids: ids, placed: map[state.ObjID]bool{}}
	pl.rest = append([]state.ObjID(nil), h.G.Zone(state.ZLibrary, p)...)
	pl.hand = append([]state.ObjID(nil), h.G.Zone(state.ZHand, p)...)
	avail := map[state.ObjID]bool{}
	for _, id := range pl.rest {
		avail[id] = true
	}
	for _, id := range pl.hand {
		avail[id] = true
	}
	claim := func(list []int) []state.ObjID {
		var out []state.ObjID
		for _, idx := range list {
			if idx < 0 || idx >= len(ids) {
				continue
			}
			if id := ids[idx]; avail[id] && !pl.placed[id] {
				pl.placed[id] = true
				out = append(out, id)
			}
		}
		return out
	}
	pl.hands = claim(s.Hand)
	var permanents []int
	for _, idx := range s.Battlefield {
		if id := startObj(ids, idx); id != 0 {
			if o := h.G.Obj(id); o != nil && !startEntersAsPermanent(o.Card, slices.Contains(s.BackFace, idx)) {
				continue // CR 304.4, 307.4: never a permanent (Validate refuses it)
			}
		}
		permanents = append(permanents, idx)
	}
	pl.field = claim(permanents)
	pl.grave = claim(s.Graveyard)
	pl.exile = claim(s.Exile)
	pl.top = claim(s.Library)
	pl.attached = map[state.ObjID]bool{}
	return pl
}

// startObj is the object a seat-local deck index names, or 0.
func startObj(ids []state.ObjID, idx int) state.ObjID {
	if idx < 0 || idx >= len(ids) {
		return 0
	}
	return ids[idx]
}

// placeStartSeat returns the dealt hand to the library, then moves every
// placed card out of it: graveyard, exile and hand first (plain placements,
// no replacement rewrites them), then the unattached battlefield cards.
func (b startBuilder) placeStartSeat(pl *startSeatPlan) {
	h := b.h
	for _, id := range pl.hand {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	b.placeStartPlain(pl.grave, state.ZGraveyard)
	b.placeStartPlain(pl.exile, state.ZExile)
	b.placeStartPlain(pl.hands, state.ZHand)
	for _, a := range pl.spec.Attach {
		if id := startObj(pl.ids, a.Card); id != 0 && slices.Contains(pl.field, id) {
			pl.attached[id] = true
		}
	}
	for _, id := range pl.field {
		if !pl.attached[id] {
			b.placeStartBattlefield(pl, id, 0)
		}
	}
	b.placeStartTokens(pl)
}

// placeStartTokens mints pl's tokens onto its battlefield, then taps them
// and adds counters as placeStartBattlefield does. The mint runs with the
// replacement pass off (no CreateToken replacement such as Doubling Season
// rewrites a placement), and applyStart's Settle drops any trigger it
// queued. The mint's TokensCreatedThisTurn tally is not undone here: turn
// one's TurnChange (beginTurn, after applyStart) clears every seat's turn
// watchers, so a setup token never counts as created this turn.
func (b startBuilder) placeStartTokens(pl *startSeatPlan) {
	h := b.h
	for _, tk := range pl.spec.Tokens {
		next := h.G.NextID
		prior := h.Replacing(true)
		h.Emit(events.Event{Kind: events.TokenCreate, Player: pl.seat, Text: tk.Script})
		h.Replacing(prior)
		if h.G.NextID == next {
			continue // an unknown script mints nothing
		}
		id := next
		if o := h.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		if tk.Tapped {
			h.Emit(events.Event{Kind: events.Tap, Obj: id})
		}
		for _, c := range tk.Counters {
			if c.N != 0 {
				h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: c.Kind, Amount: c.N})
			}
		}
		h.Emit(events.Event{Kind: events.SetupSettled, Obj: id})
	}
}

// placeStartPlain moves ids from the library into zone with no replacement
// pass (CR 614): setup is placement, not an event in the game.
func (b startBuilder) placeStartPlain(ids []state.ObjID, zone state.Zone) {
	h := b.h
	prior := h.Replacing(true)
	defer h.Replacing(prior)
	for _, id := range ids {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: zone})
	}
}

// placeStartAttached enters pl's attached Auras, each on its bearer.
func (b startBuilder) placeStartAttached(pl *startSeatPlan, ids [][]state.ObjID) {
	h := b.h
	for _, id := range pl.field {
		if !pl.attached[id] {
			continue
		}
		var bearer state.ObjID
		for _, a := range pl.spec.Attach {
			if startObj(pl.ids, a.Card) != id || a.ToSeat < 0 || a.ToSeat >= len(ids) {
				continue
			}
			bearer = startObj(ids[a.ToSeat], a.To)
			break
		}
		if o := h.G.Obj(bearer); o == nil || o.Zone != state.ZBattlefield {
			bearer = 0
		}
		b.placeStartBattlefield(pl, id, bearer)
	}
}

// placeStartBattlefield enters one permanent: the back-face flip ahead of
// the move (as a transformed ChangeZone does, so the entry fold reads the
// back face), then tapped, counters and SetupSettled. A nonzero bearer
// enters an Aura attached to it.
func (b startBuilder) placeStartBattlefield(pl *startSeatPlan, id, bearer state.ObjID) {
	h := b.h
	idx := slices.Index(pl.ids, id)
	if slices.Contains(pl.spec.BackFace, idx) {
		// Placement is not a transform (CR 701.28).
		prior := h.Replacing(true)
		h.Emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1, Text: "Transformed"})
		h.Replacing(prior)
	}
	move := events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield}
	if bearer != 0 {
		events.MarkNamedAttachEntry(&move, []state.ObjID{bearer})
	}
	h.Emit(move)
	if o := h.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		return // refused on entry (an Aura with no bearer, CR 303.4g): it stays in the library
	}
	// A back-face planeswalker whose entering face prints no positive loyalty
	// is seated with the front face's printed starting loyalty, the same
	// convention the oracle runner uses (XMage's addCard); the spec's
	// counters are then added on top.
	if ev, ok := setupBackFaceLoyaltyCounter(h.G.Obj(id)); ok {
		h.Emit(ev)
	}
	if slices.Contains(pl.spec.Tapped, idx) {
		// Tapped is how the card is placed, not an action.
		h.Emit(events.Event{Kind: events.Tap, Obj: id})
	}
	for _, c := range pl.spec.Counters {
		if c.Card == idx && c.N != 0 {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: c.Kind, Amount: c.N})
		}
	}
	h.Emit(events.Event{Kind: events.SetupSettled, Obj: id})
}

// finishStartSeat emits the seat's complete library order and its life.
// Whether a card is still in the library is read from where it landed, not
// from the plan: a placement refused on entry leaves it there.
func (b startBuilder) finishStartSeat(pl *startSeatPlan) {
	h := b.h
	order := append([]state.ObjID(nil), pl.top...)
	inLibrary := func(id state.ObjID) bool {
		o := h.G.Obj(id)
		return o != nil && o.Zone == state.ZLibrary && !slices.Contains(pl.top, id)
	}
	for _, id := range pl.rest {
		if inLibrary(id) {
			order = append(order, id)
		}
	}
	for _, id := range pl.hand {
		if inLibrary(id) {
			order = append(order, id)
		}
	}
	h.Emit(events.Event{Kind: events.LibraryOrder, Player: pl.seat, IDs: order, Secret: true})
	if pl.spec.Life != nil {
		if d := *pl.spec.Life - h.G.Players[pl.seat].Life; d != 0 {
			// A setup life total is a stated fact, not a gain: no CR 614
			// replacement may rewrite it.
			prior := h.Replacing(true)
			h.Emit(events.Event{Kind: events.LifeChange, Player: pl.seat, Amount: d})
			h.Replacing(prior)
		}
	}
}

// ErrStartNotPermanent is Validate's refusal of a battlefield card whose
// entering face is an instant or sorcery (CR 304.4, 307.4).
var ErrStartNotPermanent = errors.New("not a permanent")

// Validate refuses a start no legal game state can hold: a Battlefield card
// whose entering face (the back face when BackFace names it) is not a
// permanent. decks is Config.Decks. applyStart degrades the same spec by
// leaving such a card unplaced (in the library), as it skips a bad index.
func (s *StartSpec) Validate(decks [][]*cards.Card) error {
	for p := range s.Seats {
		if p >= len(decks) {
			break
		}
		seat := &s.Seats[p]
		for _, idx := range seat.Battlefield {
			if idx < 0 || idx >= len(decks[p]) {
				continue
			}
			if c := decks[p][idx]; !startEntersAsPermanent(c, slices.Contains(seat.BackFace, idx)) {
				return fmt.Errorf("start: seat %d battlefield card %d (%s): %w", p, idx, c.Faces[0].Name, ErrStartNotPermanent)
			}
		}
	}
	return nil
}

// startEntersAsPermanent reports whether c, entering on its back face when
// back is set, is a permanent. A card with no faces is not refused.
func startEntersAsPermanent(c *cards.Card, back bool) bool {
	if c == nil || len(c.Faces) == 0 {
		return true
	}
	f := c.Faces[0]
	if back && len(c.Faces) > 1 {
		f = c.Faces[1]
	}
	return f == nil || f.IsPermanent()
}
