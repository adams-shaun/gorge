package protocol

import "encoding/json"

// Draft-event frame kinds (eventhost). Their frames
// carry no table, match or seq: NewFrame(t, "", 0, 0, body).
const (
	TEventState   FrameType = "event_state"
	TDraftPack    FrameType = "draft_pack"
	TPodProgress  FrameType = "pod_progress"
	TSeriesUpdate FrameType = "series_update"
)

// DraftCard is one card as a draft client sees it: the server-minted
// instance ID to answer a pick with, plus the printed facts it renders.
// Rarity is one letter: C U R M (mythic) L (special) S.
type DraftCard struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Set       string `json:"set"`
	Number    string `json:"number"`
	Rarity    string `json:"rarity"`              // one letter: C U R M L S
	ManaCost  string `json:"mana_cost,omitempty"` // printed cost; "" when the card has none
	ManaValue int    `json:"mana_value,omitempty"`
	Colors    string `json:"colors,omitempty"`    // subset of "WUBRG", in WUBRG order
	TypeLine  string `json:"type_line,omitempty"` // face types, space-joined
	Promo     bool   `json:"promo,omitempty"`     // the seat's prerelease promo (sealed pools only)
	// Identity, Commander and Partner are set only in a Commander-draft
	// event: the card's colour identity (CR 903.4, a subset of "WUBRG"),
	// whether it can be a commander on its own (CR 903.3), and whether it
	// can be half of a Commander-draft pair (plain partner, or a
	// partnerless legend of at most one colour: the draft partner rule).
	Identity  string `json:"identity,omitempty"`
	Commander bool   `json:"commander,omitempty"`
	Partner   bool   `json:"partner,omitempty"`
}

// EventSeat is one entrant of a draft event. Kind is "human" or "bot";
// Policy and Drafter name a bot seat's game policy and pickbot persona.
// Picks is the public count of cards drafted so far, never the cards.
type EventSeat struct {
	Seat    int    `json:"seat"`
	Kind    string `json:"kind"`              // "human" | "bot"
	Name    string `json:"name"`              // "" until a human joins
	Policy  string `json:"policy,omitempty"`  // bot: game policy (bots registry name)
	Drafter string `json:"drafter,omitempty"` // bot: pickbot persona name
	Joined  bool   `json:"joined"`
	Picks   int    `json:"picks"` // cards drafted so far (a public count)
	Ready   bool   `json:"ready"`
}

// EventSeries is one pairing of an event round. State is "pending", "live",
// "between" or "ended" — "between" names the sideboarding step between two
// games of a series, and a rebuild event's window before game 1 of a round 2
// or 3 series (MTR 7.3), where Game is 1 and Table is empty (no game of the
// series has a table yet). Table names the live (or last) game's host
// table, so a spectator can watch at /t/{table}; Attempt is the game
// attempt in flight. Game is the live game, or the next one while between.
// Games is the winner seat per played game (-1 for a draw); Chooser names
// the event seat choosing play/draw for Game; Waiting lists the seats
// still to submit their deck while between. Winner is nil while the series
// is not ended and for a drawn series. Join appears only in the viewer's
// own state: the seat-tokenised spectate URL for that seat's side of the
// live game.
type EventSeries struct {
	Round   int    `json:"round"`
	A       int    `json:"a"`
	B       int    `json:"b"`
	BestOf  int    `json:"best_of"`
	State   string `json:"state"`             // "pending" | "live" | "between" | "ended"
	Table   string `json:"table,omitempty"`   // the live (or last) game's table: spectate at /t/{table}
	Attempt int    `json:"attempt,omitempty"` // the live game's attempt number
	Game    int    `json:"game,omitempty"`    // the live game, or the next one while between
	Games   []int  `json:"games,omitempty"`   // winner seat per played game (-1 draw)
	Chooser *int   `json:"chooser,omitempty"` // the event seat choosing play/draw for Game
	Waiting []int  `json:"waiting,omitempty"` // seats still to submit their deck while between
	Score   [2]int `json:"score"`
	Winner  *int   `json:"winner"`         // nil while not ended, and for a drawn series
	Join    string `json:"join,omitempty"` // only in the viewer's own state: /t/{table}?seat=N&token=T
}

