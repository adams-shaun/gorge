package state

import "github.com/adams-shaun/gorge/cards"

type Player struct {
	ID          PlayerID
	Name        string
	PlayerName  string
	Life        int32
	Lost        bool
	LandsPlayed int32
	Pool        Mana
	// RestrictedMana retains the spend restriction on mana produced by a
	// RestrictValid$ mana ability. It is cleared with the pool at step/phase
	// cleanup and is reconstructed from ManaAdd events.
	RestrictedMana []ManaRestriction

	// Counters records player counters (currently poison, used by Ward costs).
	Counters []Counter

	// Snow parallels Pool slot for slot: Snow[i] counts how many of the
	// Pool[i] mana units were produced by a Snow permanent (CR 107.4h — a
	// snow unit can pay a {S} pip as well as anything else one mana pays).
	// It is written only by the ManaAdd event's "S<colour>" Counter form and
	// cleared with the pool by ManaClear, so Snow[i] <= Pool[i] always holds
	// and a replay derives both identically.
	Snow Mana

	// Commanders lists this seat's commanders, in Config order, sized at
	// genesis and never grown. CmdCasts runs parallel to it: entry k counts
	// how many times Commanders[k] has been cast from the command zone.
	// CmdDamage records combat damage this seat has taken (a per-match
	// cumulative total), indexed by the match-wide dense commander index
	// assigned at genesis -- see rules.New. All three are populated by the
	// Commander genesis and written by other tasks (the commander tax, CR
	// 903.9 and commander damage hand off through them); Clone deep-copies
	// them so a cloned game's bookkeeping never aliases the live one's.
	Commanders []ObjID
	CmdCasts   []int32
	CmdDamage  []int32

	// Speed is this seat's speed (CR 702.163, "Start your engines!"): it
	// starts at 0 (or 1 the first time an engine grants speed), rises by one
	// once on each of this seat's own turns when an opponent loses life,
	// caps at 4 (max speed), and never resets. Written only by events.Apply's
	// SpeedChange case, so a log-only reconstruction rebuilds it exactly.
	Speed int32
}

// ExtraTurn is one pending CR 500.7 turn. It is deliberately a queue entry,
// rather than a per-player flag: several grants can be pending in LIFO order
// and each grant can carry a different rider.
type ExtraTurn struct {
	Player    PlayerID
	SkipUntap bool
}

// Counter returns this player's count of kind.
func (p *Player) Counter(kind string) int32 {
	for _, c := range p.Counters {
		if c.Kind == kind {
			return c.N
		}
	}
	return 0
}

// AddCounter changes one player-counter kind, clamping at zero.
func (p *Player) AddCounter(kind string, n int32) {
	for i := range p.Counters {
		if p.Counters[i].Kind == kind {
			p.Counters[i].N += n
			if p.Counters[i].N < 0 {
				p.Counters[i].N = 0
			}
			return
		}
	}
	if n > 0 {
		p.Counters = append(p.Counters, Counter{Kind: kind, N: n})
	}
}

