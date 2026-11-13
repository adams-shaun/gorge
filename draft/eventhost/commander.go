package eventhost

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// The Commander-draft event (Mystery Booster Commander): every seat drafts
// commanderPicksPerPass cards from each pack before passing it, builds a
// build.CommanderDeckSize deck around one or two drafted commanders, and the
// whole pod plays one free-for-all Commander game per event round at a
// single host table (event.StructurePod). A pod game logs only its
// PodGameEnded; the in-flight table is run memory, so a restart starts the
// round's game again on a fresh table.
const (
	formatCommander       = event.FormatCommander
	commanderPicksPerPass = 2
	// commanderDrafter is the persona name a Commander-draft bot seat
	// shows: its drafter and builder are pickbot's Commander pair whatever
	// the create request named.
	commanderDrafter = "commander"
)

// commanderSets are the sets this server hosts a Commander draft of: the
// Commander-draft products whose set data carries their own draft booster.
var commanderSets = []string{"MBC"}

// hostsCommander reports whether a set may be drafted as a Commander draft.
func hostsCommander(code string) bool {
	for _, c := range commanderSets {
		if c == code {
			return true
		}
	}
	return false
}

// podPolicy refuses a bot game policy that cannot sit at the pod's
// four-seat table.
func (h *Host) podPolicy(policy string) *Error {
	if e, ok := bots.Lookup(policy); ok && e.MaxSeats != 0 && e.MaxSeats < draft.Seats {
		return errBadRequest("eventhost: policy %q seats at most %d; a Commander draft plays %d seats at one table", policy, e.MaxSeats, draft.Seats)
	}
	return nil
}

// commander reports a Commander-draft event.
func (m meta) commander() bool { return m.Format == formatCommander }

// commanderPersona is a Commander-draft seat's drafter and builder.
func commanderPersona(lookup pickbotCardLookup) (pickbotDrafter, pickbotBuilder) {
	return &pickbot.CommanderDrafter{Lookup: lookup}, &pickbot.CommanderBuilder{Lookup: lookup}
}

// commanderCard is one pool card's Commander-draft facts.
type commanderCard struct {
	identity  uint8
	commander bool // can be a commander on its own (CR 903.3)
	partner   bool // can be half of a Commander-draft pair
}

// commanderFacts reads one card's Commander-draft facts from the corpus; a
// card the corpus does not know is no commander and has no identity.
func (r *run) commanderFacts(name string) (commanderCard, *cards.Card) {
	c, ok := r.h.opts.Cards.Lookup(name)
	if !ok || c == nil || len(c.Faces) == 0 {
		return commanderCard{}, nil
	}
	return commanderCard{
		identity:  c.ColourIdentity(),
		commander: deck.IsCommanderEligible(c),
		partner:   deck.IsDraftPairHalf(c),
	}, c
}

// validateCommanderDeck checks a Commander-draft deck against the seat's
// pool (build.ValidateCommander) and the card facts the event package cannot
// see: one commander-eligible card or a Commander-draft pair
// (deck.IsDraftCommanderPair), and every non-basic maindeck card inside the
// commanders' colour identity (CR 903.4). It returns that identity.
func (r *run) validateCommanderDeck(seat int, d build.Deck) (uint8, error) {
	pool := r.ev.Pool(seat)
	ident, err := r.commanderIdentity(pool, d.Commanders)
	if err != nil {
		return ident, err
	}
	if err := build.ValidateCommander(pool, d); err != nil {
		return ident, err
	}
	for _, id := range d.Main {
		ref, ok := refByID(pool, id)
		if !ok {
			continue // ValidateCommander refused it already
		}
		f, c := r.commanderFacts(ref.Name)
		if c == nil {
			return ident, fmt.Errorf("%s is not a card this server knows", ref.Name)
		}
		if f.identity&^ident != 0 {
			return ident, fmt.Errorf("%s (%s) is outside the commanders' colour identity (%s)", ref.Name, wubrgOf(f.identity), identityText(ident))
		}
	}
	for i, n := range d.Basics {
		if n > 0 && ident&(1<<i) == 0 {
			return ident, fmt.Errorf("%s is outside the commanders' colour identity (%s)", build.BasicNames[i], identityText(ident))
		}
	}
	return ident, nil
}