// EventStanding is one row of the event standings: MTR Appendix C points
// and match/game win percentages.
// EventPodGame is one Commander-draft free-for-all game: every seat at one
// table. State is "pending", "live" or "ended"; Winner is nil while not
// ended and for a drawn game; Join only in the viewer's own state.
type EventPodGame struct {
	Round  int    `json:"round"`
	State  string `json:"state"`
	Table  string `json:"table,omitempty"`
	Winner *int   `json:"winner"`
	Join   string `json:"join,omitempty"`
}

type EventStanding struct {
	Seat   int     `json:"seat"`
	Points int     `json:"points"`
	MW     float64 `json:"mw"`
	OMW    float64 `json:"omw"`
	GW     float64 `json:"gw"`
	OGW    float64 `json:"ogw"`
}

// EventState is the full public draft-event state, the event_state body.
// You is the viewer's seat when a valid seat token was presented, nil for a
// spectator. Series lists every pairing of rounds 1..3 in round then
// pairing order. Error carries a human-readable problem with the event's
// progress (a crashed game attempt that could not be rescheduled), never a
// request-validation message.
type EventState struct {
	Event     string          `json:"event"`
	Set       string          `json:"set"`
	Phase     string          `json:"phase"` // LOBBY DRAFT BUILD ROUND1 ROUND2 ROUND3 COMPLETE
	Round     int             `json:"round"`
	BestOf    int             `json:"best_of"` // the event's series format (1 or 3)
	PickTimer string          `json:"pick_timer,omitempty"`
	Format    string          `json:"format,omitempty"`  // "sealed" | "prerelease" | "commander"; omitted for a draft
	Rebuild   bool            `json:"rebuild,omitempty"` // MTR 7.3 continuous construction; true for sealed and prerelease by default
	Seats     []EventSeat     `json:"seats"`
	You       *int            `json:"you"`    // the viewer's seat when a valid seat token was presented
	Series    []EventSeries   `json:"series"` // every pairing of rounds 1..3, round then pairing order
	Standings []EventStanding `json:"standings"`
	Error     string          `json:"error,omitempty"`
	// PicksPerPass is how many cards a seat takes from each pack before
	// passing it; omitted (1) outside a Commander draft. DeckSize is the
	// deck's minimum size when it is not limited's 40 (a Commander draft's
	// 60). Pod is a Commander draft's free-for-all games, one per event
	// round; Series stays empty in such an event.
	PicksPerPass int            `json:"picks_per_pass,omitempty"`
	DeckSize     int            `json:"deck_size,omitempty"`
	Pod          []EventPodGame `json:"pod,omitempty"`
}

// DraftPack is the draft_pack body: the pack waiting at a seat's queue head
// and the pool it has drafted so far. Waiting means no pack is queued for
// this seat right now, so Pack and Pool carry their last-known (or empty)
// contents.
type DraftPack struct {
	Event       string `json:"event"`
	Seat        int    `json:"seat"`
	Waiting     bool   `json:"waiting"` // no pack queued for this seat right now
	Round       int    `json:"round"`
	PickNo      int    `json:"pick_no"`
	TimeLimitMS int64  `json:"time_limit_ms,omitempty"` // 0 = no pick timer
	RemainingMS int64  `json:"remaining_ms,omitempty"`
	// PicksPerPass and PassPick appear only when a seat takes more than
	// one card per pass: PassPick is which of those picks this is (1-based),
	// so a client can show "second pick from this pack".
	PicksPerPass int         `json:"picks_per_pass,omitempty"`
	PassPick     int         `json:"pass_pick,omitempty"`
	Pack         []DraftCard `json:"pack"`
	Pool         []DraftCard `json:"pool"`
}

// PodProgress is the pod_progress body: how far every seat is through the
// current draft round, as public counts.
// DraftAdvice is GET /api/events/{id}/advice: the seat's own drafter's
// suggestion for the pack at its queue head, from what the seat itself sees
// (its pack and its pool, nothing else). Picks are the best cards in score
// order, as many as the seat still takes from this pack this pass; Plan is
// the colour identity of the commander plan the drafter reads from the pool
// ("" before the pool holds a commander). Waiting when no pack is queued.
type DraftAdvice struct {
	Event   string            `json:"event"`
	Seat    int               `json:"seat"`
	Waiting bool              `json:"waiting"`
	Round   int               `json:"round"`
	PickNo  int               `json:"pick_no"`
	Plan    string            `json:"plan,omitempty"`
	Picks   []DraftAdvicePick `json:"picks"`
}

