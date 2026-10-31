package eventhost

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// Options configures a Host.
type Options struct {
	Registry       *host.Registry
	Store          *Store
	Cards          *cards.Registry
	Policies       []string // bot game policies offered; empty = every non-Env policy
	DefaultPolicy  string   // "" = bots.Default
	MaxHumans      int      // 0 = 4 (E3-5)
	MaxLiveEvents  int      // 0 = 4
	MaxAttempts    int      // 0 = 3 (E1-5 uses this)
	Mulligans      int
	AutoMana       bool
	BotAutoPayMana bool
	BotCommonSense bool
	MintSeat       func(table host.TableID, seat int) (string, error) // E1-5
	// PickTimeout, when positive, replaces the standard per-pick limit
	// table for every pick-timer mode (E3-6); 0 uses the table.
	PickTimeout time.Duration
	// Clock is the pick-timer seam (E3-6): Now reads the time the armed
	// timers and their pack views count against, AfterFunc arms one; nil
	// is the real clock.
	Clock interface {
		Now() time.Time
		AfterFunc(d time.Duration, f func()) (stop func() bool)
	}

	// Seed mints every new event's seed (E1-8: the seed is never a client
	// field). nil = crypto/rand.
	Seed func() uint64
	// LobbyTimeout is how long an event may sit in LOBBY with an unjoined
	// human seat before the sweep acts: at least one human joined fills
	// every unjoined seat with a bot and starts the event, zero joined
	// cancels it (E3-5). 0 = 15 minutes, negative =
	// disabled. IdleTimeout is how long a human seat may owe a decision
	// without acting before the sweep acts for it (auto-pick, auto-submit
	// or concede): 0 = 2 hours, negative = disabled.
	LobbyTimeout time.Duration
	IdleTimeout  time.Duration
	// Now is the clock the timeouts read; nil = time.Now (tests inject one
	// and drive the sweep with checkTimeouts directly).
	Now func() time.Time
}

// The timeout defaults and the sweep cadence (E1-8).
const (
	defaultLobbyTimeout = 15 * time.Minute
	defaultIdleTimeout  = 2 * time.Hour
)

// run is one event's live state, guarded by its own mutex.
type run struct {
	h        *Host
	mu       sync.Mutex
	meta     meta
	ev       *event.Event
	spec     setdata.Spec
	drafters [4]pickbotDrafter
	builders [4]pickbotBuilder
	subs     []subscriber // {session, seat}, subscription order
	lastPack [4]string    // last draft_pack key sent per seat
	halted   string       // persistence/replay failure: read-only, 503 on writes
	blocked  string       // E1-5: why a game could not start
	// E3-5: each live game table's per-slot game tokens — index 0 is event
	// seat a (table slot 0), 1 event seat b. Empty string = not minted.
	tokens map[host.TableID][2]string
	// E1-5: finished matches awaiting their pairing-order log slot, and
	// each series' last published public JSON. Both guarded by mu.
	outcomes   map[[2]int]outcome
	lastSeries map[[3]int]string
	// E3-4: each Bo3 series' live table id, and the parked Series-hook
	// calls per live series table. Both guarded by mu; the waiters map is
	// answered in sorted table-id order, never map order.
	seriesTable map[[2]int]host.TableID
	waiters     map[host.TableID]waiter
	live        atomic.Bool // phase not COMPLETE or CANCELLED; read under Host.mu for the capacity bound
	justDraft   bool        // the draft ended since the last broadcast: one last pod_progress
	// E1-8: when the event was created, and when each human seat last
	// acted successfully. The sweep reads them under mu; replay never does.
	createdAt time.Time
	lastAct   [draft.Seats]time.Time
	// E3-6: each human seat's armed pick timer, guarded by mu. A timer
	// with stop == nil is not armed.
	timers [4]pickTimer
}

// subscriber is one SSE session watching this event, at its seat (-1 for a
// spectator).
type subscriber struct {
	session string
	seat    int
}

// Host runs the draft events of one gorged process.
type Host struct {
	opts   Options
	mu     sync.Mutex
	events map[string]*run
	// tables maps every live event game table to the attempt it is playing,
	// so OnMatchEnd can drop a match it did not start. Guarded by mu.
	tables   map[host.TableID]gameRef
	closed   bool
	wg       sync.WaitGroup
	stop     chan struct{}
	stopOnce sync.Once
}

