package state

import "github.com/adams-shaun/gorge/cards"

type Player struct {
	ID          PlayerID
	Name        string
	Life        int32
	Lost        bool
	LandsPlayed int32
	Pool        Mana

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
	Step     Step
	Passes   int32
	Over     bool
	Winner   PlayerID
	// Draw marks a game that ended with no surviving seats (CR 104.4a).
	// Winner's zero value is PlayerID(0), a real seat, so Over alone cannot
	// distinguish "seat 0 won" from "nobody did" -- Draw is what does.
	Draw bool
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

	// zones is indexed by zoneIndex(z, p); the stack lives in Stack instead.
	zones [][]ObjID
}

const startingLife = 20

// NewGame builds a game with the default starting life total (20). It keeps
// the every-existing-caller spelling: NewGameLife is the life-taking variant,
// so callers that never set a life total observe exactly what they always
// did.
func NewGame(names []string) *Game { return NewGameLife(names, startingLife) }

// NewGameLife is NewGame with an explicit starting life total (Config.
// StartingLife's 0-means-20 convention is resolved by the caller).
func NewGameLife(names []string, life int32) *Game {
	g := &Game{NextID: 1, zones: make([][]ObjID, numZones*len(names))}
	for i, n := range names {
		g.Players = append(g.Players, Player{ID: PlayerID(i), Name: n, Life: life})
	}
	return g
}

func (g *Game) zoneIndex(z Zone, p PlayerID) int { return int(p)*numZones + int(z) }

func (g *Game) Zone(z Zone, p PlayerID) []ObjID {
	if z == ZStack {
		return g.Stack
	}
	return g.zones[g.zoneIndex(z, p)]
}

func (g *Game) SetZone(z Zone, p PlayerID, ids []ObjID) {
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
		c.Players[i].Commanders = append([]ObjID(nil), g.Players[i].Commanders...)
		c.Players[i].CmdCasts = append([]int32(nil), g.Players[i].CmdCasts...)
		c.Players[i].CmdDamage = append([]int32(nil), g.Players[i].CmdDamage...)
	}
	c.Objs = make([]Object, len(g.Objs))
	for i := range g.Objs {
		c.Objs[i] = g.Objs[i].CloneDeep()
	}
	c.Stack = append([]ObjID(nil), g.Stack...)
	c.zones = make([][]ObjID, len(g.zones))
	for i, z := range g.zones {
		if z != nil {
			c.zones[i] = append([]ObjID(nil), z...)
		}
	}
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