// Game is the complete authoritative state. Everything a client sees is a
// projection of this. Only the events package may mutate it.
type Game struct {
	Players []Player
	// Objs is a dense arena: Objs[i] has ID i+1, so ObjID 0 is "no object".
	Objs     []Object
	Stack    []ObjID
	Turn     int32
	Active   PlayerID
	Priority PlayerID
	// StartingPlayer is the seat that takes the first turn. HasStartingPlayer
	// keeps seat zero distinct from a game whose opening determination has not
	// completed (for example terminal genesis with no survivors). It is folded
	// only by events.StartingPlayerChange, so replay, Clone and snapshots retain
	// opening-hand effects that replace the toss result.
	StartingPlayer    PlayerID
	HasStartingPlayer bool
	Step              Step
	Passes            int32
	Over              bool
	Winner            PlayerID
	// Draw marks a game that ended with no surviving seats (CR 104.4a).
	// Winner's zero value is PlayerID(0), a real seat, so Over alone cannot
	// distinguish "seat 0 won" from "nobody did" -- Draw is what does.
	Draw bool
	// ExtraTurns counts, per seat, how many EXTRA turns (CR 500.7) that seat
	// still takes after the seat's current one, before turn order resumes
	// normally. Added by an api:AddTurn effect's ExtraTurn event (+Amount),
	// consumed by the turn structure (rules advanceStep emits Amount -1 and
	// repeats the same seat) -- so a log-only reconstruction folds the same
	// grants and consumptions to the same totals.
	ExtraTurns map[PlayerID]int
	// ExtraTurnQueue is the ORDERED pending extra turns, in creation order:
	// one entry per un-consumed ExtraTurn grant (+Amount event), appended on
	// the grant and removed (the seat's LAST entry) on the -1 consumption.
	// CR 500.7 takes multiple extra turns MOST RECENTLY CREATED FIRST, so the
	// turn structure consumes the queue from its end. Each entry retains the
	// grant's turn-specific rider (currently SkipUntap), which a per-seat count
	// cannot express. Every mutation rides the same events. Empty when no extra
	// turn is pending.
	ExtraTurnQueue []ExtraTurn
	// Monarch is the current monarch when HasMonarch is true. The presence bit
	// keeps seat zero distinct from no monarch.
	Monarch    PlayerID
	HasMonarch bool
	// NextID hands out object ids one at a time, starting at 1 (see NewGame)
	// and incrementing by exactly one per AddObject call below -- it can
	// never reach playerRefBit (1<<31, ids.go): a single match would need
	// over two billion objects created before that collision, far beyond
	// anything this engine's own bounds (maxTriggerFires, turn/priority
	// limits, ...) let happen. That gap is what makes PlayerRef/ObjID.PlayerRef
	// a safe way to smuggle a PlayerID through an []ObjID slot. Note that
	// int(id) of such a sentinel is negative on 32-bit builds (1<<31
	// overflows int there), so g.Obj must compare id as an ObjID/uint32 --
	// the id&playerRefBit check in its bounds test below -- never via
	// int(id) alone.
	NextID ObjID
	// Clock is a monotonic timestamp source for continuous-effect ordering.
	Clock uint32

	// Tokens is the token definitions this match may create, keyed by
	// Forge script stem; set at genesis, never mutated, so Clone shares it.
	Tokens map[string]*cards.Card

	// Delayed holds delayed-trigger registrations (CR 603.7: Mode$ Phase, or
	// the event-matched shape a DelayedTrigger.EventMode names) that have not
	// yet fired. It is game state -- a delayed trigger is
	// registered during one resolution and fires later, in general a
	// different turn, so the registration has to survive the event log to
	// survive replay, and only events.Apply may write it. Each entry records
	// the phase it fires in, the source object whose SVar table carries the
	// Execute$ ability, the controller, and the Remembered captured at
	// registration. Firing is one-shot: the DelayedPush event removes the
	// entry, so a delayed trigger fires exactly once. DelayedNext hands out
	// the registration IDs monotonically so a firing can remove precisely the
	// registration it belongs to (a source may register several).
	Delayed     []DelayedTrigger
	DelayedNext uint32

	// zones is indexed by zoneIndex(z, p); the stack lives in Stack instead.
	zones [][]ObjID

	// Entered holds, in move order, every zone entry made this turn -- the
	// per-add list Forge keeps per zone (CardUtil.getThisTurnEntered reads
	// it). It is derived exclusively inside events.Apply (each Move appends
	// one entry; the TurnChange reset clears it) so a replay folds the
	// identical list. Objects that ceased (ZCeased) stay listed: Forge's
	// list keeps the card too, and validity is evaluated against the object
	// wherever it now lives.
	Entered []ZoneEntry
}

// ZoneEntry is one zone entry made this turn: the object that moved, the
// zone it entered (To) and the zone it actually came from (From -- the real
// pre-move zone, the same value events.Move recorded on the object's
// EnteredFrom). A card that entered a zone twice (bounced and replayed) is
// listed once per entry, exactly like Forge's per-zone
// getCardsAddedThisTurn lists, which the Count$ThisTurnEntered_* heads and
// the ThisTurnEntered* filter predicates read.
type ZoneEntry struct {
	Obj  ObjID
	To   Zone
	From Zone
}