// DraftAdvicePick is one suggested card: its score on the drafter's points
// scale, the score's terms (value, commander and identity) and a short
// reason spelling them.
type DraftAdvicePick struct {
	ID        string  `json:"id"`
	Score     float64 `json:"score"`
	Value     float64 `json:"value"`
	Commander float64 `json:"commander,omitempty"`
	Identity  float64 `json:"identity,omitempty"`
	Reason    string  `json:"reason"`
}

type PodProgress struct {
	Event  string `json:"event"`
	Round  int    `json:"round"`
	Picks  []int  `json:"picks"`  // per seat
	Queued []int  `json:"queued"` // per seat
}

// SeriesUpdate is the series_update body: one series' row changed.
type SeriesUpdate struct {
	Event  string      `json:"event"`
	Series EventSeries `json:"series"`
}

// CreateEventSeat is one seat of a CreateEventRequest. Kind is "human" or
// "bot"; Policy names the bot's game policy ("" = server default) and
// Drafter the pickbot persona ("" = "rating-colour").
type CreateEventSeat struct {
	Kind    string `json:"kind"`              // "human" | "bot"
	Policy  string `json:"policy,omitempty"`  // bot game policy; "" = server default
	Drafter string `json:"drafter,omitempty"` // pickbot name; "" = "rating-colour"
}

// CreateEventRequest is the POST /api/events body: the drafted set, the
// series format and the four seats. BestOf is 1 or 3 (0 = 1). PickTimer is
// "", "off", "standard" or "relaxed". The seed is always minted by the
// server; no client field may choose it. Format is the supply: a booster draft
// (default), sealed (six Play Boosters per seat) or prerelease (sealed plus one
// rare/mythic promo per seat, best of 3 by default). Rebuild turns on MTR 7.3
// continuous construction: a rebuild window before game 1 of every series from
// round 2. When nil, the format decides: true for sealed and prerelease, false
// for draft.
type CreateEventRequest struct {
	Set       string            `json:"set"`
	Format    string            `json:"format,omitempty"`     // "" or "draft", "sealed", "prerelease", "commander"
	BestOf    int               `json:"best_of,omitempty"`    // 0 = 1; 1 or 3
	PickTimer string            `json:"pick_timer,omitempty"` // "", "off", "standard", "relaxed"
	Rebuild   *bool             `json:"rebuild,omitempty"`    // nil = the format's default: true for sealed and prerelease, false for draft
	Seats     []CreateEventSeat `json:"seats"`                // exactly 4
}

// SideboardRequest is the POST /api/events/{id}/sideboard body: the seat's
// deck for game Game of its current series. Keep re-submits the deck it
// played last game (in a rebuild event's round-2-or-later window, Game 1, the
// deck the seat played last); otherwise Deck is the new list (pool CardRef ids
// plus basics), saved as a new revision. Exactly one of Keep and Deck is set.
// Game is compared with the server's next game; a stale one is a 409.
type SideboardRequest struct {
	Game int          `json:"game"`
	Keep bool         `json:"keep,omitempty"`
	Deck *DeckRequest `json:"deck,omitempty"`
}

// EventInvite is one seat's join token in a CreateEventResponse. Join is the
// tokenised event URL /e/{event}?seat=N&token=T.
type EventInvite struct {
	Seat  int    `json:"seat"`
	Token string `json:"token"`
	Join  string `json:"join"` // /e/{event}?seat=N&token=T
}

// CreateEventResponse is the POST /api/events reply: the event id, one
// invite per seat and the event's initial state.
type CreateEventResponse struct {
	Event   string        `json:"event"`
	Invites []EventInvite `json:"invites"`
	State   EventState    `json:"state"`
}

