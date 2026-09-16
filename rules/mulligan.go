package rules

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// mulliganRound is the London mulligan round's plain-value state, built by
// New between the opening deal and turn 1 and driven one decision at a time
// by stepPregame (rules/turn.go's step dispatches here while e.pregame). It
// is plain data, never a closure, so a clone copies it like cast/choosing.
//
// seats is the round order -- e.G.AliveFrom(startingPlayer) at New time, a
// slice, deterministic: CR 103.5 has the STARTING PLAYER declare first and
// each other player follow in turn order, so the round starts at the toss
// winner, not at seat 0 (kept[i], taken[i] and seats[i] correspond by
// position). limit is Config.Mulligans, which is the PERMITTED
// COUNT -- the most mulligans one seat may take (Ruling FL-113, rule R-8.4).
// It is NOT the CR 103.5b multiplayer free mulligan, which is derived from the
// seat count and exempts a seat from the bottoming PENALTY, never from the
// allowance; an earlier version of this comment called limit "the
// free-mulligan count" and that wording was wrong. freeMulligans is fixed from
// the round's initial seat count. bottom is false during the keep/mulligan
// phase and true during the bottoming phase; cursor names the next seat to ask
// in whichever phase.
// openingEffect is one optional !PlayFirst opening-hand SVar. It is plain
// value data so clone/replay can preserve a decision waiting before the London
// round; the SA points into the immutable compiled corpus.
type openingEffect struct {
	source     state.ObjID
	controller state.PlayerID
	sa         *cards.SA
}

// openingHost runs a pregame SVar outside the stack. Its outer may decision is
// real; a nested effect chooser has no stack object to resume against, so it
// takes effects' established no-host deterministic fallback instead of
// creating an unresumable decision. Embedding keeps every state-changing call
// on Engine's normal events.Emit path.
type openingHost struct{ *Engine }

func (openingHost) Ask(*decision.Decision) bool            { return false }
func (openingHost) Suspended() bool                        { return false }
func (openingHost) SuspendContinuation(*cards.SA)          {}
func (openingHost) SuspendRepeat(effects.RepeatSuspension) {}

type mulliganRound struct {
	seats         []state.PlayerID
	kept          []bool
	taken         []int
	limit         int
	freeMulligans int
	bottom        bool
	cursor        int
}

// PregameStarter supplies the resolved CR 103.1 starting seat while the
// London mulligan round is live. It deliberately lives outside state.Game:
// the ordinary TurnChange will record Active at turn 1, and adding a separate
// genesis event solely for this transient projection would alter every replay.
func (e *Engine) PregameStarter() (state.PlayerID, bool) {
	if e == nil || !e.pregame || len(e.mulligan.seats) == 0 {
		return 0, false
	}
	return e.mulligan.seats[0], true
}

func newMulliganRound(seats []state.PlayerID, limit int) mulliganRound {
	freeMulligans := 0
	if len(seats) >= 3 {
		freeMulligans = 1
	}
	return mulliganRound{
		seats: seats, kept: make([]bool, len(seats)), taken: make([]int, len(seats)),
		limit: limit, freeMulligans: freeMulligans,
	}
}

// bottomCount is the London bottoming penalty after the multiplayer free
// mulligan exemption. The exemption changes only this penalty, never limit.
func (m *mulliganRound) bottomCount(i int) int {
	return max(0, m.taken[i]-m.freeMulligans)
}