// New validates Options and resumes every stored event.
func New(o Options) (*Host, error) {
	if o.Registry == nil || o.Store == nil || o.Cards == nil {
		return nil, fmt.Errorf("eventhost: Options.Registry, Store and Cards are required")
	}
	if o.PickTimeout < 0 {
		return nil, fmt.Errorf("eventhost: PickTimeout must not be negative, got %v", o.PickTimeout)
	}
	if o.LobbyTimeout == 0 {
		o.LobbyTimeout = defaultLobbyTimeout
	}
	if o.IdleTimeout == 0 {
		o.IdleTimeout = defaultIdleTimeout
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	dp, err := bots.Normalize(o.DefaultPolicy)
	if err != nil {
		return nil, fmt.Errorf("eventhost: %w", err)
	}
	if bots.ReadsDecklists(dp) {
		return nil, fmt.Errorf("eventhost: default policy %q reads every seat's real decklist (Env); refused", dp)
	}
	policies := make([]string, len(o.Policies))
	for i, p := range o.Policies {
		n, err := bots.Normalize(p)
		if err != nil {
			return nil, fmt.Errorf("eventhost: %w", err)
		}
		policies[i] = n
	}
	o.DefaultPolicy, o.Policies = dp, policies
	h := &Host{opts: o, events: map[string]*run{}, tables: map[host.TableID]gameRef{}, stop: make(chan struct{})}
	lookup := h.lookup()
	ids, err := o.Store.ids()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		r := &run{
			h:           h,
			tokens:      map[host.TableID][2]string{},
			outcomes:    map[[2]int]outcome{},
			lastSeries:  map[[3]int]string{},
			seriesTable: map[[2]int]host.TableID{},
			waiters:     map[host.TableID]waiter{},
		}
		m, merr := o.Store.loadMeta(id)
		if merr != nil {
			r.meta, r.halted = meta{ID: id}, fmt.Sprintf("event %s: meta unreadable: %v", id, merr)
			h.events[id] = r
			continue
		}
		r.meta = m
		// E1-8: a meta written before CreatedAt existed (or a halted run)
		// gets fresh timeout windows from now.
		now := optsNow(o)
		if m.CreatedAt.IsZero() {
			r.createdAt = now
		} else {
			r.createdAt = m.CreatedAt
		}
		r.lastAct = [draft.Seats]time.Time{now, now, now, now}
		entries, lerr := o.Store.readLog(id)
		ev, rerr := event.Replay(entries)
		switch {
		case rerr != nil:
			r.ev, r.halted = event.New(), fmt.Sprintf("event %s: log does not replay: %v", id, rerr)
		case lerr != nil:
			r.ev, r.halted = event.New(), fmt.Sprintf("event %s: log unreadable: %v", id, lerr)
		case len(entries) == 0:
			r.ev = event.New()
		default:
			r.ev = ev
		}
		if r.halted == "" {
			if err := r.prepare(lookup); err != nil {
				r.halted = fmt.Sprintf("event %s: %v", id, err)
			}
		}
		if r.halted == "" {
			r.mu.Lock()
			r.abortInFlight()
			r.advance()
			r.mu.Unlock()
		}
		r.live.Store(eventLive(r.phase()))
		h.events[id] = r
	}
	h.wg.Add(1)
	go h.sweepLoop()
	return h, nil
}

// eventLive reports whether a phase still occupies a live-event slot:
// a cancelled event freed its slot, exactly like a completed one.
func eventLive(p event.Phase) bool {
	return p != event.PhaseComplete && p != event.PhaseCancelled
}

// prepare loads the set spec and builds the run's drafters and builders.
// Every seat gets a builder (a human's is its suggestion; its drafter is
// never driven, advanceDraft only reads bot seats' drafters). The caller
// has filled meta.
func (r *run) prepare(lookup pickbotCardLookup) error {
	spec, err := setdata.Load(r.meta.Set)
	if err != nil {
		return fmt.Errorf("set %q: %w", r.meta.Set, err)
	}
	r.spec = spec
	for s, ms := range r.meta.Seats {
		persona := ms.Drafter
		if persona == "" {
			persona = "rating-colour"
		}
		d, b, err := r.h.newPersona(persona, lookup)
		if err != nil {
			return fmt.Errorf("seat %d: %w", s, err)
		}
		r.drafters[s], r.builders[s] = d, b
	}
	return nil
}