// DelayedTrigger is one registered delayed triggered ability awaiting its
// phase -- or, for an event-matched registration (EventMode set), awaiting its
// triggering event. It is reconstructed from the event log by events.Apply's
// DelayedRegister case, so a replay that folds the logged registrations
// arrives at the same set. Firing (events.Apply's DelayedPush case) removes
// the entry, which is what keeps a delayed trigger one-shot.
type DelayedTrigger struct {
	ID         uint32
	Phase      Step
	Source     ObjID
	Controller PlayerID
	Execute    string // the SVar name of the ability to run when it fires
	Remembered []Target
	// MinTurn is the earliest game turn the trigger may fire in (zero = no
	// bound). rules' delayed-trigger scan skips an entry whose MinTurn is
	// still ahead of the current turn, which is how an extra-turn grant's
	// end-step trigger (Final Fortune) skips the granting turn's own end
	// step and fires in the granted turn instead. Folded from the
	// registering event's Amount, so a replay rebuilds it.
	MinTurn int32
	// EventMode and Trigger extend the registration to the non-phase
	// (event-matched) shape: EventMode is the trigger Mode$ the registration
	// fires on ("SpellCast" -- the first spell cast whose event satisfies the
	// body's validity clauses) and Trigger is the SVar name on the source's
	// face holding that trigger body, re-parsed at fire time so ValidCard$/
	// ValidActivatingPlayer$ are evaluated against the actual cast. Both are
	// empty for a Mode$ Phase registration, so every already-registered shape
	// is unchanged; the pair travels inside the DelayedRegister event's Text
	// field ("<Mode>:<Trigger>") because the event gains no fields.
	EventMode string
	Trigger   string
	// ValidPlayer is the registering DelayedTrigger SA's ValidPlayer$ value
	// when it has one (Necropotence's "You": "put that card into your hand
	// at the beginning of YOUR next end step"). The rules-side delayed
	// trigger scan gates the fire on it -- the phase occurrence must satisfy
	// the spec against the registration's controller -- and a phase the gate
	// fails leaves the one-shot registration pending for the first later
	// occurrence that matches. Empty keeps the ungated fire every earlier
	// registration had. It rides in the DelayedRegister event's Text
	// ("<Phase>|VP=<value>") because the event gains no field (Ruling T20-a's
	// field-reuse precedent); the SpellCast decode never collides because a
	// Phase$ value contains no "|".
	ValidPlayer string
	// MaxTurn is the LATEST game turn the trigger may fire in (zero = no
	// bound) — the mirror of MinTurn for a ThisTurn$ True registration
	// (Mistrise Village's "the next spell you cast this turn can't be
	// countered"): a registration whose turn has passed without firing is
	// skipped by rules' delayed-trigger scans forever after, never removed
	// (removal would need an event of its own to keep a replay folding the
	// same set, so the expired entry just stays inert). It rides the
	// registering event's Text ("|TT=<turn>") because the event gains no
	// field (Ruling T20-a's field-reuse precedent).
	MaxTurn int32
	// SourceIncarnation is captured for keyword promises whose effect applies
	// to that exact permanent (dash/warp). Ordinary CR 603.7 delayed triggers,
	// including Encore's group cleanup, intentionally leave TrackSource false:
	// they exist independently of their source after registration.
	SourceIncarnation uint32
	TrackSource       bool
}

const startingLife = 20

// NewGame builds a game with the default starting life total (20). It keeps
// the every-existing-caller spelling: NewGameLife is the life-taking variant,
// so callers that never set a life total observe exactly what they always
// did.
func NewGame(names []string) *Game { return NewGameLife(names, startingLife) }

// NewGameLife is NewGame with an explicit starting life total (Config.
// StartingLife's 0-means-20 convention is resolved by the caller).
func NewGameLife(names []string, life int32, objectCapacity ...int) *Game {
	capacity := 0
	if len(objectCapacity) > 0 && objectCapacity[0] > 0 {
		capacity = objectCapacity[0]
	}
	g := &Game{NextID: 1, Objs: make([]Object, 0, capacity), zones: make([][]ObjID, numZones*len(names))}
	for i, n := range names {
		g.Players = append(g.Players, Player{ID: PlayerID(i), Name: n, Life: life})
	}
	return g
}

func (g *Game) zoneIndex(z Zone, p PlayerID) int { return int(p)*numZones + int(z) }

func (g *Game) Zone(z Zone, p PlayerID) []ObjID {
	if z == ZCeased {
		return nil
	}
	if z == ZStack {
		return g.Stack
	}
	return g.zones[g.zoneIndex(z, p)]
}

func (g *Game) SetZone(z Zone, p PlayerID, ids []ObjID) {
	if z == ZCeased {
		return
	}
	if z == ZStack {
		g.Stack = ids
		return
	}
	g.zones[g.zoneIndex(z, p)] = ids
}

// Obj returns the object with this ID, or nil for ObjID 0, a PlayerRef
// sentinel (ids.go), and out-of-range IDs. The sentinel check must compare
// id as an ObjID/uint32: int(id) of a PlayerRef is negative on 32-bit
// builds, so a bare int(id) > len(g.Objs) test would let it through and
// index Objs with a huge negative offset.
func (g *Game) Obj(id ObjID) *Object {
	if id == 0 || id&playerRefBit != 0 || int(id) > len(g.Objs) {
		return nil
	}
	return &g.Objs[id-1]
}