// stepPregame issues the single next pregame decision -- one keep/mulligan
// ask, one bottoming ask, or the hand-off to beginTurn -- and returns. step()
// calls it exactly once per engine step and returns right after, so Advance's
// loop issues the round one decision at a time, each Submit answering the
// previous one; there is never more than one pregame decision pending.
func (e *Engine) stepPregame() {
	if e.G.Over {
		return
	}
	// An opening-hand !PlayFirst effect (Impatient Iguana, Gemstone Caverns)
	// must see the toss designation before any mulligan declaration. Its outer
	// may choice is explicit; the effect itself is the linked Forge SVar.
	if e.openingCursor < len(e.opening) {
		e.askOpeningEffect(e.opening[e.openingCursor])
		return
	}
	m := &e.mulligan
	if m.seats == nil {
		if e.openingLimit == 0 {
			e.pregame = false
			e.beginTurn(e.G.StartingPlayer)
			return
		}
		mCopy := newMulliganRound(e.G.AliveFrom(e.G.StartingPlayer), e.openingLimit)
		*m = mCopy
	}
	if m.bottom {
		// Bottoming phase: each seat bottoms its penalty count from its kept
		// hand (the London end-of-round bottoming). A seat whose penalty is
		// zero, including its first mulligan in multiplayer, is skipped.
		for m.cursor < len(m.seats) {
			i := m.cursor
			if m.bottomCount(i) == 0 {
				m.cursor++
				continue
			}
			e.askBottoming(i)
			return
		}
		// Every seat that had something to bottom has bottomed: the round is
		// over and the round's first seat -- m.seats[0], the toss winner the
		// round was built from, not always seat 0 since the CR 103.1 toss --
		// begins turn 1, exactly as before.
		e.pregame = false
		e.beginTurn(m.seats[0])
		return
	}
	// Keep/mulligan phase: ask each not-yet-kept seat, in round order,
	// whether to keep or (while it still has a permitted mulligan) mulligan.
	for m.cursor < len(m.seats) {
		i := m.cursor
		if m.kept[i] {
			m.cursor++
			continue
		}
		e.askKeepMulligan(i)
		return
	}
	// CR 103.5 is ROUND-ROBIN: every un-kept player has declared once before
	// any player who mulliganed declares again. If this pass has mulliganers,
	// restart at its first seat; only a pass in which everybody keeps reaches
	// London bottoming.
	for _, kept := range m.kept {
		if !kept {
			m.cursor = 0
			e.stepPregame()
			return
		}
	}
	// Every seat has kept: move to the bottoming phase.
	m.bottom = true
	m.cursor = 0
	e.stepPregame()
}

// askKeepMulligan offers seat i of the round a keep/mulligan decision. It is
// Min == Max == 1 over the same distinct-index shape Validate enforces
// everywhere (Ruling U2). While the seat still has a permitted mulligan
// (taken < limit) it offers both the "keep" and "mulligan" options; once it
// has used its whole allowance, London offers only "keep" -- you keep what
// you have.
// putCount is the number phrase for a London bottoming penalty: "1 card" or
// "2 cards" -- real English singular and plural, not "(s)" (finding bh). It
// is the human-facing count every mulligan prompt embeds.
func putCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return fmt.Sprintf("%d cards", n)
}

// bottomingPrompt is the human-readable wording for a London bottoming ask:
// `taken` cards leave the kept hand for the bottom of the library. It is the
// last real sentence a player reads in the mulligan round, so it is written
// for a human -- finding bh: no "(s)", no "bottoms" as a verb.
func bottomingPrompt(bottom int) string {
	if bottom == 0 {
		return "Keep all seven cards"
	}
	return fmt.Sprintf("Put %s on the bottom of your library", putCount(bottom))
}

// keepMulliganPrompt is the human-readable wording for a keep/mulligan ask.
// It names the starting player first (fix round rv2a: the ask is the one
// always-visible surface a seat reads before deciding keep/mulligan -- the
// transcript that carries the toss Note starts hidden for a seated player,
// and CR 103.5 runs the round from the starter, so the seat being asked is
// not always the one who plays first) and then the bottoming penalty a keep
// accepts, in the same real English as bottomingPrompt (finding bh: the old
// "keeps 7 and bottoms 1, or mulligans" was engine-speak). With a permitted
// mulligan remaining the seat has a choice; once the allowance is spent
// London offers only a keep. The prompt is chain-free wire text, so it names
// the starter with their display PlayerName when present (falling back to the
// deck identity); deck identities are not unique at a table and therefore
// cannot tell a seated human who plays first.
func keepMulliganPrompt(starterName string, bottom, taken, limit, freeMulligans int) string {
	penalty := fmt.Sprintf("put %s on the bottom of your library", putCount(bottom))
	if bottom == 0 && freeMulligans > 0 {
		penalty = "keep all seven cards"
	}
	var choice string
	if taken < limit {
		choice = fmt.Sprintf("Keep your hand (%s) or take a mulligan?", penalty)
	} else {
		choice = fmt.Sprintf("Keep your hand (%s)", penalty)
	}
	return starterName + " plays first. " + choice
}