// phase reads the event's phase under run.mu.
func (r *run) phase() event.Phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ev.Phase()
}

// lookup is the card-facts lookup the personas read through.
func (h *Host) lookup() pickbotCardLookup {
	reg := h.opts.Cards
	return func(name string) *cards.Card { c, _ := reg.Lookup(name); return c }
}

// Close stops the host. Afterwards every method answers unavailable. It
// releases the parked series-hook calls first (each answers stop, ending
// its table's goroutine), so wg.Wait cannot wait on them.
func (h *Host) Close() error {
	h.stopOnce.Do(func() { close(h.stop) })
	h.mu.Lock()
	h.closed = true
	h.mu.Unlock()
	for _, r := range h.events {
		r.mu.Lock()
		r.stopPickTimers()
		r.releaseWaiters()
		r.mu.Unlock()
	}
	h.wg.Wait()
	return nil
}

// find returns the run for id, or a not_found error. It takes Host.mu only
// to find the run and never while holding a run.mu.
func (h *Host) find(id string) (*run, *Error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, errUnavailable("eventhost: host is closed")
	}
	r, ok := h.events[id]
	if !ok {
		return nil, errNotFound("eventhost: no event %q", id)
	}
	return r, nil
}

// Events lists every event's public state, in id order.
func (h *Host) Events() []protocol.EventState {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	ids := make([]string, 0, len(h.events))
	for id := range h.events {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return idNum(ids[i]) < idNum(ids[j]) })
	out := make([]protocol.EventState, 0, len(ids))
	for _, id := range ids {
		r, err := h.find(id)
		if err != nil {
			continue
		}
		r.mu.Lock()
		out = append(out, r.stateFor(-1))
		r.mu.Unlock()
	}
	return out
}

// EventState is one event's state as its viewer sees it: a seat token
// yields that seat's own view, an empty token the public spectator view.
func (h *Host) EventState(id, token string) (protocol.EventState, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.EventState{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, true)
	if terr != nil {
		return protocol.EventState{}, terr
	}
	return r.stateFor(seat), nil
}