// commanderIdentity checks the named commanders (one eligible card, or two
// forming a Commander-draft pair) and returns their combined identity.
func (r *run) commanderIdentity(pool []limited.CardRef, ids []string) (uint8, error) {
	var cs []*cards.Card
	var ident uint8
	for _, id := range ids {
		ref, ok := refByID(pool, id)
		if !ok {
			return 0, fmt.Errorf("commander %q is not in your pool", id)
		}
		f, c := r.commanderFacts(ref.Name)
		if c == nil {
			return 0, fmt.Errorf("commander %s is not a card this server knows", ref.Name)
		}
		ident |= f.identity
		cs = append(cs, c)
	}
	switch len(cs) {
	case 0:
		return 0, errors.New("choose a commander")
	case 1:
		if !deck.IsCommanderEligible(cs[0]) {
			return ident, fmt.Errorf("%s cannot be your commander", cs[0].Faces[0].Name)
		}
	case 2:
		if !deck.IsDraftCommanderPair(cs[0], cs[1]) {
			return ident, fmt.Errorf("%s and %s are not a commander pair: both need partner, or both must be legends of at most one colour without a partner ability", cs[0].Faces[0].Name, cs[1].Faces[0].Name)
		}
	default:
		return ident, fmt.Errorf("%d commanders, want 1 or 2", len(cs))
	}
	return ident, nil
}

// identityText spells an identity for an error message.
func identityText(m uint8) string {
	if m == 0 {
		return "colourless"
	}
	return wubrgOf(m)
}

// refByID finds a pool card by id.
func refByID(pool []limited.CardRef, id string) (limited.CardRef, bool) {
	for _, c := range pool {
		if c.ID == id {
			return c, true
		}
	}
	return limited.CardRef{}, false
}

// commanderAutoLands is the basics a Commander-draft main needs to reach
// build.CommanderDeckSize, split by pip share over the commanders' identity
// (every basic outside it would make the deck illegal). An identity with no
// pips in the main splits evenly over its colours; a colourless identity
// gets none.
func (r *run) commanderAutoLands(seat int, main []string, ident uint8) [5]int {
	pips := r.pipsOf(seat)
	masked := func(id string) [5]int {
		p := pips(id)
		for i := range p {
			if ident&(1<<i) == 0 {
				p[i] = 0
			}
		}
		return p
	}
	out := build.AutoLandsTo(main, masked, build.CommanderDeckSize)
	if out != ([5]int{}) || ident == 0 {
		return out
	}
	lands := build.CommanderDeckSize - len(main)
	n := bits.OnesCount8(ident)
	if lands <= 0 || n == 0 {
		return out
	}
	k := 0
	for i := range out {
		if ident&(1<<i) == 0 {
			continue
		}
		out[i] = lands / n
		if k < lands%n {
			out[i]++
		}
		k++
	}
	return out
}

// commanderDeckCheck is the DeckCheck body for a Commander-draft deck.
func (r *run) commanderDeckCheck(seat int, req protocol.DeckRequest) protocol.DeckCheck {
	d := build.Deck{Main: req.Main, Basics: req.Basics, Commanders: req.Commanders}
	out := protocol.DeckCheck{Total: d.Total(), Size: build.CommanderDeckSize}
	ident, err := r.validateCommanderDeck(seat, d)
	out.Identity = wubrgOf(ident)
	if len(req.Commanders) > 0 {
		out.AutoLands = r.commanderAutoLands(seat, req.Main, ident)
	}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Valid = true
	return out
}

// commanderCards adds the Commander-draft facts to a wire card list.
func (r *run) commanderCards(out []protocol.DraftCard) {
	for i := range out {
		f, c := r.commanderFacts(out[i].Name)
		if c == nil {
			continue
		}
		out[i].Identity = wubrgOf(f.identity)
		out[i].Commander = f.commander
		out[i].Partner = f.partner
	}
}

// scaledCardsLeft maps the cards left in a pack onto the 15-card pick-limit
// table, so a pack larger than 15 cards (MBC's 20) opens at the table's
// full-pack limit and ends at its last-card limit.
func scaledCardsLeft(left, packSize int) int {
	if packSize <= 15 || left <= 0 {
		return left
	}
	return (left*15 + packSize - 1) / packSize
}

// adviser is a drafter that can rate a pack for a player: the Commander
// drafter (pickbot.CommanderDrafter.Rank).
type adviser interface {
	Rank(v pickbot.PickView) ([]pickbot.PickAdvice, uint8)
}