func (g *Game) AddObject(card *cards.Card, owner PlayerID) *Object {
	o := Object{ID: g.NextID, Card: card, Owner: owner, Controller: owner, Zone: ZLibrary}
	g.NextID++
	g.Objs = append(g.Objs, o)
	return &g.Objs[len(g.Objs)-1]
}

// Clone deep-copies the game. Everything is slices of value types, so this is
// a handful of copy() calls rather than a graph walk. Player's Commander
// bookkeeping slices (Commanders/CmdCasts/CmdDamage) are deep-copied as well
// -- they are sized once at genesis and written throughout a match, so a
// clone that shared the live one's backing arrays would let either evolve
// and silently corrupt the other. Game.Clone is the hottest path in the
// engine; three small copy() calls per seat (nil slices cost nothing) is the
// whole price.
func (g *Game) Clone() *Game {
	c := *g
	c.Players = make([]Player, len(g.Players))
	for i := range g.Players {
		c.Players[i] = g.Players[i]
		c.Players[i].Counters = append([]Counter(nil), g.Players[i].Counters...)
		c.Players[i].Commanders = append([]ObjID(nil), g.Players[i].Commanders...)
		c.Players[i].CmdCasts = append([]int32(nil), g.Players[i].CmdCasts...)
		c.Players[i].CmdDamage = append([]int32(nil), g.Players[i].CmdDamage...)
		c.Players[i].RestrictedMana = append([]ManaRestriction(nil), g.Players[i].RestrictedMana...)
	}
	c.Objs = make([]Object, len(g.Objs))
	for i := range g.Objs {
		c.Objs[i] = g.Objs[i].CloneDeep()
	}
	c.Stack = append([]ObjID(nil), g.Stack...)
	// Entered is appended to in place by every Move (one entry per zone
	// entry this turn), so a clone must own its own copy -- sharing the
	// backing array would let either evolve and corrupt the other (the same
	// rule as Delayed below and Stack above).
	c.Entered = append([]ZoneEntry(nil), g.Entered...)
	// Delayed is written in place (append on registration, remove on firing),
	// so a clone must own its own registrations -- sharing the live one's
	// backing array would let either evolve and corrupt the other (the same
	// rule as every other mutated slice in this struct). Each entry's
	// Remembered slice is deep-copied too. DelayedNext is a plain value and
	// travels with the struct copy above.
	if g.Delayed != nil {
		c.Delayed = make([]DelayedTrigger, len(g.Delayed))
		for i, dt := range g.Delayed {
			dt.Remembered = append([]Target(nil), dt.Remembered...)
			c.Delayed[i] = dt
		}
	}
	c.zones = make([][]ObjID, len(g.zones))
	for i, z := range g.zones {
		if z != nil {
			c.zones[i] = append([]ObjID(nil), z...)
		}
	}
	if g.ExtraTurns != nil {
		c.ExtraTurns = make(map[PlayerID]int, len(g.ExtraTurns))
		for p, n := range g.ExtraTurns {
			c.ExtraTurns[p] = n
		}
	}
	c.ExtraTurnQueue = append([]ExtraTurn(nil), g.ExtraTurnQueue...)
	return &c
}

// AliveFrom lists surviving seats in APNAP order starting at start.
func (g *Game) AliveFrom(start PlayerID) []PlayerID {
	n := PlayerID(len(g.Players))
	out := make([]PlayerID, 0, n)
	for i := PlayerID(0); i < n; i++ {
		p := (start + i) % n
		if !g.Players[p].Lost {
			out = append(out, p)
		}
	}
	return out
}

func (g *Game) AliveCount() int { return len(g.AliveFrom(0)) }

// IsMonarch reports whether p currently holds the monarch designation.
func (g *Game) IsMonarch(p PlayerID) bool { return g.HasMonarch && g.Monarch == p }

// IsStartingPlayer reports whether p currently holds the CR 103.1 first-turn
// designation. The presence bit makes the zero seat unambiguous.
func (g *Game) IsStartingPlayer(p PlayerID) bool {
	return g.HasStartingPlayer && g.StartingPlayer == p
}

// NextAlive returns the next surviving seat after p, or p itself if none is.
func (g *Game) NextAlive(p PlayerID) PlayerID {
	n := PlayerID(len(g.Players))
	for i := PlayerID(1); i <= n; i++ {
		q := (p + i) % n
		if !g.Players[q].Lost {
			return q
		}
	}
	return p
}