// CreateEvent validates a create request, stores the event and registers
// its run. Every human seat gets an invite token, returned once in the
// response and stored only as its sha256.
func (h *Host) CreateEvent(req protocol.CreateEventRequest) (protocol.CreateEventResponse, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errUnavailable("eventhost: host is closed")
	}
	if len(req.Seats) != draft.Seats {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errBadRequest("eventhost: an event needs exactly %d seats, got %d", draft.Seats, len(req.Seats))
	}
	if _, err := setdata.Load(req.Set); err != nil {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errBadRequest("eventhost: %v", err)
	}
	if req.BestOf != 0 && req.BestOf != 1 && req.BestOf != 3 {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errBadRequest("eventhost: best_of must be 1 or 3, got %d", req.BestOf)
	}
	// E3-6: the pick-timer mode is a bounded create option, decided here
	// from the seat list, never echoed straight into the meta.
	switch req.PickTimer {
	case "", "off", "standard", "relaxed":
	default:
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errBadRequest("eventhost: pick_timer must be off, standard or relaxed, got %q", req.PickTimer)
	}
	seed, serr := h.mintSeed()
	if serr != nil {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errUnavailable("eventhost: minting seed: %v", serr)
	}
	m := meta{Set: req.Set, Seed: seed, BestOf: req.BestOf, CreatedAt: h.clockNow()}
	humans := 0
	var invites []protocol.EventInvite
	for s, cs := range req.Seats {
		ms := metaSeat{Kind: cs.Kind}
		switch cs.Kind {
		case "human":
			humans++
			if humans > h.maxHumans() {
				h.mu.Unlock()
				return protocol.CreateEventResponse{}, errBadRequest("eventhost: at most %d human seats", h.maxHumans())
			}
			tok, terr := mintToken()
			if terr != nil {
				h.mu.Unlock()
				return protocol.CreateEventResponse{}, errUnavailable("eventhost: minting seat token: %v", terr)
			}
			sum := sha256.Sum256([]byte(tok))
			ms.TokenSHA256 = hex.EncodeToString(sum[:])
			invites = append(invites, protocol.EventInvite{Seat: s, Token: tok})
		case "bot":
			policy, perr := h.botPolicy(cs.Policy)
			if perr != nil {
				h.mu.Unlock()
				return protocol.CreateEventResponse{}, perr
			}
			drafter := cs.Drafter
			if drafter == "" {
				drafter = "rating-colour"
			}
			if _, _, err := h.newPersona(drafter, h.lookup()); err != nil {
				h.mu.Unlock()
				return protocol.CreateEventResponse{}, errBadRequest("eventhost: %v", err)
			}
			ms.Policy, ms.Drafter = policy, drafter
			ms.Name = fmt.Sprintf("Bot %d (%s)", s+1, policy)
		default:
			h.mu.Unlock()
			return protocol.CreateEventResponse{}, errBadRequest("eventhost: seat %d kind %q is neither human nor bot", s, cs.Kind)
		}
		m.Seats[s] = ms
	}
	// E3-6: two or more humans draft against the clock (standard unless
	// the request named a mode); one human or none has no timer at all.
	m.PickTimer = req.PickTimer
	if humans <= 1 {
		m.PickTimer = "off"
	} else if m.PickTimer == "" {
		m.PickTimer = "standard"
	}
	if live := h.liveCount(); live >= h.maxLive() {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errCapacity("eventhost: %d live events is the limit (%d running)", h.maxLive(), live)
	}
	id, err := h.opts.Store.newID(h.liveID())
	if err != nil {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errUnavailable("eventhost: %v", err)
	}
	m.ID = id
	if err := h.opts.Store.saveMeta(m); err != nil {
		h.mu.Unlock()
		return protocol.CreateEventResponse{}, errUnavailable("eventhost: %v", err)
	}
	r := &run{
		h:           h,
		meta:        m,
		ev:          event.New(),
		tokens:      map[host.TableID][2]string{},
		outcomes:    map[[2]int]outcome{},
		lastSeries:  map[[3]int]string{},
		seriesTable: map[[2]int]host.TableID{},
		waiters:     map[host.TableID]waiter{},
		justDraft:   false,
	}
	r.createdAt = m.CreatedAt
	r.lastAct = [draft.Seats]time.Time{m.CreatedAt, m.CreatedAt, m.CreatedAt, m.CreatedAt}
	r.live.Store(true)
	h.events[id] = r
	h.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.prepare(h.lookup()); err != nil {
		r.halted = err.Error()
	} else {
		r.advance()
	}
	r.broadcast()
	resp := protocol.CreateEventResponse{Event: id, Invites: invites}
	for i := range resp.Invites {
		resp.Invites[i].Join = fmt.Sprintf("/e/%s?seat=%d&token=%s", id, resp.Invites[i].Seat, resp.Invites[i].Token)
	}
	resp.State = r.stateFor(-1)
	return resp, nil
}

// liveCount counts the runs whose event has not reached COMPLETE. It reads
// each run's atomic live flag, so Host.mu stays a leaf.
func (h *Host) liveCount() int {
	n := 0
	for _, r := range h.events {
		if r.live.Load() {
			n++
		}
	}
	return n
}

// liveID reports whether an id names a registered run, for Store.newID.
// The caller holds Host.mu.
func (h *Host) liveID() func(string) bool {
	return func(id string) bool {
		_, ok := h.events[id]
		return ok
	}
}

func (h *Host) maxHumans() int {
	if h.opts.MaxHumans > 0 {
		return h.opts.MaxHumans
	}
	return 4
}

func (h *Host) maxLive() int {
	if h.opts.MaxLiveEvents > 0 {
		return h.opts.MaxLiveEvents
	}
	return 4
}