// DraftAdvice is the seat's own drafter's suggestion for the pack at its
// queue head: the best cards for the picks the seat still owes this pass,
// with their reasons. It reads only the seat's own pack and pool, logs
// nothing and is not an action (no idle-clock refresh). An event whose
// drafter cannot explain itself answers conflict.
func (h *Host) DraftAdvice(id, token string) (protocol.DraftAdvice, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.DraftAdvice{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.DraftAdvice{}, terr
	}
	adv, ok := r.drafters[seat].(adviser)
	if !ok {
		return protocol.DraftAdvice{}, errConflict("eventhost: this event's drafter gives no advice")
	}
	out := protocol.DraftAdvice{Event: r.meta.ID, Seat: seat}
	v, ok := r.ev.Next(seat)
	if r.ev.Phase() != event.PhaseDraft || !ok {
		out.Waiting = true
		return out, nil
	}
	out.Round, out.PickNo = v.Round, v.PickNo
	ranked, plan := adv.Rank(v)
	out.Plan = wubrgOf(plan)
	order := make([]int, len(ranked))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return ranked[order[a]].Score > ranked[order[b]].Score })
	want := 1
	if ppp := r.ev.PicksPerPass(); ppp > 1 {
		want = ppp - (v.PickNo-1)%ppp
	}
	for _, i := range order {
		if len(out.Picks) == want {
			break
		}
		a := ranked[i]
		out.Picks = append(out.Picks, protocol.DraftAdvicePick{
			ID: a.ID, Score: a.Score, Value: a.Value, Commander: a.Commander, Identity: a.Identity, Reason: a.Reason,
		})
	}
	return out, nil
}

// podGameSeed derives one pod game attempt's table seed from the event
// seed: gameSeed's stream with both seats 0, a pairing no series plays.
func podGameSeed(seed uint64, round, attempt int) uint64 {
	return gameSeed(seed, round, 0, 0, 0, attempt)
}

// podTableID names one pod game attempt's OnDemand table.
func podTableID(ev string, round, attempt int) host.TableID {
	return host.TableID(fmt.Sprintf("%s-r%d-pod-a%d", ev, round, attempt))
}

// podLive is the in-flight pod game of a Commander-draft event: its round,
// attempt and table, and each human seat's game token.
type podLive struct {
	round   int
	attempt int
	table   host.TableID
	tokens  [draft.Seats]string
}

// advancePod drives a Commander-draft event's rounds: it starts the pending
// round's free-for-all table when none is live, and closes the event once
// the last round's game is logged. The caller holds r.mu.
func (r *run) advancePod() bool {
	if !strings.HasPrefix(string(r.ev.Phase()), "ROUND") {
		return false
	}
	r.h.mu.Lock()
	closing := r.h.closed
	r.h.mu.Unlock()
	if closing || r.blocked != "" {
		return false
	}
	round, ok := r.ev.PendingPodGame()
	if !ok {
		if err := r.apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
			return false
		}
		return true
	}
	if r.pod != nil && r.pod.round == round {
		return false
	}
	if err := r.startPodGame(round); err != nil {
		return false
	}
	return false // a started table logs nothing
}