// mulliganStarterName is the seat-facing identity for the pregame prompt.
// PlayerName is supplied by the table and identifies a human even when two
// players chose the same deck; Name is the deterministic fallback for bots or
// callers without display names. The final fallback is defensive: mulligan
// seats originate from AliveFrom, but malformed state must not panic while
// constructing a client decision.
func mulliganStarterName(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) {
		if name := g.Players[p].PlayerName; name != "" {
			return name
		}
		if name := g.Players[p].Name; name != "" {
			return name
		}
	}
	return fmt.Sprintf("seat %d", p)
}

func (e *Engine) askKeepMulligan(i int) {
	m := &e.mulligan
	p := m.seats[i]
	opts := []decision.Option{{Index: 0, Kind: "keep", Label: "keep"}}
	if m.taken[i] < m.limit {
		opts = append(opts, decision.Option{Index: 1, Kind: "mulligan", Label: "mulligan"})
	}
	// CR 103.4: the seat re-drew a full openingHand on every mulligan, so
	// while it is deciding it always holds seven and will bottom bottomCount
	// cards if it keeps -- the bottoming is the entire penalty. The prompt
	// names who plays first (m.seats[0], the round's own starting seat --
	// the toss winner AliveFrom(start) begins with), so the decision is
	// made knowing play/draw without opening the transcript.
	e.ask(decision.New(p, decision.KMulligan,
		keepMulliganPrompt(mulliganStarterName(e.G, m.seats[0]), m.bottomCount(i), m.taken[i], m.limit, m.freeMulligans), 1, 1, opts))
}

// askBottoming offers seat i a bottoming decision over its kept hand: one
// "bottom" option per card, Min == Max == bottomCount(i) -- exactly the
// distinct-index shape Validate already enforces for KTriggerOrder (a
// bottoming choice is a permutation of hand indices; Ruling U2).
func (e *Engine) askBottoming(i int) {
	m := &e.mulligan
	p := m.seats[i]
	hand := e.G.Zone(state.ZHand, p)
	opts := make([]decision.Option, len(hand))
	for j, id := range hand {
		opts[j] = decision.Option{Index: j, Kind: "bottom",
			Label: e.G.Obj(id).Face().Name, Obj: id, Player: p}
	}
	bottom := m.bottomCount(i)
	e.ask(decision.New(p, decision.KMulligan, bottomingPrompt(bottom), bottom, bottom, opts))
}

// handleMulligan applies a KMulligan answer. In the keep/mulligan phase (the
// round's first half, e.mulligan.bottom false) a keep marks the seat kept; a
// mulligan shuffles the seat's hand back into its library (Shuffle, secret),
// takes a permitted mulligan and draws a full new hand of seven -- CR 103.4: the
// bottoming of `taken` cards at the round's end is the entire penalty, never
// a smaller redraw. Every mutation is an e.emit or drawCard, so the whole
// round is event-driven and replays byte-for-byte. In the bottoming phase it
// moves each chosen card to its library bottom and advances the round past
// this seat.
func (e *Engine) handleMulligan(d *decision.Decision, in decision.Intent) {
	if e.mulligan.bottom {
		e.handleBottoming(d, in)
		return
	}
	// Resolve the round index from the answering seat rather than the
	// cursor (Finding 3, fix round 1): d.Player is the authority -- Submit
	// has already rejected any answer whose Player is not the pending
	// decision's, and askKeepMulligan addressed that decision to
	// seats[cursor] -- so the two always agree. Deriving the index from the
	// seat keeps the seats slice the round's single mapping from seat to
	// kept/taken bucket and removes the cursor coupling for free.
	i := -1
	for j, s := range e.mulligan.seats {
		if s == d.Player {
			i = j
			break
		}
	}
	p := d.Player
	chosen := d.Chosen(in)
	if len(chosen) > 0 && chosen[0].Kind == "keep" {
		e.mulligan.kept[i] = true
		return
	}
	// A mulligan: the seat stays un-kept (it must decide again on a later
	// PASS, after every other un-kept seat declares once). Advance cursor now;
	// stepPregame resets it only after the current round has completed. taken
	// increments first; once it reaches limit the next-pass ask offers keep.
	e.mulligan.taken[i]++
	e.mulligan.cursor++
	for _, id := range e.G.Zone(state.ZHand, p) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand,
			To: state.ZLibrary, Player: p, Text: "mulligan"})
	}
	order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
	e.rng.Shuffle(order)
	e.emit(events.Event{Kind: events.Shuffle, Player: p, IDs: order, Secret: true})
	// CR 103.4: a mulligan draws a FULL new hand of seven -- the later
	// bottoming is the entire penalty, and the redraw is literally the same
	// loop genesis already runs in New's opening deal. If the seat decks out
	// mid-redraw (drawCard's own checkStateBased sets Over; reachable only
	// from a hand-made tiny deck) stop drawing: the round respects Over
	// everywhere else and must here too.
	for j := 0; j < openingHand && !e.G.Over; j++ {
		e.drawCard(p)
	}
}