// botPolicy validates and normalises one bot seat's game policy.
func (h *Host) botPolicy(name string) (string, *Error) {
	policy, err := bots.Normalize(name)
	if err != nil {
		return "", errBadRequest("eventhost: %v", err)
	}
	if bots.ReadsDecklists(policy) {
		return "", errBadRequest("eventhost: policy %q reads every seat's real decklist (Env); draft events refuse Env policies", policy)
	}
	if len(h.opts.Policies) > 0 && !slices.Contains(h.opts.Policies, policy) {
		return "", errBadRequest("eventhost: policy %q is not offered (offered: %s)", policy, strings.Join(h.opts.Policies, ", "))
	}
	e, ok := bots.Lookup(policy)
	if !ok {
		return "", errBadRequest("eventhost: policy %q is not registered", policy)
	}
	if !slices.Contains(e.Formats, "constructed") {
		return "", errBadRequest("eventhost: policy %q does not play constructed", policy)
	}
	if e.MaxSeats != 0 && e.MaxSeats < 2 {
		return "", errBadRequest("eventhost: policy %q seats at most %d", policy, e.MaxSeats)
	}
	return policy, nil
}

// mintToken mints one seat invite token: 16 crypto/rand bytes, hex.
func mintToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// mintSeed is the default event-seed source: 8 crypto/rand bytes.
func mintSeed() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

// mintSeed mints one event seed: the injected Options.Seed when set, else
// crypto/rand.
func (h *Host) mintSeed() (uint64, error) {
	if h.opts.Seed != nil {
		return h.opts.Seed(), nil
	}
	return mintSeed()
}

// markAct records a human seat's successful action for the idle sweep.
// The host's own auto-actions never call it: an idle seat stays idle
// until its human acts. The caller holds run.mu.
func (r *run) markAct(seat int, now time.Time) {
	if seat >= 0 && seat < draft.Seats && r.meta.Seats[seat].Kind == "human" {
		r.lastAct[seat] = now
	}
}

// JoinEvent records a human seat's display name. In LOBBY it joins; later
// it only confirms the same name (idempotent) or refuses another.
func (h *Host) JoinEvent(id, token string, req protocol.JoinEventRequest) (protocol.EventState, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.EventState{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.EventState{}, terr
	}
	if r.meta.Seats[seat].Kind != "human" {
		return protocol.EventState{}, errBadRequest("eventhost: seat %d is not a human seat", seat)
	}
	if r.halted != "" {
		return protocol.EventState{}, errUnavailable("%s", r.halted)
	}
	name, nerr := cleanName(req.Name)
	if nerr != nil {
		return protocol.EventState{}, nerr
	}
	if r.ev.Phase() == event.PhaseLobby {
		r.meta.Seats[seat].Name = name
		if err := r.h.opts.Store.saveMeta(r.meta); err != nil {
			r.halted = fmt.Sprintf("event %s: persistence failed: %v", r.meta.ID, err)
			return protocol.EventState{}, errUnavailable("%s", r.halted)
		}
		r.advance()
		r.markAct(seat, r.h.clockNow())
		r.broadcast()
	} else if r.ev.Phase() == event.PhaseCancelled {
		return protocol.EventState{}, errConflict("eventhost: event %s was cancelled", r.meta.ID)
	} else if r.meta.Seats[seat].Name != name {
		return protocol.EventState{}, errConflict("eventhost: seat %d already joined as %q", seat, r.meta.Seats[seat].Name)
	}
	return r.stateFor(seat), nil
}

// cleanName validates a display name: 1–32 runes after trimming, no
// control characters.
func cleanName(name string) (string, *Error) {
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) == 0 || len(runes) > 32 {
		return "", errBadRequest("eventhost: name must be 1-32 characters")
	}
	for _, c := range runes {
		if c == 0x7F || (c < 0x20 && c != 0x09) || (c >= 0x80 && c < 0xA0) {
			return "", errBadRequest("eventhost: name must not contain control characters")
		}
	}
	return name, nil
}

// DraftPack is the pack at the seat's queue head and its pool so far.
func (h *Host) DraftPack(id, token string) (protocol.DraftPack, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.DraftPack{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.DraftPack{}, terr
	}
	return r.packFor(seat), nil
}