// startPodGame seats every event seat at one Commander table: event seat s
// plays table seat s (match 1 deals Decks[(s+1)%4], so the list is rotated
// back by one), 40 life, the commanders from each seat's readied deck and
// the Commander-draft partner rule. A failure blocks the event (no game
// starts until a restart) and returns the error.
func (r *run) startPodGame(round int) error {
	o := r.h.opts
	// The attempt counter is run memory: after a restart it starts
	// again, so an id a table of this process still holds is skipped
	// (AddTable refuses a duplicate) by bumping the attempt.
	attempt := r.podAttempts[round] + 1
	id := podTableID(r.meta.ID, round, attempt)
	r.podAttempts[round] = attempt
	seed := podGameSeed(r.meta.Seed, round, attempt)
	live := &podLive{round: round, attempt: attempt, table: id}
	fail := func(err error) error {
		r.h.mu.Lock()
		delete(r.h.tables, id)
		r.h.mu.Unlock()
		r.pod = nil
		r.blocked = err.Error()
		return err
	}
	n := draft.Seats
	decks := make([]string, n)
	names := make([]string, n)
	policies := make([]string, n)
	var humans []int
	for s := 0; s < n; s++ {
		decks[(s+1)%n] = DeckID(r.meta.ID, s, r.ev.ReadyRev(s))
		names[s] = r.nameOf(s)
		policies[s] = r.policyOrEmpty(s)
		if r.meta.Seats[s].Kind == "human" {
			humans = append(humans, s)
		}
	}
	for _, s := range humans {
		if o.MintSeat == nil {
			return fail(errors.New("eventhost: Options.MintSeat is required to seat a human"))
		}
		tok, err := o.MintSeat(id, s)
		if err != nil {
			return fail(err)
		}
		live.tokens[s] = tok
	}
	r.h.mu.Lock()
	r.h.tables[id] = gameRef{run: r, round: round, game: 1, attempt: attempt, pod: true}
	r.h.mu.Unlock()
	r.pod = live
	cfg := host.TableConfig{
		ID:              id,
		Name:            fmt.Sprintf("%s commander game %d", r.meta.ID, round),
		Seats:           n,
		Decks:           decks,
		Seed:            seed,
		PlayerNames:     names,
		SeatBotPolicies: policies,
		Humans:          humans,
		BotPolicy:       o.DefaultPolicy,
		Spectator:       view.Public,
		Format:          host.FormatCommander,
		DraftPartners:   true,
		Mulligans:       o.Mulligans,
		AutoMana:        o.AutoMana,
		BotAutoPayMana:  o.BotAutoPayMana,
		BotCommonSense:  o.BotCommonSense,
		OnDemand:        true,
		Event:           r.meta.ID,
	}
	err := o.Registry.AddTable(cfg)
	for tries := 0; err != nil && strings.Contains(err.Error(), "already exists") && tries < 16; tries++ {
		r.h.mu.Lock()
		delete(r.h.tables, id)
		attempt++
		id = podTableID(r.meta.ID, round, attempt)
		r.h.tables[id] = gameRef{run: r, round: round, game: 1, attempt: attempt, pod: true}
		r.h.mu.Unlock()
		r.podAttempts[round] = attempt
		live.attempt, live.table = attempt, id
		cfg.ID = id
		if len(humans) > 0 {
			// The minted game tokens name the old table id.
			for _, s := range humans {
				tok, merr := o.MintSeat(id, s)
				if merr != nil {
					return fail(merr)
				}
				live.tokens[s] = tok
			}
		}
		err = o.Registry.AddTable(cfg)
	}
	if err != nil {
		return fail(err)
	}
	if err := o.Registry.Start(id); err != nil {
		return fail(err)
	}
	return nil
}

// podGameOver logs one finished pod game, or retries a crashed one: past
// MaxAttempts the round logs as a draw so the event goes on. A callback for
// a table that is not the live pod game is dropped. The caller holds r.mu.
func (r *run) podGameOver(ref gameRef, m protocol.MatchInfo) {
	if r.pod == nil || string(r.pod.table) != m.Table {
		return
	}
	r.h.mu.Lock()
	delete(r.h.tables, host.TableID(m.Table))
	r.h.mu.Unlock()
	if r.podTables == nil {
		r.podTables = map[int]string{}
	}
	r.podTables[ref.round] = m.Table
	r.pod = nil
	en := event.Entry{Kind: event.KindPodGameEnded, Round: ref.round, Head: m.Head}
	switch {
	case m.State == protocol.MatchFinished:
		if m.Winner != nil {
			w := int(*m.Winner)
			en.Winner = &w
		}
	case r.podAttempts[ref.round] < r.maxAttempts():
		// A crashed attempt: the next advance starts the round again on a
		// fresh table.
		r.advance()
		r.broadcast()
		return
	default:
		en.Head = "crashed"
	}
	if err := r.apply(en); err != nil {
		return
	}
	r.advance()
	r.broadcast()
}

// podRows is a Commander-draft event's pod-game rows for a viewer: one per
// event round, ended from the log, live from the in-flight table (with the
// viewer's own join link), pending otherwise.
func (r *run) podRows(seat int) []protocol.EventPodGame {
	var out []protocol.EventPodGame
	ended := map[int]event.Entry{}
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPodGameEnded {
			ended[en.Round] = en
		}
	}
	for rd := 1; rd <= r.ev.Rounds(); rd++ {
		row := protocol.EventPodGame{Round: rd, State: "pending"}
		if en, ok := ended[rd]; ok {
			row.State = "ended"
			row.Table = r.podTables[rd]
			if en.Winner != nil {
				w := *en.Winner
				row.Winner = &w
			}
		} else if r.pod != nil && r.pod.round == rd {
			row.State = "live"
			row.Table = string(r.pod.table)
			if seat >= 0 && seat < draft.Seats && r.pod.tokens[seat] != "" {
				row.Join = fmt.Sprintf("/t/%s?seat=%d&token=%s&event=%s", r.pod.table, seat, r.pod.tokens[seat], r.meta.ID)
			}
		}
		out = append(out, row)
	}
	return out
}