// EventSetInfo is one draftable set as GET /api/events/sets reports it.
// Ratings is the rating source the pickbot uses for it
// (pickbot.RatingSource): "17lands" for a 17Lands rating file, "simulated"
// for a simulated card-strength file, "heuristic" when none exists.
type EventSetInfo struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	PackSize int    `json:"pack_size"`
	Ratings  string `json:"ratings"` // "17lands" | "simulated" | "heuristic"
	// Commander is true when this server hosts a Commander draft of the set
	// (create's format "commander"); the lobby offers that mode only then.
	Commander bool `json:"commander,omitempty"`
}

// EventPersonas is the GET /api/events/personas reply: the registered
// pickbot persona names, offered to a create body's Drafter fields.
type EventPersonas struct {
	Drafters []string `json:"drafters"`
	Builders []string `json:"builders"`
}

// JoinEventRequest is the POST /api/events/{id}/join body: the joining
// human's display name.
type JoinEventRequest struct {
	Name string `json:"name"`
}

// PickRequest is the POST /api/events/{id}/pick body: which card instance
// the seat takes from the pack at its queue head. Round and PickNo are
// compared against server state; a stale answer is a 409.
type PickRequest struct {
	Round  int    `json:"round"`
	PickNo int    `json:"pick_no"`
	ID     string `json:"id"`
}

// DeckRequest is a seat's deck list: maindeck CardRef IDs from its own pool
// plus basic-land counts (W U B R G). It is also DeckPool's saved-deck and
// suggestion shape.
type DeckRequest struct {
	Main   []string `json:"main"`   // CardRef ids from the seat's pool
	Basics [5]int   `json:"basics"` // W U B R G
	// Commanders names a Commander-draft deck's one or two commanders, as
	// ids Main also carries. Omitted in every other event.
	Commanders []string `json:"commanders,omitempty"`
}

// DeckCheck is the deck-check reply: whether the submitted (or suggested)
// deck is legal, its total card count, the auto-land counts the builder
// would add to this main, and the revision a successful save produced.
type DeckCheck struct {
	Valid     bool   `json:"valid"`
	Total     int    `json:"total"`
	Error     string `json:"error,omitempty"`
	AutoLands [5]int `json:"auto_lands"`    // basics AutoLands would add to this main
	Rev       int    `json:"rev,omitempty"` // set by a successful save
	// Identity is a Commander-draft deck's commanders' colour identity
	// (a subset of "WUBRG"; "" for colourless or no commander), and Size
	// the deck's minimum size; both omitted outside a Commander draft.
	Identity string `json:"identity,omitempty"`
	Size     int    `json:"size,omitempty"`
}

// EventPool is the GET /api/events/{id}/pool body: the seat's drafted cards,
// its latest saved deck revision (nil before the first save), whether the
// seat has readied, and the bot builder's suggested deck for this pool.
// Playing is, in a round, the deck the seat plays or last played in its
// series, with PlayingRev naming its revision.
type EventPool struct {
	Event      string       `json:"event"`
	Seat       int          `json:"seat"`
	Pool       []DraftCard  `json:"pool"`
	Deck       *DeckRequest `json:"deck"` // latest saved revision, nil before the first save
	Rev        int          `json:"rev"`
	Ready      bool         `json:"ready"`
	Playing    *DeckRequest `json:"playing,omitempty"` // the deck the seat plays or last played in its series
	PlayingRev int          `json:"playing_rev,omitempty"`
	Suggestion DeckRequest  `json:"suggestion"` // the bot builder's deck for this pool
}

// ReadyRequest is the POST /api/events/{id}/ready body: the deck revision
// the seat is readying; a stale revision is a 409.
type ReadyRequest struct {
	Rev int `json:"rev"`
}

// EventLog is the GET /api/events/{id}/log body: the event's log entries.
// Entries stay redacted until the event is COMPLETE (see event.LogFor).
type EventLog struct {
	Event    string            `json:"event"`
	Complete bool              `json:"complete"`
	Entries  []json.RawMessage `json:"entries"` // event.Entry JSON, redacted until COMPLETE
}

// EventSubscribe is the POST /api/events/{id}/subscribe body: the session id
// the client echoes, opening its event SSE stream.
type EventSubscribe struct {
	Session string `json:"session"`
}