// Pick takes one card from the pack at the seat's queue head, applies the
// bots' answers that follow it, and returns the seat's next pack.
func (h *Host) Pick(id, token string, req protocol.PickRequest) (protocol.DraftPack, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.DraftPack{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.DraftPack{}, terr
	}
	if r.halted != "" {
		return protocol.DraftPack{}, errUnavailable("%s", r.halted)
	}
	if r.ev.Phase() != event.PhaseDraft {
		return protocol.DraftPack{}, errConflict("eventhost: the draft is not running (phase %s)", r.ev.Phase())
	}
	v, ok := r.ev.Next(seat)
	if !ok {
		return protocol.DraftPack{}, errNotYourTurn("eventhost: seat %d has no pack to pick from", seat)
	}
	if req.Round != v.Round || req.PickNo != v.PickNo {
		return protocol.DraftPack{}, errStalePick("eventhost: seat %d's pack is round %d pick %d, not round %d pick %d",
			seat, v.Round, v.PickNo, req.Round, req.PickNo)
	}
	inPack := false
	for _, c := range v.Pack {
		if c.ID == req.ID {
			inPack = true
			break
		}
	}
	if !inPack {
		return protocol.DraftPack{}, errNotInPack("eventhost: card %q is not in seat %d's pack", req.ID, seat)
	}
	if err := r.apply(event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: seat, ID: req.ID}); err != nil {
		return protocol.DraftPack{}, err
	}
	r.advance()
	r.markAct(seat, r.h.clockNow())
	r.broadcast()
	return r.packFor(seat), nil
}

// SubscribeEvent binds one open SSE session to this event at the token's
// seat (or as a spectator with an empty token), replacing any earlier
// binding of the same session, and pushes the session its first frames at
// once.
func (h *Host) SubscribeEvent(id, token, session string) error {
	r, ferr := h.find(id)
	if ferr != nil {
		return ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := h.opts.Registry.Session(session)
	if !ok {
		return errNotFound("eventhost: no session %q", session)
	}
	seat, terr := r.seatFor(token, true)
	if terr != nil {
		return terr
	}
	replaced := false
	for i := range r.subs {
		if r.subs[i].session == session {
			r.subs[i].seat = seat
			replaced = true
			break
		}
	}
	if !replaced {
		r.subs = append(r.subs, subscriber{session: session, seat: seat})
	}
	r.publish(s, protocol.TEventState, r.stateFor(seat))
	if r.ev.Phase() == event.PhaseDraft {
		// E1-8: a mid-draft subscriber learns the pod's progress before
		// its pack, so a fresh UI renders the table first.
		r.publish(s, protocol.TPodProgress, r.progress())
		if seat >= 0 {
			r.publish(s, protocol.TDraftPack, r.packFor(seat))
		}
	}
	return nil
}

// seatFor resolves a request token to a seat. An empty token is a
// spectator (seat -1) when spectatorOK, and unauthorised on seat-scoped
// calls. A non-empty token is matched against each seat's stored sha256
// with a constant-time compare.
func (r *run) seatFor(token string, spectatorOK bool) (int, *Error) {
	if token == "" {
		if spectatorOK {
			return -1, nil
		}
		return -1, errUnauthorized("eventhost: a seat token is required")
	}
	sum := sha256.Sum256([]byte(token))
	cand := hex.EncodeToString(sum[:])
	for s := range r.meta.Seats {
		stored := r.meta.Seats[s].TokenSHA256
		if stored == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(cand), []byte(stored)) == 1 {
			return s, nil
		}
	}
	return -1, errUnauthorized("eventhost: unknown seat token")
}

// apply is the run's one mutation path: validate and apply the entry in
// memory, then append it to the log. An append failure halts the run: the
// event goes read-only, every later write answers unavailable.
func (r *run) apply(en event.Entry) error {
	if r.halted != "" {
		return errUnavailable("%s", r.halted)
	}
	if err := r.ev.Apply(en); err != nil {
		return errUnavailable("eventhost: %s entry refused: %v", en.Kind, err)
	}
	if err := r.h.opts.Store.appendLog(r.meta.ID, en); err != nil {
		r.halted = fmt.Sprintf("event %s: persistence failed: %v", r.meta.ID, err)
		return errUnavailable("%s", r.halted)
	}
	r.live.Store(eventLive(r.ev.Phase()))
	return nil
}

// publish builds and pushes one frame; a marshal failure of our own struct
// is dropped (it cannot happen), a full session is dropped by Publish.
func (r *run) publish(s *host.Session, t protocol.FrameType, body any) {
	f, err := protocol.NewFrame(t, "", 0, 0, body)
	if err != nil {
		return
	}
	r.h.opts.Registry.Publish(s, f)
}