// handleBottoming moves each card the bottoming answer chose to the bottom
// of its owner's library -- a library's bottom is its last element (Move
// appends to the destination zone's end) -- in the order the client
// submitted them, then advances the round past this seat.
// openingEffects finds every current opening-hand effect whose Forge keyword
// carries !PlayFirst. The condition is evaluated from state.Game, not a toss
// Note or an assumed seat zero, so the next card using the same keyword has
// the same gate. Effects and seats are collected in turn order and hand order
// only; neither source can reach a map iteration.
func (e *Engine) openingEffects() []openingEffect {
	var out []openingEffect
	for _, p := range e.G.AliveFrom(e.G.StartingPlayer) {
		if e.G.IsStartingPlayer(p) {
			continue
		}
		for _, id := range e.G.Zone(state.ZHand, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			for _, keyword := range o.Face().Keywords {
				if cards.KeywordHead(keyword) != "MayEffectFromOpeningHand" {
					continue
				}
				param, ok := o.Face().KeywordParam("MayEffectFromOpeningHand")
				if !ok {
					continue
				}
				parts := strings.Split(param, ":")
				if len(parts) < 2 || !strings.EqualFold(strings.TrimSpace(parts[len(parts)-1]), "!PlayFirst") {
					continue
				}
				sa := cards.ResolveSVar(o.Face().SVars, strings.TrimSpace(parts[0]))
				if sa != nil {
					out = append(out, openingEffect{source: id, controller: p, sa: sa})
				}
			}
		}
	}
	return out
}

func (e *Engine) askOpeningEffect(effect openingEffect) {
	o := e.G.Obj(effect.source)
	label := "Use opening-hand effect"
	if o != nil && o.Face() != nil {
		label = "Use " + o.Face().Name + "'s opening-hand effect"
	}
	e.ask(&decision.Decision{Player: effect.controller, Kind: decision.KChoose,
		ResumeKind: "opening_effect", Source: effect.source,
		Prompt: label + "?", Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "yes", Label: "Yes", Obj: effect.source, Player: effect.controller},
			{Index: 1, Kind: "no", Label: "No", Obj: effect.source, Player: effect.controller}}})
}

func (e *Engine) handleOpeningEffect(d *decision.Decision, in decision.Intent) {
	if e.openingCursor >= len(e.opening) {
		return
	}
	effect := e.opening[e.openingCursor]
	chosen := d.Chosen(in)
	if len(chosen) == 1 && chosen[0].Kind == "yes" {
		o := e.G.Obj(effect.source)
		if o != nil && o.Zone == state.ZHand {
			ctx := &effects.Ctx{Source: effect.source, Controller: effect.controller, SVars: o.Face().SVars}
			effects.Resolve(openingHost{e}, ctx, effect.sa)
		}
	}
	e.openingCursor++
}

func (e *Engine) handleBottoming(d *decision.Decision, in decision.Intent) {
	e.mulligan.cursor++
	for _, o := range d.Chosen(in) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.Obj, From: state.ZHand,
			To: state.ZLibrary, Player: d.Player, Text: "bottomed"})
	}
}
